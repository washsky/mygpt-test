// Package live provides bounded server-sent notifications and an opt-in text relay.
package live

import (
 "crypto/subtle"
 "encoding/json"
 "fmt"
 "net/http"
 "strings"
 "sync"
 "time"
)

type Options struct { Realtime, Clipboard bool }
type Hub struct {
 mu sync.Mutex
 clients map[chan string]struct{}
 options func() (Options, error)
 token string
 text string
 expires time.Time
}
func New(options func() (Options,error), token string) *Hub { return &Hub{clients:make(map[chan string]struct{}),options:options,token:token} }
func (h *Hub) publish(path string) { h.mu.Lock(); defer h.mu.Unlock(); for ch := range h.clients { select { case ch <- path: default: // Coalesce backlog into a full-state invalidation.
 select {case <-ch:default:};select {case ch <- "":default:}
 } } }
func (h *Hub) Register(mux *http.ServeMux) { mux.HandleFunc("GET /api/events",h.events);mux.HandleFunc("GET /api/clipboard",h.clipboard);mux.HandleFunc("PUT /api/clipboard",h.clipboard);mux.HandleFunc("DELETE /api/clipboard",h.clipboard) }
type writer struct { http.ResponseWriter; status int }
func (w *writer) WriteHeader(code int) { if w.status==0 { w.status=code;w.ResponseWriter.WriteHeader(code) } }
func (w *writer) Write(data []byte)(int,error){if w.status==0 {w.WriteHeader(http.StatusOK)};return w.ResponseWriter.Write(data)}
func (w *writer) Unwrap() http.ResponseWriter{return w.ResponseWriter}
func (h *Hub) Wrap(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
 if r.Method==http.MethodGet||r.Method==http.MethodHead {next.ServeHTTP(w,r);return}
 out:=&writer{ResponseWriter:w};next.ServeHTTP(out,r)
 if out.status>=400{return}
 path:=r.URL.Path
 if strings.HasPrefix(path,"/api/forum/")||strings.HasPrefix(path,"/api/files")||path=="/api/clipboard" {
  if path=="/api/forum/settings" {if opt,err:=h.options();err==nil&&!opt.Clipboard{h.mu.Lock();h.text="";h.expires=time.Time{};h.mu.Unlock()}}
  h.publish(path)
 }
 }) }
func (h *Hub) events(w http.ResponseWriter,r *http.Request){
 opt,err:=h.options();if err!=nil{http.Error(w,"settings unavailable",503);return};if !opt.Realtime{w.WriteHeader(http.StatusNoContent);return}
 ch:=make(chan string,16);h.mu.Lock();if len(h.clients)>=128{h.mu.Unlock();http.Error(w,"too many live connections",503);return};h.clients[ch]=struct{}{};h.mu.Unlock()
 defer func(){h.mu.Lock();delete(h.clients,ch);h.mu.Unlock()}()
 w.Header().Set("Content-Type","text/event-stream");w.Header().Set("Cache-Control","no-cache, no-transform");w.Header().Set("X-Accel-Buffering","no")
 control:=http.NewResponseController(w)
 send:=func(event,data string)bool{_ = control.SetWriteDeadline(time.Now().Add(10*time.Second));_,err:=fmt.Fprintf(w,"event: %s\ndata: %s\n\n",event,data);if err!=nil{return false};return control.Flush()==nil}
 if !send("ready",`{}`){return}
 heartbeat:=time.NewTicker(20*time.Second);defer heartbeat.Stop()
 for {select {
 case <-r.Context().Done():return
 case path:=<-ch:
  opt,err=h.options();if err!=nil{return};if !opt.Realtime{send("disabled",`{}`);return}
  payload,_:=json.Marshal(map[string]string{"path":path});if !send("change",string(payload)){return}
 case <-heartbeat.C:
  if !send("ping",`{}`){return}
 }}
}
func (h *Hub) clipboard(w http.ResponseWriter,r *http.Request){
 w.Header().Set("Cache-Control","no-store")
 token:=r.Header.Get("X-Admin-Token");if len(token)!=len(h.token)||subtle.ConstantTimeCompare([]byte(token),[]byte(h.token))!=1{http.Error(w,"administrator token required",403);return}
 opt,err:=h.options();if err!=nil{http.Error(w,"settings unavailable",503);return};if !opt.Clipboard{http.Error(w,"clipboard relay disabled",403);return}
 var value struct {Text string `json:"text"`}
 if r.Method==http.MethodPut {r.Body=http.MaxBytesReader(w,r.Body,128<<10);if err:=json.NewDecoder(r.Body).Decode(&value);err!=nil||len(value.Text)>32<<10{http.Error(w,"text must be at most 32 KiB",400);return}}
 h.mu.Lock();defer h.mu.Unlock()
 if time.Now().After(h.expires){h.text="";h.expires=time.Time{}}
 switch r.Method {case http.MethodPut:h.text=value.Text;h.expires=time.Now().Add(10*time.Minute);case http.MethodDelete:h.text="";h.expires=time.Time{}}
 w.Header().Set("Content-Type","application/json; charset=utf-8")
 _=json.NewEncoder(w).Encode(map[string]any{"text":h.text,"expires_at":h.expires})
}
