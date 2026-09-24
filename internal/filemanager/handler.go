package filemanager

import (
	"bytes"
	"crypto/subtle"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxRequestSize = MaxUploadSize + (1 << 20)

type apiHandler struct {
	store Store
	links Linker
	token string
}

type Linker interface {
	Attach(fileID, kind, id string) error
	Detach(fileID string) error
}

func RegisterRoutes(mux *http.ServeMux, store Store, links Linker, token string) {
	handler := &apiHandler{store: store, links: links, token: token}
	mux.HandleFunc("GET /api/files", handler.list)
	mux.HandleFunc("GET /api/files/trash", handler.trash)
	mux.HandleFunc("DELETE /api/files/trash", handler.emptyTrash)
	mux.HandleFunc("POST /api/files", handler.upload)
	mux.HandleFunc("POST /api/files/{id}/restore", handler.restore)
	mux.HandleFunc("DELETE /api/files/{id}/purge", handler.purge)
	mux.HandleFunc("GET /api/files/{id}", handler.download)
	mux.HandleFunc("GET /api/files/{id}/preview", handler.preview)
	mux.HandleFunc("DELETE /api/files/{id}", handler.delete)
	mux.HandleFunc("PUT /api/files/{id}/association", handler.associate)
	mux.HandleFunc("GET /files", handler.page)
}

func (h *apiHandler) admin(w http.ResponseWriter, r *http.Request) bool {
	value := r.Header.Get("X-Admin-Token")
	if len(value) != len(h.token) || subtle.ConstantTimeCompare([]byte(value), []byte(h.token)) != 1 {
		http.Error(w, "admin token required (see data/config/admin-token)", http.StatusForbidden)
		return false
	}
	return true
}

func (h *apiHandler) discardUpload(id string) {
	_ = h.links.Detach(id)
	if err := h.store.Delete(id); err == nil { _ = h.store.Purge(id) }
}

func (h *apiHandler) list(w http.ResponseWriter, r *http.Request) {
	files, err := h.store.List()
	if err != nil {
		http.Error(w, "could not read file list", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": files})
}

func (h *apiHandler) trash(w http.ResponseWriter, r *http.Request) {
	if !h.admin(w,r) { return }
	files, err := h.store.Trash()
	if err != nil { http.Error(w,"could not read recycle bin",http.StatusInternalServerError); return }
	writeJSON(w,http.StatusOK,map[string]any{"items":files})
}

func (h *apiHandler) emptyTrash(w http.ResponseWriter, r *http.Request) {
	if !h.admin(w,r) { return }
	files, err := h.store.Trash()
	if err != nil { http.Error(w,"could not read recycle bin",http.StatusInternalServerError); return }
	for _, file := range files {
		if err := h.store.Purge(file.ID); err != nil { http.Error(w,"could not empty recycle bin",http.StatusInternalServerError); return }
		if err := h.links.Detach(file.ID); err != nil { http.Error(w,"file removed but its content link could not be cleared",http.StatusInternalServerError); return }
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandler) restore(w http.ResponseWriter, r *http.Request) {
	if !h.admin(w,r) { return }
	file, err := h.store.Restore(r.PathValue("id"))
	if errors.Is(err,ErrNotFound) { http.NotFound(w,r); return }
	if err != nil { http.Error(w,"could not restore file",http.StatusInternalServerError); return }
	writeJSON(w,http.StatusOK,file)
}

func (h *apiHandler) purge(w http.ResponseWriter, r *http.Request) {
	if !h.admin(w,r) { return }
	if err := h.store.Purge(r.PathValue("id")); errors.Is(err,ErrNotFound) { http.NotFound(w,r); return } else if err != nil { http.Error(w,"could not permanently delete file",http.StatusInternalServerError); return }
	if err := h.links.Detach(r.PathValue("id")); err != nil { http.Error(w,"file removed but its content link could not be cleared",http.StatusInternalServerError); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandler) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		http.Error(w, "invalid upload form or request exceeds size limit", http.StatusBadRequest)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "choose a file to upload using field 'file'", http.StatusBadRequest)
		return
	}
	defer file.Close()
	if header.Size > MaxUploadSize {
		http.Error(w, "file exceeds 20 MB limit", http.StatusRequestEntityTooLarge)
		return
	}
	name := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	if name == "" || name == "." {
		http.Error(w, "invalid file name", http.StatusBadRequest)
		return
	}
	prefix := make([]byte, 512)
	n, readErr := io.ReadFull(file, prefix)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		http.Error(w, "could not read upload", http.StatusBadRequest)
		return
	}
	prefix = prefix[:n]
	contentType := http.DetectContentType(prefix)
	association, err := parseAssociation(r.FormValue("related_type"), r.FormValue("related_id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	record, err := h.store.Save(name, contentType, Association{}, io.MultiReader(bytes.NewReader(prefix), file))
	if err != nil {
		if errors.Is(err, ErrTooLarge) {
			http.Error(w, "file exceeds 20 MB limit", http.StatusRequestEntityTooLarge)
			return
		}
		if errors.Is(err, ErrEmptyFile) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "could not store uploaded file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if association.Type != "" {
		if err := h.links.Attach(record.ID, association.Type, association.ID); err != nil {
			h.discardUpload(record.ID)
			http.Error(w, "could not attach file: "+err.Error(), http.StatusBadRequest)
			return
		}
		record, err = h.store.SetAssociation(record.ID, association)
		if err != nil {
			h.discardUpload(record.ID)
			http.Error(w, "could not store file association", http.StatusInternalServerError)
			return
		}
	}
	record.DownloadURL = "/api/files/" + record.ID
	writeJSON(w, http.StatusCreated, record)
}

func (h *apiHandler) download(w http.ResponseWriter, r *http.Request) {
	record, file, err := h.store.Open(r.PathValue("id"))
	if errors.Is(err, ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": record.Name}))
	w.Header().Set("Content-Length", strconv.FormatInt(record.Size, 10))
	_, _ = io.Copy(w, file)
}

func (h *apiHandler) delete(w http.ResponseWriter, r *http.Request) {
	if !h.admin(w,r) { return }
	if err := h.store.Delete(r.PathValue("id")); errors.Is(err, ErrNotFound) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		http.Error(w, "could not remove file", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *apiHandler) associate(w http.ResponseWriter, r *http.Request) {
	if !h.admin(w,r) { return }
	var input Association
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	association, err := parseAssociation(input.Type, input.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	previous, err := h.store.Get(r.PathValue("id"))
	if errors.Is(err, ErrNotFound) { http.NotFound(w, r); return }
	if err != nil { http.Error(w, "could not read file", 500); return }
	if previous.Association == association { writeJSON(w, 200, previous); return }
	if previous.Association.Type != "" { if err := h.links.Detach(previous.ID); err != nil { http.Error(w, "could not update file association", 500); return } }
	if association.Type != "" {
		if err := h.links.Attach(previous.ID, association.Type, association.ID); err != nil {
			if previous.Association.Type != "" { _ = h.links.Attach(previous.ID, previous.Association.Type, previous.Association.ID) }
			http.Error(w, "could not attach file: "+err.Error(), 400); return
		}
	}
	record, err := h.store.SetAssociation(r.PathValue("id"), association)
	if errors.Is(err, ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		_ = h.links.Detach(previous.ID)
		if previous.Association.Type != "" { _ = h.links.Attach(previous.ID, previous.Association.Type, previous.Association.ID) }
		http.Error(w, "could not update file association", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *apiHandler) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, filePage)
}

func (h *apiHandler) preview(w http.ResponseWriter, r *http.Request) {
	record, file, err := h.store.Open(r.PathValue("id"))
	if errors.Is(err, ErrNotFound) { http.NotFound(w, r); return }
	if err != nil { http.Error(w, "could not open file", 500); return }
	defer file.Close()
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	ext := strings.ToLower(filepath.Ext(record.Name))
	if record.ContentType == "image/png" || record.ContentType == "image/jpeg" || record.ContentType == "image/gif" || record.ContentType == "image/webp" {
		w.Header().Set("Content-Type", record.ContentType)
		_, _ = io.Copy(w, file)
		return
	}
	if ext == ".pdf" && record.ContentType == "application/pdf" {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = io.Copy(w, file)
		return
	}
	if record.Size > 1<<20 { http.Error(w, "text preview is limited to 1 MB; download this file instead", 413); return }
	switch ext { case ".txt", ".md", ".markdown", ".json", ".csv", ".log": default: http.Error(w, "preview unavailable; download this file instead", 415); return }
	data, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil { http.Error(w, "could not read preview", 500); return }
	if !utf8.Valid(data) { http.Error(w, "text is not UTF-8; download this file instead", 415); return }
	if ext == ".json" {
		var formatted bytes.Buffer
		if err := json.Indent(&formatted, data, "", "  "); err != nil { http.Error(w, "invalid JSON; download this file instead", 415); return }
		data = formatted.Bytes()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<!doctype html><html lang=\"zh-CN\"><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>%s · 预览</title><style>body{max-width:900px;margin:3rem auto;padding:0 1rem;font:16px/1.6 system-ui}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:#f5f6f9;padding:1rem;border-radius:10px}table{border-collapse:collapse}td{border:1px solid #ddd;padding:5px}</style><h1>%s</h1><a href=\"/api/files/%s\">下载原文件</a>", html.EscapeString(record.Name), html.EscapeString(record.Name), record.ID)
	if ext == ".csv" {
		rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
		if err != nil || len(rows) > 500 { _, _ = fmt.Fprintf(w, "<pre>%s</pre>", html.EscapeString(string(data))) } else {
			_, _ = io.WriteString(w, "<table>")
			for _, row := range rows { _, _ = io.WriteString(w, "<tr>"); for _, cell := range row { _, _ = fmt.Fprintf(w, "<td>%s</td>", html.EscapeString(cell)) }; _, _ = io.WriteString(w, "</tr>") }
			_, _ = io.WriteString(w, "</table>")
		}
	} else { _, _ = fmt.Fprintf(w, "<pre>%s</pre>", html.EscapeString(string(data))) }
	_, _ = io.WriteString(w, "</html>")
}

func parseAssociation(kind, id string) (Association, error) {
	kind = strings.TrimSpace(strings.ToLower(kind))
	id = strings.TrimSpace(id)
	if kind == "" && id == "" {
		return Association{}, nil
	}
	switch kind {
	case "post", "reply":
	default:
		return Association{}, fmt.Errorf("related_type must be post or reply")
	}
	if id == "" || len(id) > 128 {
		return Association{}, fmt.Errorf("related_id is required and must be at most 128 characters")
	}
	return Association{Type: kind, ID: id}, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
