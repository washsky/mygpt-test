// Package live provides bounded server-sent notifications and an opt-in text relay.
package live

import (
 "database/sql"
 "github.com/washsky/mygpt-test/internal/filemanager"
 "encoding/json"
 "fmt"
 "net/http"
 "strings"
 "sync"
 "time"
)

type Options struct { Realtime, Clipboard bool; ClipboardTTLMinutes int }
type Hub struct {
 mu sync.Mutex
 clients map[chan string]struct{}
 options func() (Options, error)
 db *sql.DB
 files filemanager.Store
}
func New(options func() (Options,error), dataDir string, files filemanager.Store) (*Hub,error) {
 h:=&Hub{clients:make(map[chan string]struct{}),options:options,files:files}
 if err:=h.initClipboard(dataDir);err!=nil{return nil,err};return h,nil
}
func (h *Hub) Close() error{return h.db.Close()}
func (h *Hub) publish(path string) { h.mu.Lock(); defer h.mu.Unlock(); for ch := range h.clients { select { case ch <- path: default: // Coalesce backlog into a full-state invalidation.
 select {case <-ch:default:};select {case ch <- "":default:}
 } } }
func (h *Hub) Register(mux *http.ServeMux) { mux.HandleFunc("GET /api/events",h.events);mux.HandleFunc("GET /api/clipboard",h.listClipboard);mux.HandleFunc("POST /api/clipboard",h.createClipboard);mux.HandleFunc("DELETE /api/clipboard",h.deleteClipboard);mux.HandleFunc("PUT /api/clipboard/{id}",h.itemClipboard);mux.HandleFunc("DELETE /api/clipboard/{id}",h.itemClipboard) }
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
  if path=="/api/forum/settings" {if opt,err:=h.options();err==nil&&!opt.Clipboard{_ = h.clearClipboard()}}
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
