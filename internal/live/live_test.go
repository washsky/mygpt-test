package live

import (
 "bufio"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"

 "github.com/washsky/mygpt-test/internal/filemanager"
)
func testHub(t *testing.T,options func()(Options,error),files filemanager.Store)*Hub {t.Helper();h,err:=New(options,t.TempDir(),files);if err!=nil{t.Fatal(err)};t.Cleanup(func(){h.Close()});return h}
func TestClipboardHistoryEditExpiryAndNoToken(t *testing.T){
 enabled:=false;ttl:=10
 h:=testHub(t,func()(Options,error){return Options{Clipboard:enabled,ClipboardTTLMinutes:ttl},nil},nil)
 mux:=http.NewServeMux();h.Register(mux)
 request:=func(method,path,body string)*httptest.ResponseRecorder{t.Helper();r:=httptest.NewRequest(method,path,strings.NewReader(body));w:=httptest.NewRecorder();mux.ServeHTTP(w,r);return w}
 if w:=request("GET","/api/clipboard","");w.Code!=403{t.Fatalf("disabled: %d",w.Code)}
 enabled=true
 w:=request("POST","/api/clipboard",`{"kind":"text","text":"hello"}`);if w.Code!=201{t.Fatalf("anonymous send: %d %s",w.Code,w.Body)}
 var item Entry;if err:=json.Unmarshal(w.Body.Bytes(),&item);err!=nil{t.Fatal(err)}
 if w:=request("PUT","/api/clipboard/"+item.ID,`{"text":"edited"}`);w.Code!=204{t.Fatalf("edit: %d",w.Code)}
 w=request("GET","/api/clipboard","");var list struct{Items []Entry `json:"items"`};if err:=json.Unmarshal(w.Body.Bytes(),&list);err!=nil||len(list.Items)!=1||list.Items[0].Text!="edited"{t.Fatalf("history: %s",w.Body)}
 h.db.Exec("UPDATE clipboard_entries SET expires_at=? WHERE id=?",time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano),item.ID)
 w=request("GET","/api/clipboard","");if err:=json.Unmarshal(w.Body.Bytes(),&list);err!=nil||len(list.Items)!=0{t.Fatalf("expiry: %s",w.Body)}
 ttl=1
 w=request("POST","/api/clipboard",`{"text":"new"}`);if w.Code!=201{t.Fatalf("create after expiry: %d",w.Code)}
 if w:=request("DELETE","/api/clipboard","");w.Code!=204{t.Fatalf("clear: %d",w.Code)}
 w=request("GET","/api/clipboard","");if err:=json.Unmarshal(w.Body.Bytes(),&list);err!=nil||len(list.Items)!=0{t.Fatalf("clear history: %s",w.Body)}
 if w:=request("POST","/api/clipboard",`{"text":"`+strings.Repeat("x",32769)+`"}`);w.Code!=400{t.Fatalf("oversize: %d",w.Code)}
}
func TestFileReferenceRequiresLiveFile(t *testing.T){
 files,err:=filemanager.NewStore(t.TempDir());if err!=nil{t.Fatal(err)}
 h:=testHub(t,func()(Options,error){return Options{Clipboard:true,ClipboardTTLMinutes:10},nil},files)
 mux:=http.NewServeMux();h.Register(mux)
 send:=func(id string)*httptest.ResponseRecorder{req:=httptest.NewRequest("POST","/api/clipboard",strings.NewReader(`{"kind":"file","file_id":"`+id+`"}`));w:=httptest.NewRecorder();mux.ServeHTTP(w,req);return w}
 if w:=send(strings.Repeat("a",32));w.Code!=404{t.Fatalf("missing file: %d",w.Code)}
 file,err:=files.Save("memo.txt","text/plain",filemanager.Association{},strings.NewReader("content"));if err!=nil{t.Fatal(err)}
 if w:=send(file.ID);w.Code!=201||!strings.Contains(w.Body.String(),"memo.txt"){t.Fatalf("shared file: %d %s",w.Code,w.Body)}
}
func TestMutationNotifications(t *testing.T){
 h:=testHub(t,func()(Options,error){return Options{},nil},nil);ch:=make(chan string,16);h.clients[ch]=struct{}{}
 for _,test:=range []struct{method string;status int;notify bool}{{"GET",200,false},{"POST",400,false},{"POST",201,true},{"DELETE",204,true}}{
  handler:=h.Wrap(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(test.status)}));w:=httptest.NewRecorder();handler.ServeHTTP(w,httptest.NewRequest(test.method,"/api/forum/posts",nil))
  select{case <-ch:if !test.notify{t.Fatal("unexpected notification")};default:if test.notify{t.Fatal("missing notification")}}
 }
}
func TestEventsDisabled(t *testing.T){h:=testHub(t,func()(Options,error){return Options{},nil},nil);w:=httptest.NewRecorder();h.events(w,httptest.NewRequest("GET","/api/events",nil));if w.Code!=204{t.Fatalf("disabled stream: %d",w.Code)}}
func TestServerPushWithoutPolling(t *testing.T){
 h:=testHub(t,func()(Options,error){return Options{Realtime:true},nil},nil)
 mux:=http.NewServeMux();h.Register(mux);mux.HandleFunc("POST /api/forum/posts",func(w http.ResponseWriter,r *http.Request){w.WriteHeader(http.StatusCreated)})
 server:=httptest.NewServer(h.Wrap(mux));defer server.Close()
 client:=&http.Client{Timeout:3*time.Second};response,err:=client.Get(server.URL+"/api/events");if err!=nil{t.Fatal(err)};defer response.Body.Close()
 reader:=bufio.NewReader(response.Body);readEvent:=func()string{t.Helper();var value strings.Builder;for {line,err:=reader.ReadString('\n');if err!=nil{t.Fatal(err)};value.WriteString(line);if line=="\n"{return value.String()}}}
 if event:=readEvent();!strings.Contains(event,"event: ready"){t.Fatalf("initial event: %q",event)}
 changed,err:=client.Post(server.URL+"/api/forum/posts","application/json",strings.NewReader("{}"));if err!=nil{t.Fatal(err)};changed.Body.Close()
 if event:=readEvent();!strings.Contains(event,"event: change")||!strings.Contains(event,"/api/forum/posts"){t.Fatalf("push event: %q",event)}
}
