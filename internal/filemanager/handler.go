package filemanager

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

const maxRequestSize = MaxUploadSize + (1 << 20)

type apiHandler struct {
	store Store
}

func RegisterRoutes(mux *http.ServeMux, store Store) {
	handler := &apiHandler{store: store}
	mux.HandleFunc("GET /api/files", handler.list)
	mux.HandleFunc("POST /api/files", handler.upload)
	mux.HandleFunc("GET /api/files/{id}", handler.download)
	mux.HandleFunc("DELETE /api/files/{id}", handler.delete)
	mux.HandleFunc("PUT /api/files/{id}/association", handler.associate)
	mux.HandleFunc("GET /files", handler.page)
}

func (h *apiHandler) list(w http.ResponseWriter, r *http.Request) {
	files, err := h.store.List()
	if err != nil {
		http.Error(w, "could not read file list", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": files})
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
	record, err := h.store.Save(name, contentType, association, io.MultiReader(bytes.NewReader(prefix), file))
	if err != nil {
		if strings.Contains(err.Error(), "exceeds") {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "could not store uploaded file: "+err.Error(), http.StatusInternalServerError)
		return
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
	record, err := h.store.SetAssociation(r.PathValue("id"), association)
	if errors.Is(err, ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not update file association", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (h *apiHandler) page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, filePage)
}

func parseAssociation(kind, id string) (Association, error) {
	kind = strings.TrimSpace(strings.ToLower(kind))
	id = strings.TrimSpace(id)
	if kind == "" && id == "" {
		return Association{}, nil
	}
	switch kind {
	case "post", "reply", "user":
	default:
		return Association{}, fmt.Errorf("related_type must be post, reply, or user")
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

