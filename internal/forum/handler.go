package forum

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
)

type Handler struct {
	Store *Store
	Token string
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/",h.page)
	mux.HandleFunc("GET /api/forum/settings",h.settings)
	mux.HandleFunc("PUT /api/forum/settings",h.saveSettings)
	mux.HandleFunc("GET /api/forum/topics",h.topics)
	mux.HandleFunc("POST /api/forum/topics",h.createTopic)
	mux.HandleFunc("GET /api/forum/posts",h.posts)
	mux.HandleFunc("POST /api/forum/posts",h.createPost)
	mux.HandleFunc("GET /api/forum/posts/{id}",h.post)
	mux.HandleFunc("POST /api/forum/posts/{id}/replies",h.createReply)
}

func respond(w http.ResponseWriter,status int,value any){w.Header().Set("Content-Type","application/json; charset=utf-8");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(value)}
func fail(w http.ResponseWriter,err error){status:=http.StatusInternalServerError;message:="server error";if errors.Is(err,ErrNotFound){status=http.StatusNotFound;message="record not found"};http.Error(w,message,status)}
func decode(w http.ResponseWriter,r *http.Request,out any) bool {
	r.Body=http.MaxBytesReader(w,r.Body,110000)
	d:=json.NewDecoder(r.Body);d.DisallowUnknownFields()
	if err:=d.Decode(out);err!=nil{http.Error(w,"invalid JSON request: "+err.Error(),400);return false}
	var extra any;if err:=d.Decode(&extra);err!=io.EOF{http.Error(w,"unexpected data after JSON request",400);return false}
	return true
}
func (h *Handler) admin(w http.ResponseWriter,r *http.Request) bool {
	value:=r.Header.Get("X-Admin-Token")
	if len(value)!=len(h.Token)||subtle.ConstantTimeCompare([]byte(value),[]byte(h.Token))!=1{http.Error(w,"admin token required (see data/config/admin-token)",http.StatusForbidden);return false};return true
}
func (h *Handler) page(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/"{http.NotFound(w,r);return};if r.Method!=http.MethodGet{w.Header().Set("Allow",http.MethodGet);http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return};w.Header().Set("Content-Type","text/html; charset=utf-8");w.Header().Set("Content-Security-Policy","default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data:; object-src 'none'; base-uri 'none'");_,_=io.WriteString(w,forumPage)}
func (h *Handler) settings(w http.ResponseWriter,r *http.Request){v,err:=h.Store.Settings();if err!=nil{fail(w,err);return};respond(w,200,v)}
func (h *Handler) saveSettings(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};var v Settings;if !decode(w,r,&v){return};if err:=h.Store.SaveSettings(v);err!=nil{http.Error(w,err.Error(),400);return};respond(w,200,v)}
func (h *Handler) topics(w http.ResponseWriter,r *http.Request){v,err:=h.Store.Topics();if err!=nil{fail(w,err);return};respond(w,200,map[string]any{"items":v})}
func (h *Handler) createTopic(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};var in struct{Name string `json:"name"`;Description string `json:"description"`};if !decode(w,r,&in){return};v,err:=h.Store.CreateTopic(in.Name,in.Description);if err!=nil{http.Error(w,err.Error(),400);return};respond(w,201,v)}
func (h *Handler) posts(w http.ResponseWriter,r *http.Request){limit,_:=strconv.Atoi(r.URL.Query().Get("limit"));offset,_:=strconv.Atoi(r.URL.Query().Get("offset"));v,err:=h.Store.Posts(r.URL.Query().Get("topic_id"),limit,offset);if err!=nil{fail(w,err);return};respond(w,200,map[string]any{"items":v})}
func (h *Handler) createPost(w http.ResponseWriter,r *http.Request){settings,err:=h.Store.Settings();if err!=nil{fail(w,err);return};if !settings.AllowGuestPosts&&!h.admin(w,r){return};var in struct{TopicID string `json:"topic_id"`;Title string `json:"title"`;Body string `json:"body"`;Author string `json:"author"`};if !decode(w,r,&in){return};v,err:=h.Store.CreatePost(in.TopicID,in.Title,in.Body,in.Author);if errors.Is(err,ErrNotFound){fail(w,err);return};if err!=nil{http.Error(w,err.Error(),400);return};respond(w,201,v)}
func (h *Handler) post(w http.ResponseWriter,r *http.Request){v,err:=h.Store.Post(r.PathValue("id"));if err!=nil{fail(w,err);return};respond(w,200,v)}
func (h *Handler) createReply(w http.ResponseWriter,r *http.Request){settings,err:=h.Store.Settings();if err!=nil{fail(w,err);return};if !settings.AllowGuestReplies&&!h.admin(w,r){return};var in struct{Body string `json:"body"`;Author string `json:"author"`};if !decode(w,r,&in){return};v,err:=h.Store.CreateReply(r.PathValue("id"),in.Body,in.Author);if errors.Is(err,ErrNotFound){fail(w,err);return};if err!=nil{http.Error(w,err.Error(),400);return};respond(w,201,v)}

