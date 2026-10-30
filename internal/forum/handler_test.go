package forum

import (
	"encoding/json"
	"github.com/washsky/mygpt-test/internal/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/washsky/mygpt-test/internal/filemanager"
)

func TestForumTrashAndSearchAPI(t *testing.T) {
	root := t.TempDir()
	files, err := filemanager.NewStore(root + "/files")
	if err != nil {
		t.Fatal(err)
	}
	store, err := Open(root, files)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	topics, err := store.Topics()
	if err != nil {
		t.Fatal(err)
	}
	post, err := store.CreatePost(topics[0].ID, "needle title", "body has needle", "tester")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	(&Handler{Store: store}).Register(mux)
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
	if response := request(http.MethodDelete, "/api/forum/posts/"+post.ID, false); response.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated delete status = %d", response.Code)
	}
	if response := request(http.MethodDelete, "/api/forum/posts/"+post.ID, true); response.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, "/api/forum/posts/"+post.ID, false); response.Code != http.StatusNotFound {
		t.Fatalf("trashed post lookup status = %d", response.Code)
	}
	if response := request(http.MethodGet, "/api/forum/trash", true); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), post.ID) {
		t.Fatalf("trash response status=%d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, "/api/forum/trash/posts/"+post.ID+"/restore", true); response.Code != http.StatusNoContent {
		t.Fatalf("restore status = %d body=%s", response.Code, response.Body.String())
	}
	if response := request(http.MethodGet, "/api/forum/posts?q=needle", false); response.Code != http.StatusForbidden {
		t.Fatalf("search while disabled status = %d", response.Code)
	}
	if err := store.SaveSettings(Settings{Title: "test", EnableSearch: true}); err != nil {
		t.Fatal(err)
	}
	if response := request(http.MethodGet, "/api/forum/posts?q=needle", false); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), post.ID) {
		t.Fatalf("search response status=%d body=%s", response.Code, response.Body.String())
	}
}
