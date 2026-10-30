package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/washsky/mygpt-test/internal/auth"
	"github.com/washsky/mygpt-test/internal/filemanager"
	"github.com/washsky/mygpt-test/internal/forum"
)

type testSession struct {
	cookie *http.Cookie
	csrf   string
}

func TestAccountsAndGuestPermissions(t *testing.T) {
	root := t.TempDir()
	files, err := filemanager.NewStore(root + "/files")
	if err != nil {
		t.Fatal(err)
	}
	board, err := forum.Open(root, files)
	if err != nil {
		t.Fatal(err)
	}
	defer board.Close()
	accounts, err := auth.Open(root, func() (bool, error) { v, e := board.Settings(); return v.AllowRegistration, e })
	if err != nil {
		t.Fatal(err)
	}
	defer accounts.Close()
	mux := http.NewServeMux()
	accounts.Register(mux)
	(&forum.Handler{Store: board, Accounts: accounts}).Register(mux)
	filemanager.RegisterRoutes(mux, files, board)
	handler := sameOriginWrites(accounts.Wrap(guestPermissions(board, mux)))
	sequence := 0
	request := func(method, path, body string, session testSession, token string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://example.com")
		sequence++
		r.RemoteAddr = "192.0.2." + strconv.Itoa(sequence) + ":1234"
		if session.cookie != nil {
			r.AddCookie(session.cookie)
			r.Header.Set("X-CSRF-Token", session.csrf)
		}
		if token != "" {
			r.Header.Set("X-Admin-Token", token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	check := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("status=%d want=%d body=%s", w.Code, status, w.Body.String())
		}
	}
	login := func(name string) testSession {
		t.Helper()
		w := request("POST", "/api/auth/login", `{"username":"`+name+`","password":"correct horse battery"}`, testSession{}, "")
		check(w, 200)
		var data struct {
			CSRF string `json:"csrf_token"`
		}
		json.Unmarshal(w.Body.Bytes(), &data)
		cookies := w.Result().Cookies()
		if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
			t.Fatal("unsafe session cookie")
		}
		return testSession{cookies[0], data.CSRF}
	}
	check(request("POST", "/api/auth/setup", `{"username":"hijacker","password":"correct horse battery"}`, testSession{}, "owner-secret"), 404)
	wFirst := request("POST", "/api/auth/register", `{"username":"owner","password":"correct horse battery"}`, testSession{}, "")
	check(wFirst, 201)
	var first struct {
		User auth.User `json:"user"`
	}
	json.Unmarshal(wFirst.Body.Bytes(), &first)
	if first.User.Role != "admin" || first.User.UID != 1 {
		t.Fatalf("first registration: %+v", first.User)
	}
	for _, name := range []string{"alice", "bob"} {
		check(request("POST", "/api/auth/register", `{"username":"`+name+`","password":"correct horse battery"}`, testSession{}, ""), 201)
	}
	check(request("POST", "/api/auth/register", `{"username":"evil","password":"correct horse battery","role":"admin"}`, testSession{}, ""), 400)
	check(request("POST", "/api/auth/login", `{"username":"alice","password":"incorrect password"}`, testSession{}, ""), 401)
	alice, bob, admin := login("alice"), login("bob"), login("owner")
	check(request("GET", "/api/auth/users", "", alice, ""), 403)
	check(request("GET", "/api/auth/users", "", admin, ""), 200)
	check(request("GET", "/api/auth/users", "", testSession{}, "owner-secret"), 403)
	check(request("PUT", "/api/forum/settings", `{}`, testSession{}, "owner-secret"), 403)
	// No CSRF token and a forged token must both fail with a valid cookie.
	check(request("POST", "/api/auth/logout", "{}", testSession{alice.cookie, ""}, ""), 403)
	check(request("POST", "/api/auth/logout", "{}", testSession{alice.cookie, "forged"}, ""), 403)
	topics, _ := board.Topics()
	topic := topics[0].ID
	postBody := `{"topic_id":"` + topic + `","title":"hello","body":"public note","author":"forged owner"}`
	w := request("POST", "/api/forum/posts", postBody, alice, "")
	check(w, 201)
	var post forum.Post
	json.Unmarshal(w.Body.Bytes(), &post)
	if post.UserID == "" || post.Author != "alice" || post.UserUID != 2 {
		t.Fatalf("untrusted author: %+v", post)
	}
	w = request("GET", "/api/forum/posts/"+post.ID, "", testSession{}, "")
	check(w, 200)
	if !strings.Contains(w.Body.String(), post.UserID) || !strings.Contains(w.Body.String(), `"user_uid":2`) {
		t.Fatal("lost persistent authorship")
	}
	w = request("GET", "/api/forum/posts", "", testSession{}, "")
	check(w, 200)
	if !strings.Contains(w.Body.String(), post.UserID) || !strings.Contains(w.Body.String(), `"user_uid":2`) {
		t.Fatal("list lost authorship")
	}
	check(request("PUT", "/api/forum/posts/"+post.ID, `{"title":"changed","body":"owned"}`, bob, ""), 403)
	check(request("PUT", "/api/forum/posts/"+post.ID, `{"title":"changed","body":"owned"}`, alice, ""), 204)
	check(request("DELETE", "/api/forum/posts/"+post.ID, "", testSession{}, ""), 403)
	w = request("POST", "/api/forum/posts/"+post.ID+"/replies", `{"body":"hi","author":"owner"}`, alice, "")
	check(w, 201)
	var reply forum.Reply
	json.Unmarshal(w.Body.Bytes(), &reply)
	if reply.UserID != post.UserID || reply.Author != "alice" || reply.UserUID != 2 {
		t.Fatal("reply identity forged")
	}
	check(request("PUT", "/api/forum/replies/"+reply.ID, `{"body":"edited"}`, bob, ""), 403)
	check(request("PUT", "/api/forum/replies/"+reply.ID, `{"body":"edited"}`, alice, ""), 204)
	// Standalone and guest-authored content stay available under the default policy.
	w = request("POST", "/api/forum/posts", postBody, testSession{}, "")
	check(w, 201)
	var guestPost forum.Post
	json.Unmarshal(w.Body.Bytes(), &guestPost)
	if guestPost.UserID != "" {
		t.Fatal("guest claimed an account")
	}
	check(request("POST", "/api/forum/posts/"+post.ID+"/replies", `{"body":"guest reply","author":"alice"}`, testSession{}, ""), 201)
	upload := func(session testSession, related string) *httptest.ResponseRecorder {
		t.Helper()
		var b bytes.Buffer
		m := multipart.NewWriter(&b)
		file, _ := m.CreateFormFile("file", "test.txt")
		file.Write([]byte("public attachment"))
		if related != "" {
			m.WriteField("related_type", "post")
			m.WriteField("related_id", related)
		}
		m.Close()
		r := httptest.NewRequest("POST", "/api/files", &b)
		r.Header.Set("Content-Type", m.FormDataContentType())
		if session.cookie != nil {
			r.AddCookie(session.cookie)
			r.Header.Set("X-CSRF-Token", session.csrf)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	check(upload(testSession{}, ""), 201)
	check(upload(testSession{}, post.ID), 403)
	check(upload(bob, post.ID), 403)
	check(upload(alice, post.ID), 201)
	// Turning off guest writes does not prevent member posts, replies or uploads.
	settings, _ := board.Settings()
	settings.AllowGuestPosts = false
	settings.AllowGuestReplies = false
	settings.AllowGuestUploads = false
	settings.AllowGuestClipboard = false
	settings.AllowRegistration = false
	encoded, _ := json.Marshal(settings)
	check(request("PUT", "/api/forum/settings", string(encoded), alice, ""), 403)
	check(request("PUT", "/api/forum/settings", string(encoded), admin, ""), 200)
	check(request("POST", "/api/forum/posts", postBody, testSession{}, ""), 403)
	check(request("POST", "/api/forum/posts", postBody, alice, ""), 201)
	check(request("POST", "/api/forum/posts/"+post.ID+"/replies", `{"body":"member reply","author":"other"}`, alice, ""), 201)
	check(request("POST", "/api/forum/posts/"+post.ID+"/replies", `{"body":"guest reply","author":"other"}`, testSession{}, ""), 403)
	check(upload(testSession{}, ""), 401)
	check(upload(alice, ""), 201)
	check(request("GET", "/api/clipboard", "", testSession{}, ""), 401)
	check(request("POST", "/api/auth/register", `{"username":"carol","password":"correct horse battery"}`, testSession{}, ""), 403)
	check(request("GET", "/api/forum/posts/"+post.ID, "", testSession{}, ""), 200)
	// Guest settings are persisted, and existing unowned content remains admin managed.
	check(request("PUT", "/api/forum/posts/"+guestPost.ID, `{"title":"stolen","body":"stolen"}`, alice, ""), 403)
	check(request("DELETE", "/api/forum/posts/"+post.ID, "", alice, ""), 204)
	check(request("POST", "/api/forum/trash/posts/"+post.ID+"/restore", "", alice, ""), 403)
	check(request("POST", "/api/forum/trash/posts/"+post.ID+"/restore", "", admin, ""), 204)
	// Disabled accounts lose every session; enabling does not revive old cookies.
	w = request("GET", "/api/auth/me", "", bob, "")
	var me struct {
		User auth.User `json:"user"`
	}
	json.Unmarshal(w.Body.Bytes(), &me)
	check(request("PUT", "/api/auth/users/"+me.User.ID, `{"disabled":true}`, admin, ""), 204)
	check(request("POST", "/api/forum/posts", postBody, bob, ""), 401)
	check(request("PUT", "/api/auth/users/"+me.User.ID, `{"disabled":false}`, admin, ""), 204)
	check(request("POST", "/api/forum/posts", postBody, bob, ""), 401)
	// Password changes revoke both the current and other-device sessions.
	otherAlice := login("alice")
	check(request("POST", "/api/auth/password", `{"current_password":"wrong","new_password":"replacement password"}`, alice, ""), 403)
	check(request("POST", "/api/auth/password", `{"current_password":"correct horse battery","new_password":"replacement password"}`, alice, ""), 204)
	check(request("POST", "/api/forum/posts", postBody, otherAlice, ""), 401)
	check(request("POST", "/api/auth/login", `{"username":"alice","password":"correct horse battery"}`, testSession{}, ""), 401)
	check(request("POST", "/api/auth/login", `{"username":"alice","password":"replacement password"}`, testSession{}, ""), 200)
	check(request("POST", "/api/auth/logout", "{}", admin, ""), 204)
	check(request("PUT", "/api/forum/settings", string(encoded), admin, ""), 401)
}

func TestCrossOriginWritesRejected(t *testing.T) {
	h := sameOriginWrites(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, headers := range []map[string]string{{"Origin": "https://evil.example"}, {"Sec-Fetch-Site": "cross-site"}, {"Origin": "null"}} {
		r := httptest.NewRequest("POST", "http://blog.test/api/auth/login", nil)
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("accepted cross-origin request: %v", headers)
		}
	}
}
