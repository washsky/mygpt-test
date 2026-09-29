package live

import (
 "crypto/rand"
 "database/sql"
 "encoding/hex"
 "encoding/json"
 "errors"
 "io"
 "net/http"
 "path/filepath"
 "strings"
 "time"

 "github.com/washsky/mygpt-test/internal/filemanager"
 _ "github.com/ncruces/go-sqlite3/driver"
 _ "github.com/ncruces/go-sqlite3/embed"
)

type Entry struct {
 ID string `json:"id"`
 Kind string `json:"kind"`
 Text string `json:"text,omitempty"`
 FileID string `json:"file_id,omitempty"`
 Name string `json:"name,omitempty"`
 CreatedAt string `json:"created_at"`
 ExpiresAt string `json:"expires_at"`
}

func (h *Hub) initClipboard(dataDir string) error {
 db,err:=sql.Open("sqlite3",filepath.Join(dataDir,"clipboard.sqlite"));if err!=nil{return err}
 db.SetMaxOpenConns(1)
 for _,statement:=range []string{`CREATE TABLE IF NOT EXISTS clipboard_entries (id TEXT PRIMARY KEY, kind TEXT NOT NULL, text TEXT NOT NULL, file_id TEXT NOT NULL, name TEXT NOT NULL, created_at TEXT NOT NULL, expires_at TEXT NOT NULL)`,`CREATE INDEX IF NOT EXISTS clipboard_expiry ON clipboard_entries(expires_at)`}{if _,err=db.Exec(statement);err!=nil{db.Close();return err}}
 h.db=db;return nil
}
func (h *Hub) clearClipboard() error {_,err:=h.db.Exec("DELETE FROM clipboard_entries");return err}
func (h *Hub) prune() error {
 if _,err:=h.db.Exec("DELETE FROM clipboard_entries WHERE expires_at<=?",time.Now().UTC().Format(time.RFC3339Nano));err!=nil{return err}
 // Bound storage independently of the configured expiry.
 _,err:=h.db.Exec("DELETE FROM clipboard_entries WHERE id NOT IN (SELECT id FROM clipboard_entries ORDER BY created_at DESC LIMIT 50)")
 return err
}
func (h *Hub) listClipboard(w http.ResponseWriter,r *http.Request){
 w.Header().Set("Cache-Control","no-store")
 options,err:=h.options();if err!=nil{http.Error(w,"settings unavailable",503);return};if !options.Clipboard{http.Error(w,"clipboard disabled",403);return}
 if err:=h.prune();err!=nil{http.Error(w,"clipboard unavailable",500);return}
 rows,err:=h.db.Query("SELECT id,kind,text,file_id,name,created_at,expires_at FROM clipboard_entries ORDER BY created_at DESC LIMIT 50");if err!=nil{http.Error(w,"clipboard unavailable",500);return};defer rows.Close()
 items:=[]Entry{}
 for rows.Next(){var e Entry;if err:=rows.Scan(&e.ID,&e.Kind,&e.Text,&e.FileID,&e.Name,&e.CreatedAt,&e.ExpiresAt);err!=nil{http.Error(w,"clipboard unavailable",500);return};items=append(items,e)}
 if rows.Err()!=nil{http.Error(w,"clipboard unavailable",500);return}
 w.Header().Set("Content-Type","application/json; charset=utf-8");_ = json.NewEncoder(w).Encode(map[string]any{"items":items})
}
func (h *Hub) createClipboard(w http.ResponseWriter,r *http.Request){
 w.Header().Set("Cache-Control","no-store")
 options,err:=h.options();if err!=nil{http.Error(w,"settings unavailable",503);return};if !options.Clipboard{http.Error(w,"clipboard disabled",403);return}
 r.Body=http.MaxBytesReader(w,r.Body,128<<10)
 var in struct { Kind string `json:"kind"`; Text string `json:"text"`; FileID string `json:"file_id"` }
 decoder:=json.NewDecoder(r.Body);decoder.DisallowUnknownFields();if err:=decoder.Decode(&in);err!=nil{http.Error(w,"invalid clipboard entry",400);return};var extra any;if decoder.Decode(&extra)!=io.EOF{http.Error(w,"unexpected request data",400);return}
 if in.Kind=="" {in.Kind="text"}
 var name string
 switch in.Kind {
 case "text":if in.Text==""||len(in.Text)>32<<10||in.FileID!=""{http.Error(w,"text must be 1–32768 bytes",400);return}
 case "file":if in.FileID==""||in.Text!=""||h.files==nil{http.Error(w,"invalid file reference",400);return};file,err:=h.files.Get(in.FileID);if errors.Is(err,filemanager.ErrNotFound){http.NotFound(w,r);return};if err!=nil{http.Error(w,"could not read file",500);return};name=file.Name
 default:http.Error(w,"kind must be text or file",400);return
 }
 var random [16]byte;if _,err=rand.Read(random[:]);err!=nil{http.Error(w,"could not create entry",500);return}
 now:=time.Now().UTC();ttl:=options.ClipboardTTLMinutes;if ttl<1||ttl>10080{ttl=10}
 e:=Entry{ID:hex.EncodeToString(random[:]),Kind:in.Kind,Text:in.Text,FileID:in.FileID,Name:name,CreatedAt:now.Format(time.RFC3339Nano),ExpiresAt:now.Add(time.Duration(ttl)*time.Minute).Format(time.RFC3339Nano)}
 if _,err=h.db.Exec("INSERT INTO clipboard_entries(id,kind,text,file_id,name,created_at,expires_at) VALUES(?,?,?,?,?,?,?)",e.ID,e.Kind,e.Text,e.FileID,e.Name,e.CreatedAt,e.ExpiresAt);err!=nil{http.Error(w,"could not save entry",500);return}
 _=h.prune();w.Header().Set("Content-Type","application/json; charset=utf-8");w.WriteHeader(201);_ = json.NewEncoder(w).Encode(e)
}
func (h *Hub) itemClipboard(w http.ResponseWriter,r *http.Request){
 w.Header().Set("Cache-Control","no-store")
 options,err:=h.options();if err!=nil{http.Error(w,"settings unavailable",503);return};if !options.Clipboard{http.Error(w,"clipboard disabled",403);return}
 id:=r.PathValue("id");if len(id)!=32||strings.Trim(id,"0123456789abcdef")!=""{http.NotFound(w,r);return}
 _=h.prune()
 if r.Method==http.MethodDelete{result,err:=h.db.Exec("DELETE FROM clipboard_entries WHERE id=?",id);if err!=nil{http.Error(w,"could not delete entry",500);return};n,_:=result.RowsAffected();if n==0{http.NotFound(w,r);return};w.WriteHeader(204);return}
 r.Body=http.MaxBytesReader(w,r.Body,128<<10)
 var in struct{Text string `json:"text"`};decoder:=json.NewDecoder(r.Body);decoder.DisallowUnknownFields();if err:=decoder.Decode(&in);err!=nil||in.Text==""||len(in.Text)>32<<10{http.Error(w,"text must be 1–32768 bytes",400);return};var extra any;if decoder.Decode(&extra)!=io.EOF{http.Error(w,"unexpected request data",400);return}
 result,err:=h.db.Exec("UPDATE clipboard_entries SET text=? WHERE id=? AND kind='text'",in.Text,id);if err!=nil{http.Error(w,"could not update entry",500);return};n,_:=result.RowsAffected();if n==0{http.NotFound(w,r);return};w.WriteHeader(204)
}
func (h *Hub) deleteClipboard(w http.ResponseWriter,r *http.Request){options,err:=h.options();if err!=nil{http.Error(w,"settings unavailable",503);return};if !options.Clipboard{http.Error(w,"clipboard disabled",403);return};if err:=h.clearClipboard();err!=nil{http.Error(w,"could not clear clipboard",500);return};w.WriteHeader(204)}
