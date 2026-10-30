package auth

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentFirstRegistration(t *testing.T) {
	s, err := Open(t.TempDir(), func() (bool, error) { return true, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			r := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(fmt.Sprintf(`{"username":"first%d","password":"correct horse battery"}`, i)))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.register(w, r)
			results <- w
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	admins := 0
	seen := map[int64]bool{}
	for w := range results {
		if w.Code != 201 {
			t.Fatal(w.Body.String())
		}
		var data struct {
			User User `json:"user"`
		}
		if err = json.Unmarshal(w.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if seen[data.User.UID] || data.User.UID < 1 || data.User.UID > 4 {
			t.Fatalf("invalid UID: %+v", data.User)
		}
		seen[data.User.UID] = true
		if data.User.Role == "admin" {
			admins++
			if data.User.UID != 1 {
				t.Fatal("first admin UID is not 1")
			}
		}
	}
	if admins != 1 {
		t.Fatalf("concurrent first registrations produced %d admins", admins)
	}
}

func TestFirstRegistrationAndConcurrentUIDs(t *testing.T) {
	enabled := false
	s, err := Open(t.TempDir(), func() (bool, error) { return enabled, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	register := func(name string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(fmt.Sprintf(`{"username":%q,"password":"correct horse battery"}`, name)))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.register(w, r)
		return w
	}
	first := register("first")
	if first.Code != 201 {
		t.Fatal(first.Body.String())
	}
	var data struct {
		User User `json:"user"`
	}
	json.Unmarshal(first.Body.Bytes(), &data)
	if data.User.UID != 1 || data.User.Role != "admin" {
		t.Fatalf("first account = %+v", data.User)
	}
	if w := register("closed"); w.Code != 403 {
		t.Fatalf("registration ignored disabled flag: %d", w.Code)
	}
	enabled = true
	if w := register("first"); w.Code != 409 {
		t.Fatal("duplicate accepted")
	}
	if w := register("x"); w.Code != 400 {
		t.Fatal("invalid username accepted")
	}
	var wg sync.WaitGroup
	results := make(chan *httptest.ResponseRecorder, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results <- register(fmt.Sprintf("member%d", i)) }(i)
	}
	wg.Wait()
	close(results)
	seen := map[int64]bool{1: true}
	for w := range results {
		if w.Code != 201 {
			t.Fatal(w.Body.String())
		}
		json.Unmarshal(w.Body.Bytes(), &data)
		if data.User.Role != "member" || data.User.UID < 2 || data.User.UID > 5 || seen[data.User.UID] {
			t.Fatalf("invalid account: %+v", data.User)
		}
		seen[data.User.UID] = true
	}
	var admins int
	if err = s.db.QueryRow("SELECT count(*) FROM users WHERE role='admin'").Scan(&admins); err != nil || admins != 1 {
		t.Fatalf("admin count=%d err=%v", admins, err)
	}
}

func TestLegacyUIDMigration(t *testing.T) {
	for _, withAdmin := range []bool{false, true} {
		t.Run(fmt.Sprint(withAdmin), func(t *testing.T) {
			dir := t.TempDir()
			db, err := sql.Open("sqlite3", filepath.Join(dir, "accounts.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec("CREATE TABLE users(id TEXT PRIMARY KEY,username TEXT NOT NULL UNIQUE,password TEXT NOT NULL,role TEXT NOT NULL,disabled INTEGER NOT NULL DEFAULT 0)")
			if err != nil {
				t.Fatal(err)
			}
			// Lexical IDs deliberately disagree with insertion order.
			if _, err = db.Exec("INSERT INTO users VALUES('z-oldest','older','hash','member',0)"); err != nil {
				t.Fatal(err)
			}
			role := "member"
			if withAdmin {
				role = "admin"
			}
			if _, err = db.Exec("INSERT INTO users VALUES('a-newer','newer','hash',?,0)", role); err != nil {
				t.Fatal(err)
			}
			db.Close()
			s, err := Open(dir, func() (bool, error) { return true, nil })
			if err != nil {
				t.Fatal(err)
			}
			for id, want := range map[string]int64{"z-oldest": 1, "a-newer": 2} {
				uid, err := s.UID(id)
				if err != nil || uid != want {
					t.Fatalf("uid=%d want=%d err=%v", uid, want, err)
				}
			}
			var adminID string
			if err = s.db.QueryRow("SELECT id FROM users WHERE role='admin'").Scan(&adminID); err != nil {
				t.Fatal(err)
			}
			expected := "z-oldest"
			if withAdmin {
				expected = "a-newer"
			}
			if adminID != expected {
				t.Fatalf("admin=%s want=%s", adminID, expected)
			}
			raw, csrf, err := s.newSession("z-oldest", "hash")
			if err != nil {
				t.Fatal(err)
			}
			s.Close()
			s, err = Open(dir, func() (bool, error) { return true, nil })
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			r := httptest.NewRequest("GET", "/api/auth/me", nil)
			r.AddCookie(&http.Cookie{Name: cookieName, Value: raw})
			r.Header.Set("X-CSRF-Token", csrf)
			w := httptest.NewRecorder()
			s.Wrap(http.HandlerFunc(s.me)).ServeHTTP(w, r)
			if w.Code != 200 || !strings.Contains(w.Body.String(), `"uid":1`) {
				t.Fatalf("session or UID lost after restart: %s", w.Body.String())
			}
		})
	}
}
