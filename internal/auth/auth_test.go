package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPersistentSessionExpiryAndRateLimit(t *testing.T) {
	dir := t.TempDir()
	open := func() *Service {
		s, e := Open(dir, "secret", func() (bool, error) { return true, nil })
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	s := open()
	s.db.Exec("INSERT INTO users(id,username,password,role) VALUES('user','tester','hash','member')")
	raw, csrf, err := s.newSession("user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	s.db.QueryRow("SELECT hash FROM sessions").Scan(&stored)
	if stored == raw || stored != digest(raw) {
		t.Fatal("raw session stored in database")
	}
	s.Close()
	s = open()
	defer s.Close()
	handler := s.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if Current(r) == nil {
			w.WriteHeader(401)
		} else {
			w.WriteHeader(204)
		}
	}))
	request := func(method string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/example", nil)
		r.AddCookie(&http.Cookie{Name: cookieName, Value: raw})
		r.Header.Set("X-CSRF-Token", csrf)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := request("POST"); w.Code != 204 {
		t.Fatalf("session did not survive restart: %d", w.Code)
	}
	s.db.Exec("UPDATE sessions SET expires=?", time.Now().Add(-time.Second).Unix())
	if w := request("POST"); w.Code != 401 {
		t.Fatal("expired cookie became guest write")
	}
	mux := http.NewServeMux()
	s.Register(mux)
	for i := 0; i < 11; i++ {
		r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if i == 10 && w.Code != 429 {
			t.Fatalf("no rate limit: %d", w.Code)
		}
	}
}
