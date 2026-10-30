package filemanager

import (
	"bytes"
	"encoding/json"
	"github.com/washsky/mygpt-test/internal/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testLinker struct{}

func (testLinker) Attach(string, string, string) error { return nil }
func (testLinker) Detach(string) error                 { return nil }

func TestFileRecycleBinRequiresAdmin(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.Save("note.txt", "text/plain", Association{}, bytes.NewBufferString("note"))
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, store, testLinker{})
	accounts, err := auth.Open(t.TempDir(), func() (bool, error) { return true, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer accounts.Close()
	accounts.Register(mux)
	handler := accounts.Wrap(mux)
	reg := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(`{"username":"owner","password":"correct horse battery"}`))
	reg.Header.Set("Content-Type", "application/json")
	registered := httptest.NewRecorder()
	handler.ServeHTTP(registered, reg)
	if registered.Code != 201 {
		t.Fatal(registered.Body.String())
	}
	login := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"owner","password":"correct horse battery"}`))
	login.Header.Set("Content-Type", "application/json")
	logged := httptest.NewRecorder()
	handler.ServeHTTP(logged, login)
	if logged.Code != 200 {
		t.Fatal(logged.Body.String())
	}
	cookie := logged.Result().Cookies()[0]
	var session struct {
		CSRF string `json:"csrf_token"`
	}
	json.Unmarshal(logged.Body.Bytes(), &session)
	request := func(method, path string, admin bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if admin {
			req.AddCookie(cookie)
			req.Header.Set("X-CSRF-Token", session.CSRF)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	if response := request(http.MethodDelete, "/api/files/"+record.ID, false); response.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated delete status = %d", response.Code)
	}
	if _, err := store.Get(record.ID); err != nil {
		t.Fatalf("unauthenticated delete changed file: %v", err)
	}
	if response := request(http.MethodDelete, "/api/files/"+record.ID, true); response.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, "/api/files/trash", false); response.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated trash status = %d", response.Code)
	}
	if response := request(http.MethodPost, "/api/files/"+record.ID+"/restore", true); response.Code != http.StatusOK {
		t.Fatalf("restore status = %d body=%s", response.Code, response.Body.String())
	}
	if _, err := store.Get(record.ID); err != nil {
		t.Fatalf("restored file unavailable: %v", err)
	}
}
