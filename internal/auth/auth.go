// Package auth provides local accounts and revocable, server-side sessions.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

const cookieName = "mygpt_session"
const sessionTTL = 7 * 24 * time.Hour

type User struct {
	UID      int64  `json:"uid"`
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Disabled bool   `json:"disabled"`
}
type identity struct {
	User *User
	CSRF string
}
type contextKey struct{}

func Current(r *http.Request) *User {
	v, _ := r.Context().Value(contextKey{}).(identity)
	return v.User
}
func IsAdmin(r *http.Request) bool { user := Current(r); return user != nil && user.Role == "admin" }

func random() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}
func digest(s string) string { d := sha256.Sum256([]byte(s)); return hex.EncodeToString(d[:]) }

type window struct {
	Count int
	Until time.Time
}
type Service struct {
	db           *sql.DB
	registration func() (bool, error)
	secure       bool
	dummy        string
	mu           sync.Mutex
	attempts     map[string]window
	workers      chan struct{}
}

func Open(dir string, registration func() (bool, error)) (*Service, error) {
	path := filepath.Join(dir, "accounts.sqlite")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	f.Close()
	if err = os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		"PRAGMA foreign_keys=ON",
		"CREATE TABLE IF NOT EXISTS users(id TEXT PRIMARY KEY,username TEXT NOT NULL UNIQUE,password TEXT NOT NULL,role TEXT NOT NULL CHECK(role IN ('member','admin')),disabled INTEGER NOT NULL DEFAULT 0)",
		"CREATE TABLE IF NOT EXISTS sessions(hash TEXT PRIMARY KEY,user_id TEXT NOT NULL REFERENCES users(id),csrf TEXT NOT NULL,expires INTEGER NOT NULL)",
		"CREATE INDEX IF NOT EXISTS sessions_by_user ON sessions(user_id)",
		"CREATE TABLE IF NOT EXISTS user_uids(uid INTEGER PRIMARY KEY AUTOINCREMENT,user_id TEXT NOT NULL UNIQUE REFERENCES users(id))",
	} {
		if _, err = db.Exec(q); err != nil {
			db.Close()
			return nil, err
		}
	}
	// Keep the original account IDs and sessions intact while assigning public UIDs.
	tx, err := db.Begin()
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err = tx.Exec("INSERT INTO user_uids(user_id) SELECT id FROM users WHERE id NOT IN (SELECT user_id FROM user_uids) ORDER BY rowid"); err == nil {
		_, err = tx.Exec("UPDATE users SET role='admin' WHERE id=(SELECT user_id FROM user_uids ORDER BY uid LIMIT 1) AND NOT EXISTS(SELECT 1 FROM users WHERE role='admin')")
	}
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		db.Close()
		return nil, err
	}
	dummy, err := hashPassword("unavailable-account-password")
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Service{db: db, registration: registration, secure: os.Getenv("MYGPT_SECURE_COOKIE") == "1", dummy: dummy, attempts: map[string]window{}, workers: make(chan struct{}, 4)}, nil
}
func (s *Service) Close() error { return s.db.Close() }

// Wrap rejects stale cookies instead of silently turning expired users into guests.
// Only sessions, never passwords or raw cookie values, enter the request context.
func (s *Service) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		cookie, err := r.Cookie(cookieName)
		if err == nil {
			var u User
			var csrf string
			err = s.db.QueryRow("SELECT u.id,u.username,u.role,u.disabled,s.csrf,n.uid FROM sessions s JOIN users u ON u.id=s.user_id JOIN user_uids n ON n.user_id=u.id WHERE s.hash=? AND s.expires>? AND u.disabled=0", digest(cookie.Value), time.Now().Unix()).Scan(&u.ID, &u.Username, &u.Role, &u.Disabled, &csrf, &u.UID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "身份验证暂时不可用", 503)
				return
			}
			if err == nil {
				if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(csrf)) != 1 {
					http.Error(w, "会话校验失败，请刷新页面", 403)
					return
				}
				r = r.WithContext(context.WithValue(r.Context(), contextKey{}, identity{&u, csrf}))
			} else {
				s.clearCookie(w, r)
				if r.Method != "GET" && r.Method != "HEAD" && r.URL.Path != "/api/auth/login" && r.URL.Path != "/api/auth/register" && r.URL.Path != "/api/auth/logout" {
					http.Error(w, "登录已过期，请重新登录或刷新后以访客继续", 401)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /account", s.page)
	mux.HandleFunc("GET /api/auth/me", s.me)
	mux.HandleFunc("POST /api/auth/register", s.limited(s.register))
	mux.HandleFunc("POST /api/auth/login", s.limited(s.login))
	mux.HandleFunc("POST /api/auth/logout", s.logout)
	mux.HandleFunc("POST /api/auth/password", s.limited(s.password))
	mux.HandleFunc("GET /api/auth/users", s.users)
	mux.HandleFunc("PUT /api/auth/users/{id}", s.disable)
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func input(w http.ResponseWriter, r *http.Request, v any) bool {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		http.Error(w, "请使用 JSON 请求", 415)
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		http.Error(w, "请求格式无效", 400)
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		http.Error(w, "请求格式无效", 400)
		return false
	}
	return true
}

// Address keys deliberately ignore spoofable proxy headers. A proxy may share a quota.
func (s *Service) limited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		now := time.Now()
		s.mu.Lock()
		for key, v := range s.attempts {
			if now.After(v.Until) {
				delete(s.attempts, key)
			}
		}
		v := s.attempts[host]
		global := s.attempts["*"]
		if v.Count >= 10 || global.Count >= 120 || len(s.attempts) >= 4096 {
			s.mu.Unlock()
			w.Header().Set("Retry-After", "60")
			http.Error(w, "请求过于频繁，请稍后重试", 429)
			return
		}
		if v.Count == 0 {
			v.Until = now.Add(time.Minute)
		}
		v.Count++
		s.attempts[host] = v
		if global.Count == 0 {
			global.Until = now.Add(time.Minute)
		}
		global.Count++
		s.attempts["*"] = global
		s.mu.Unlock()
		select {
		case s.workers <- struct{}{}:
			defer func() { <-s.workers }()
			next(w, r)
		default:
			w.Header().Set("Retry-After", "5")
			http.Error(w, "登录服务繁忙，请稍后重试", 429)
		}
	}
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{2,31}$`)

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func validPassword(p string) bool { return p != "" }
func (s *Service) me(w http.ResponseWriter, r *http.Request) {
	enabled, err := s.registration()
	if err != nil {
		http.Error(w, "无法读取账户设置", 500)
		return
	}
	var count int
	if err = s.db.QueryRow("SELECT count(*) FROM users").Scan(&count); err != nil {
		http.Error(w, "无法读取账户设置", 500)
		return
	}
	id, _ := r.Context().Value(contextKey{}).(identity)
	reply(w, 200, map[string]any{"user": id.User, "csrf_token": id.CSRF, "allow_registration": enabled || count == 0, "first_registration": count == 0})
}
func (s *Service) register(w http.ResponseWriter, r *http.Request) {
	enabled, err := s.registration()
	if err != nil {
		http.Error(w, "无法读取账户设置", 500)
		return
	}
	var c credentials
	if !input(w, r, &c) {
		return
	}
	c.Username = strings.ToLower(strings.TrimSpace(c.Username))
	if !usernamePattern.MatchString(c.Username) || !validPassword(c.Password) {
		http.Error(w, "用户名需 3–32 位小写字母、数字、下划线或短横线，密码不能为空", 400)
		return
	}
	hash, err := hashPassword(c.Password)
	if err != nil {
		http.Error(w, "无法创建账户", 500)
		return
	}
	id, err := random()
	if err != nil {
		http.Error(w, "无法创建账户", 500)
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		http.Error(w, "无法创建账户", 500)
		return
	}
	defer tx.Rollback()
	var total int
	if err = tx.QueryRow("SELECT count(*) FROM users").Scan(&total); err != nil {
		http.Error(w, "无法读取账户设置", 500)
		return
	}
	if total > 0 && !enabled {
		http.Error(w, "本站已关闭注册，仍可按站点设置以访客使用", 403)
		return
	}
	role := "member"
	if total == 0 {
		role = "admin"
	}
	var n int
	if err = tx.QueryRow("SELECT count(*) FROM users WHERE username=?", c.Username).Scan(&n); err != nil {
		http.Error(w, "无法创建账户", 500)
		return
	}
	if n != 0 {
		http.Error(w, "用户名不可用", 409)
		return
	}
	_, err = tx.Exec("INSERT INTO users(id,username,password,role) VALUES(?,?,?,?)", id, c.Username, string(hash), role)
	if err != nil {
		http.Error(w, "无法创建账户", 500)
		return
	}
	result, err := tx.Exec("INSERT INTO user_uids(user_id) VALUES(?)", id)
	if err != nil {
		http.Error(w, "无法分配 UID", 500)
		return
	}
	uid, err := result.LastInsertId()
	if err != nil {
		http.Error(w, "无法分配 UID", 500)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "无法创建账户", 500)
		return
	}
	message := "账户已创建，请登录"
	if role == "admin" {
		message = "首个管理员账户已创建，请登录"
	}
	reply(w, 201, map[string]any{"message": message, "user": User{ID: id, Username: c.Username, Role: role, UID: uid}})
}
func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !input(w, r, &c) {
		return
	}
	c.Username = strings.ToLower(strings.TrimSpace(c.Username))
	var u User
	var hash string
	err := s.db.QueryRow("SELECT u.id,u.username,u.role,u.disabled,u.password,n.uid FROM users u JOIN user_uids n ON n.user_id=u.id WHERE u.username=?", c.Username).Scan(&u.ID, &u.Username, &u.Role, &u.Disabled, &hash, &u.UID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "登录暂时不可用", 503)
		return
	}
	check := hash
	if err != nil {
		check = s.dummy
	}
	if verifyPassword(check, c.Password) != nil || err != nil || u.Disabled {
		http.Error(w, "用户名或密码错误，或账户已停用", 401)
		return
	}
	raw, csrf, err := s.newSession(u.ID, hash)
	if err != nil {
		http.Error(w, "无法创建会话", 500)
		return
	}
	if old, e := r.Cookie(cookieName); e == nil {
		_, _ = s.db.Exec("DELETE FROM sessions WHERE hash=?", digest(old.Value))
	}
	s.setCookie(w, r, raw)
	reply(w, 200, map[string]any{"user": u, "csrf_token": csrf})
}
func (s *Service) newSession(id, expectedHash string) (string, string, error) {
	raw, err := random()
	if err != nil {
		return "", "", err
	}
	csrf, err := random()
	if err != nil {
		return "", "", err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	var currentHash string
	if err = tx.QueryRow("SELECT password FROM users WHERE id=? AND disabled=0", id).Scan(&currentHash); err != nil {
		return "", "", err
	}
	if currentHash != expectedHash {
		return "", "", errors.New("credentials changed")
	}
	if _, err = tx.Exec("DELETE FROM sessions WHERE expires<=?", time.Now().Unix()); err != nil {
		return "", "", err
	}
	// Keep a bounded number of active sessions for each account.
	if _, err = tx.Exec("DELETE FROM sessions WHERE user_id=? AND hash NOT IN (SELECT hash FROM sessions WHERE user_id=? ORDER BY expires DESC LIMIT 9)", id, id); err != nil {
		return "", "", err
	}
	_, err = tx.Exec("INSERT INTO sessions(hash,user_id,csrf,expires) VALUES(?,?,?,?)", digest(raw), id, csrf, time.Now().Add(sessionTTL).Unix())
	if err != nil {
		return "", "", err
	}
	return raw, csrf, tx.Commit()
}
func (s *Service) setCookie(w http.ResponseWriter, r *http.Request, value string) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: value, Path: "/", HttpOnly: true, Secure: s.secure || r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: int(sessionTTL.Seconds())})
}
func (s *Service) clearCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/", HttpOnly: true, Secure: s.secure || r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
}
func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		if _, err = s.db.Exec("DELETE FROM sessions WHERE hash=?", digest(c.Value)); err != nil {
			http.Error(w, "退出失败，请重试", 500)
			return
		}
	}
	s.clearCookie(w, r)
	w.WriteHeader(204)
}
func (s *Service) password(w http.ResponseWriter, r *http.Request) {
	u := Current(r)
	if u == nil {
		http.Error(w, "请先登录", 401)
		return
	}
	var in struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if !input(w, r, &in) {
		return
	}
	if !validPassword(in.New) {
		http.Error(w, "新密码不能为空", 400)
		return
	}
	var hash string
	if err := s.db.QueryRow("SELECT password FROM users WHERE id=?", u.ID).Scan(&hash); err != nil {
		http.Error(w, "无法读取账户", 500)
		return
	}
	if verifyPassword(hash, in.Current) != nil {
		http.Error(w, "当前密码错误", 403)
		return
	}
	next, err := hashPassword(in.New)
	if err != nil {
		http.Error(w, "修改密码失败", 500)
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		http.Error(w, "修改密码失败", 500)
		return
	}
	defer tx.Rollback()
	// Conditional update prevents two concurrent password changes using an old hash.
	result, err := tx.Exec("UPDATE users SET password=? WHERE id=? AND password=?", string(next), u.ID, hash)
	if err != nil {
		http.Error(w, "修改密码失败", 500)
		return
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		http.Error(w, "密码已更改，请重新登录", 409)
		return
	}
	if _, err = tx.Exec("DELETE FROM sessions WHERE user_id=?", u.ID); err != nil {
		http.Error(w, "无法撤销会话", 500)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "修改密码失败", 500)
		return
	}
	s.clearCookie(w, r)
	w.WriteHeader(204)
}
func (s *Service) users(w http.ResponseWriter, r *http.Request) {
	if !IsAdmin(r) {
		http.Error(w, "需要管理员身份", 403)
		return
	}
	rows, err := s.db.Query("SELECT u.id,u.username,u.role,u.disabled,n.uid FROM users u JOIN user_uids n ON n.user_id=u.id ORDER BY n.uid")
	if err != nil {
		http.Error(w, "无法读取用户", 500)
		return
	}
	defer rows.Close()
	items := []User{}
	for rows.Next() {
		var u User
		if err = rows.Scan(&u.ID, &u.Username, &u.Role, &u.Disabled, &u.UID); err != nil {
			http.Error(w, "无法读取用户", 500)
			return
		}
		items = append(items, u)
	}
	if rows.Err() != nil {
		http.Error(w, "无法读取用户", 500)
		return
	}
	reply(w, 200, map[string]any{"items": items})
}
func (s *Service) disable(w http.ResponseWriter, r *http.Request) {
	if !IsAdmin(r) {
		http.Error(w, "需要管理员身份", 403)
		return
	}
	var in struct {
		Disabled bool `json:"disabled"`
	}
	if !input(w, r, &in) {
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		http.Error(w, "无法修改用户", 500)
		return
	}
	defer tx.Rollback()
	result, err := tx.Exec("UPDATE users SET disabled=? WHERE id=? AND role='member'", in.Disabled, r.PathValue("id"))
	if err != nil {
		http.Error(w, "无法修改用户", 500)
		return
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		http.Error(w, "只能启用或停用普通用户", 400)
		return
	}
	if _, err = tx.Exec("DELETE FROM sessions WHERE user_id=?", r.PathValue("id")); err != nil {
		http.Error(w, "无法撤销会话", 500)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "无法修改用户", 500)
		return
	}
	w.WriteHeader(204)
}

// UID resolves the public registration sequence without changing content ownership.
func (s *Service) UID(id string) (int64, error) {
	var uid int64
	err := s.db.QueryRow("SELECT uid FROM user_uids WHERE user_id=?", id).Scan(&uid)
	return uid, err
}
