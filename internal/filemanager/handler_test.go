package filemanager

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testLinker struct{}

func (testLinker) Attach(string, string, string) error { return nil }
func (testLinker) Detach(string) error                 { return nil }

func TestFileRecycleBinRequiresAdmin(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil { t.Fatal(err) }
	record, err := store.Save("note.txt", "text/plain", Association{}, bytes.NewBufferString("note"))
	if err != nil { t.Fatal(err) }
	mux := http.NewServeMux()
	RegisterRoutes(mux,store,testLinker{},"admin-token")
	request := func(method,path,token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method,path,nil)
		if token != "" { req.Header.Set("X-Admin-Token",token) }
		response := httptest.NewRecorder()
		mux.ServeHTTP(response,req)
		return response
	}
	if response := request(http.MethodDelete,"/api/files/"+record.ID,""); response.Code != http.StatusForbidden { t.Fatalf("unauthenticated delete status = %d",response.Code) }
	if _, err := store.Get(record.ID); err != nil { t.Fatalf("unauthenticated delete changed file: %v",err) }
	if response := request(http.MethodDelete,"/api/files/"+record.ID,"admin-token"); response.Code != http.StatusNoContent { t.Fatalf("delete status = %d body=%s",response.Code,response.Body.String()) }
	if response := request(http.MethodGet,"/api/files/trash",""); response.Code != http.StatusForbidden { t.Fatalf("unauthenticated trash status = %d",response.Code) }
	if response := request(http.MethodPost,"/api/files/"+record.ID+"/restore","admin-token"); response.Code != http.StatusOK { t.Fatalf("restore status = %d body=%s",response.Code,response.Body.String()) }
	if _, err := store.Get(record.ID); err != nil { t.Fatalf("restored file unavailable: %v",err) }
}
