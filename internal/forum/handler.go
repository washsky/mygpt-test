package forum

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
 "strings"
 "regexp"
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
	mux.HandleFunc("GET /api/forum/trash",h.trash)
	mux.HandleFunc("DELETE /api/forum/trash",h.emptyTrash)
	mux.HandleFunc("POST /api/forum/trash/posts/{id}/restore",h.restorePost)
	mux.HandleFunc("DELETE /api/forum/trash/posts/{id}",h.purgePost)
	mux.HandleFunc("POST /api/forum/trash/replies/{id}/restore",h.restoreReply)
	mux.HandleFunc("DELETE /api/forum/trash/replies/{id}",h.purgeReply)
	mux.HandleFunc("GET /api/forum/posts/{id}",h.post)
	mux.HandleFunc("PUT /api/forum/posts/{id}",h.updatePost)
	mux.HandleFunc("DELETE /api/forum/posts/{id}",h.deletePost)
	mux.HandleFunc("POST /api/forum/posts/{id}/replies",h.createReply)
	mux.HandleFunc("GET /api/forum/replies/{id}/context",h.replyContext)
 mux.HandleFunc("PUT /api/forum/replies/{id}",h.updateReply)
	mux.HandleFunc("DELETE /api/forum/replies/{id}",h.deleteReply)
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
func (h *Handler) page(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/"{http.NotFound(w,r);return};if r.Method!=http.MethodGet{w.Header().Set("Allow",http.MethodGet);http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return};w.Header().Set("Content-Type","text/html; charset=utf-8");w.Header().Set("Content-Security-Policy","default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' https: data: blob:; object-src 'none'; base-uri 'none'");_,_=io.WriteString(w,forumPage)}
func (h *Handler) settings(w http.ResponseWriter,r *http.Request){v,err:=h.Store.Settings();if err!=nil{fail(w,err);return};respond(w,200,v)}
func (h *Handler) saveSettings(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};var v Settings;if !decode(w,r,&v){return};if err:=h.Store.SaveSettings(v);err!=nil{http.Error(w,err.Error(),400);return};respond(w,200,v)}
func (h *Handler) topics(w http.ResponseWriter,r *http.Request){v,err:=h.Store.Topics();if err!=nil{fail(w,err);return};respond(w,200,map[string]any{"items":v})}
func (h *Handler) createTopic(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};var in struct{Name string `json:"name"`;Description string `json:"description"`};if !decode(w,r,&in){return};v,err:=h.Store.CreateTopic(in.Name,in.Description);if err!=nil{http.Error(w,err.Error(),400);return};respond(w,201,v)}
func (h *Handler) posts(w http.ResponseWriter,r *http.Request){limit,_:=strconv.Atoi(r.URL.Query().Get("limit"));offset,_:=strconv.Atoi(r.URL.Query().Get("offset"));topicID:=r.URL.Query().Get("topic_id");query:=r.URL.Query().Get("q");var v []Post;var err error;if query!=""{if len(query)>200{http.Error(w,"search query is too long",http.StatusBadRequest);return};settings,settingsErr:=h.Store.Settings();if settingsErr!=nil{fail(w,settingsErr);return};if !settings.EnableSearch{http.Error(w,"search is disabled in settings",http.StatusForbidden);return};v,err=h.Store.SearchPosts(query,topicID,limit,offset)}else{v,err=h.Store.Posts(topicID,limit,offset)};if err!=nil{fail(w,err);return};respond(w,200,map[string]any{"items":v})}
func (h *Handler) createPost(w http.ResponseWriter,r *http.Request){settings,err:=h.Store.Settings();if err!=nil{fail(w,err);return};if !settings.AllowGuestPosts&&!h.admin(w,r){return};var in struct{TopicID string `json:"topic_id"`;Title string `json:"title"`;Body string `json:"body"`;Author string `json:"author"`;Timezone string `json:"timezone"`};if !decode(w,r,&in){return};client:=ClientInfo{};if settings.EnableClientInfo{client=clientInfo(r,in.Timezone)};v,err:=h.Store.CreatePostWithClient(in.TopicID,in.Title,in.Body,in.Author,client);if errors.Is(err,ErrNotFound){fail(w,err);return};if err!=nil{http.Error(w,err.Error(),400);return};respond(w,201,v)}
func (h *Handler) post(w http.ResponseWriter,r *http.Request){v,err:=h.Store.Post(r.PathValue("id"));if err!=nil{fail(w,err);return};respond(w,200,v)}
func (h *Handler) trash(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};posts,err:=h.Store.TrashPosts();if err!=nil{fail(w,err);return};replies,err:=h.Store.TrashReplies();if err!=nil{fail(w,err);return};respond(w,200,map[string]any{"posts":posts,"replies":replies})}
func (h *Handler) emptyTrash(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};if err:=h.Store.EmptyTrash();err!=nil{fail(w,err);return};w.WriteHeader(http.StatusNoContent)}
func (h *Handler) restorePost(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};if err:=h.Store.RestorePost(r.PathValue("id"));err!=nil{fail(w,err);return};w.WriteHeader(http.StatusNoContent)}
func (h *Handler) purgePost(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};if err:=h.Store.PurgePost(r.PathValue("id"));err!=nil{fail(w,err);return};w.WriteHeader(http.StatusNoContent)}
func (h *Handler) restoreReply(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};if err:=h.Store.RestoreReply(r.PathValue("id"));err!=nil{fail(w,err);return};w.WriteHeader(http.StatusNoContent)}
func (h *Handler) purgeReply(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};if err:=h.Store.PurgeReply(r.PathValue("id"));err!=nil{fail(w,err);return};w.WriteHeader(http.StatusNoContent)}
func (h *Handler) updatePost(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};var in struct{Title string `json:"title"`;Body string `json:"body"`};if !decode(w,r,&in){return};if err:=h.Store.UpdatePost(r.PathValue("id"),in.Title,in.Body);err!=nil{if errors.Is(err,ErrNotFound){fail(w,err)}else{http.Error(w,err.Error(),400)};return};w.WriteHeader(http.StatusNoContent)}
func (h *Handler) deletePost(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};if err:=h.Store.DeletePost(r.PathValue("id"));err!=nil{if errors.Is(err,ErrNotFound){fail(w,err)}else{http.Error(w,err.Error(),500)};return};w.WriteHeader(http.StatusNoContent)}
func (h *Handler) createReply(w http.ResponseWriter,r *http.Request){settings,err:=h.Store.Settings();if err!=nil{fail(w,err);return};if !settings.AllowGuestReplies&&!h.admin(w,r){return};var in struct{Body string `json:"body"`;Author string `json:"author"`;Timezone string `json:"timezone"`};if !decode(w,r,&in){return};client:=ClientInfo{};if settings.EnableClientInfo{client=clientInfo(r,in.Timezone)};v,err:=h.Store.CreateReplyWithClient(r.PathValue("id"),in.Body,in.Author,client);if errors.Is(err,ErrNotFound){fail(w,err);return};if err!=nil{http.Error(w,err.Error(),400);return};respond(w,201,v)}
func (h *Handler) updateReply(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};var in struct{Body string `json:"body"`};if !decode(w,r,&in){return};if err:=h.Store.UpdateReply(r.PathValue("id"),in.Body);err!=nil{if errors.Is(err,ErrNotFound){fail(w,err)}else{http.Error(w,err.Error(),400)};return};w.WriteHeader(http.StatusNoContent)}
func (h *Handler) deleteReply(w http.ResponseWriter,r *http.Request){if !h.admin(w,r){return};if err:=h.Store.DeleteReply(r.PathValue("id"));err!=nil{if errors.Is(err,ErrNotFound){fail(w,err)}else{http.Error(w,err.Error(),500)};return};w.WriteHeader(http.StatusNoContent)}

// replyContext resolves a live reply without exposing deleted content.
func (h *Handler) replyContext(w http.ResponseWriter, r *http.Request) {
 postID, err := h.Store.ReplyPostID(r.PathValue("id"))
 if err != nil { fail(w, err); return }
 respond(w, http.StatusOK, map[string]string{"post_id": postID})
}

var timezonePattern = regexp.MustCompile(`^[A-Za-z0-9_+./-]{1,64}$`)
var browserVersion = regexp.MustCompile(`^[0-9]{1,4}`)
var languagePattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,20}$`)

// clientInfo records broad browser details. The browser header is summarized,
// never retained in full, and these values do not identify a person.
func clientInfo(r *http.Request, timezone string) ClientInfo {
 ua:=r.UserAgent();if len(ua)>1024{ua=ua[:1024]}
 info:=ClientInfo{Browser:"其他浏览器",OS:"其他系统",Device:"电脑"}
 for _,item:=range []struct{needle,name string}{{"Edg/","Edge"},{"OPR/","Opera"},{"Firefox/","Firefox"},{"CriOS/","Chrome"},{"Chrome/","Chrome"},{"Version/","Safari"}}{if pos:=strings.Index(ua,item.needle);pos>=0{info.Browser=item.name;v:=browserVersion.FindString(ua[pos+len(item.needle):]);if v!=""{info.Browser+=" "+v};break}}
 switch{case strings.Contains(ua,"Android"):info.OS="Android";case strings.Contains(ua,"iPhone")||strings.Contains(ua,"iPad"):info.OS="iOS / iPadOS";case strings.Contains(ua,"Windows"):info.OS="Windows";case strings.Contains(ua,"Mac OS X"):info.OS="macOS";case strings.Contains(ua,"Linux"):info.OS="Linux"}
 switch{case strings.Contains(ua,"iPad")||strings.Contains(ua,"Tablet"):info.Device="平板";case strings.Contains(ua,"Mobile")||strings.Contains(ua,"iPhone")||strings.Contains(ua,"Android"):info.Device="手机"}
 language:=strings.TrimSpace(strings.Split(r.Header.Get("Accept-Language"),",")[0]);if len(language)<=20&&languagePattern.MatchString(language){info.Language=language}
 if timezonePattern.MatchString(timezone){info.Timezone=timezone}
 return info
}
