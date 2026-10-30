package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestFullPasswordHashAndLegacyCompatibility(t *testing.T) {
	for _, password := range []string{"a", "密", strings.Repeat("long password 密码", 600)} {
		hash, err := hashPassword(password)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(hash, passwordPrefix) {
			t.Fatal("wrong hash format")
		}
		if err = verifyPassword(hash, password); err != nil {
			t.Fatal("password did not verify")
		}
		if verifyPassword(hash, password+"different") == nil {
			t.Fatal("password suffix was ignored")
		}
	}
	if _, err := hashPassword(""); err == nil {
		t.Fatal("empty password accepted")
	}
	for _, hash := range []string{"", passwordPrefix + "invalid", "$argon2id$v=19$m=999999999,t=1,p=2$invalid$invalid"} {
		if verifyPassword(hash, "x") == nil {
			t.Fatal("malformed hash accepted")
		}
	}
	legacy, err := bcrypt.GenerateFromPassword([]byte("old password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if verifyPassword(string(legacy), "old password") != nil {
		t.Fatal("legacy account cannot log in")
	}
	if verifyPassword(string(legacy), strings.Repeat("x", 80)) == nil {
		t.Fatal("legacy bcrypt accepted excessive input")
	}
}

func TestShortAndLongPasswordsThroughAPI(t *testing.T) {
	s, err := Open(t.TempDir(), func() (bool, error) { return true, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mux := http.NewServeMux()
	s.Register(mux)
	handler := s.Wrap(mux)
	var cookie *http.Cookie
	var csrf string
	request := func(path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		data, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", path, strings.NewReader(string(data)))
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	register := func(name, password string) {
		t.Helper()
		w := request("/api/auth/register", map[string]string{"username": name, "password": password})
		if w.Code != 201 {
			t.Fatalf("registration status=%d: %s", w.Code, w.Body.String())
		}
	}
	login := func(name, password string, want int) {
		t.Helper()
		w := request("/api/auth/login", map[string]string{"username": name, "password": password})
		if w.Code != want {
			t.Fatalf("login status=%d want=%d: %s", w.Code, want, w.Body.String())
		}
		if want == 200 {
			cookie = w.Result().Cookies()[0]
			var data struct {
				CSRF string `json:"csrf_token"`
			}
			json.Unmarshal(w.Body.Bytes(), &data)
			csrf = data.CSRF
		}
	}
	register("short", "a")
	login("short", "a", 200)
	long := strings.Repeat("密码", 1200) + "end"
	w := request("/api/auth/password", map[string]string{"current_password": "a", "new_password": long})
	if w.Code != 204 {
		t.Fatalf("password change=%d: %s", w.Code, w.Body.String())
	}
	cookie = nil
	csrf = ""
	login("short", long+"wrong", 401)
	login("short", long, 200)
	register("long", long)
	// Legacy hashes remain valid until the user changes their password.
	legacy, err := bcrypt.GenerateFromPassword([]byte("legacy password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO users(id,username,password,role) VALUES('legacy','legacy',?,'member')", string(legacy)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO user_uids(user_id) VALUES('legacy')"); err != nil {
		t.Fatal(err)
	}
	cookie = nil
	csrf = ""
	login("legacy", "legacy password", 200)
	w = request("/api/auth/password", map[string]string{"current_password": "legacy password", "new_password": "新"})
	if w.Code != 204 {
		t.Fatalf("legacy password change=%d: %s", w.Code, w.Body.String())
	}
	cookie = nil
	csrf = ""
	login("legacy", "新", 200)
}
