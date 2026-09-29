package live

import (
 "bufio"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"
)
func TestClipboardPermissionGatesAndExpiry(t *testing.T){
 enabled:=false
 h:=New(func()(Options,error){return Options{Clipboard:enabled},nil},"secret")
 request:=func(method,body,token string)*httptest.ResponseRecorder{r:=httptest.NewRequest(method,"/api/clipboard",strings.NewReader(body));r.Header.Set("X-Admin-Token",token);w:=httptest.NewRecorder();h.clipboard(w,r);return w}
 if w:=request("GET","","secret");w.Code!=403{t.Fatalf("disabled: %d",w.Code)}
 enabled=true
 if w:=request("GET","","");w.Code!=403{t.Fatalf("anonymous: %d",w.Code)}
 if w:=request("PUT",`{"text":"hello"}`,"secret");w.Code!=200{t.Fatalf("send: %d",w.Code)}
 w:=request("GET","","secret");var result struct{Text string `json:"text"`};if err:=json.Unmarshal(w.Body.Bytes(),&result);err!=nil||result.Text!="hello"{t.Fatalf("receive: %s",w.Body.String())}
 h.expires=time.Now().Add(-time.Second)
 w=request("GET","","secret");if err:=json.Unmarshal(w.Body.Bytes(),&result);err!=nil||result.Text!=""{t.Fatal("expired text was returned")}
 payload,_:=json.Marshal(map[string]string{"text":strings.Repeat("x",32769)})
 if w=request("PUT",string(payload),"secret");w.Code!=400{t.Fatalf("oversize: %d",w.Code)}
 request("PUT",`{"text":"again"}`,"secret");request("DELETE","","secret");if h.text!=""{t.Fatal("delete did not clear relay")}
}
func TestMutationNotifications(t *testing.T){
 h:=New(func()(Options,error){return Options{},nil},"secret");ch:=make(chan string,16);h.clients[ch]=struct{}{}
 for _,test:=range []struct{method string;status int;notify bool}{{"GET",200,false},{"POST",400,false},{"POST",201,true},{"DELETE",204,true}}{
  handler:=h.Wrap(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(test.status)}));w:=httptest.NewRecorder();handler.ServeHTTP(w,httptest.NewRequest(test.method,"/api/forum/posts",nil))
  select{case <-ch:if !test.notify{t.Fatal("unexpected notification")};default:if test.notify{t.Fatal("missing notification")}}
 }
}
func TestEventsDisabled(t *testing.T){h:=New(func()(Options,error){return Options{},nil},"");w:=httptest.NewRecorder();h.events(w,httptest.NewRequest("GET","/api/events",nil));if w.Code!=204{t.Fatalf("disabled stream: %d",w.Code)}}

func TestServerPushWithoutPolling(t *testing.T){
 h:=New(func()(Options,error){return Options{Realtime:true},nil},"secret")
 mux:=http.NewServeMux();h.Register(mux)
 mux.HandleFunc("POST /api/forum/posts",func(w http.ResponseWriter,r *http.Request){w.WriteHeader(http.StatusCreated)})
 server:=httptest.NewServer(h.Wrap(mux));defer server.Close()
 client:=&http.Client{Timeout:3*time.Second}
 response,err:=client.Get(server.URL+"/api/events");if err!=nil{t.Fatal(err)};defer response.Body.Close()
 reader:=bufio.NewReader(response.Body)
 readEvent:=func()string{t.Helper();var value strings.Builder;for {line,err:=reader.ReadString('\n');if err!=nil{t.Fatal(err)};value.WriteString(line);if line=="\n"{return value.String()}}}
 if event:=readEvent();!strings.Contains(event,"event: ready"){t.Fatalf("initial event: %q",event)}
 changed,err:=client.Post(server.URL+"/api/forum/posts","application/json",strings.NewReader("{}"));if err!=nil{t.Fatal(err)};changed.Body.Close()
 if event:=readEvent();!strings.Contains(event,"event: change")||!strings.Contains(event,"/api/forum/posts"){t.Fatalf("push event: %q",event)}
}
