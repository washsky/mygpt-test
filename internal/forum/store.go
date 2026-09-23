package forum

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/washsky/mygpt-test/internal/filemanager"
)

type Settings struct {
	Title string `json:"title"`
	Description string `json:"description"`
	AllowGuestPosts bool `json:"allow_guest_posts"`
	AllowGuestReplies bool `json:"allow_guest_replies"`
}

type Topic struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Description string `json:"description"`
	CreatedAt string `json:"created_at"`
}

type Post struct {
	ID string `json:"id"`
	TopicID string `json:"topic_id"`
	Title string `json:"title"`
	Body string `json:"body"`
	Author string `json:"author"`
	CreatedAt string `json:"created_at"`
	ReplyCount int `json:"reply_count"`
	Attachments []filemanager.File `json:"attachments"`
	Replies []Reply `json:"replies"`
}

type Reply struct {
	ID string `json:"id"`
	PostID string `json:"post_id"`
	Body string `json:"body"`
	Author string `json:"author"`
	CreatedAt string `json:"created_at"`
	Attachments []filemanager.File `json:"attachments"`
}

type Store struct {
	mu sync.Mutex
	catalog *sql.DB
	dir string
	files filemanager.Store
}

var ErrNotFound = errors.New("record not found")
var ErrConflict = errors.New("file is already attached to another record")

func Open(dir string, files filemanager.Store) (*Store, error) {
	db, err := sql.Open("sqlite3", filepath.Join(dir, "catalog.sqlite"))
	if err != nil { return nil, err }
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"PRAGMA foreign_keys=ON",
		"CREATE TABLE IF NOT EXISTS topics (id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL, created_at TEXT NOT NULL)",
		"CREATE TABLE IF NOT EXISTS post_index (id TEXT PRIMARY KEY, topic_id TEXT NOT NULL REFERENCES topics(id), title TEXT NOT NULL, author TEXT NOT NULL, created_at TEXT NOT NULL, shard TEXT NOT NULL, reply_count INTEGER NOT NULL DEFAULT 0)",
		"CREATE INDEX IF NOT EXISTS posts_by_topic ON post_index(topic_id, created_at DESC)",
		"CREATE TABLE IF NOT EXISTS reply_index (id TEXT PRIMARY KEY, post_id TEXT NOT NULL REFERENCES post_index(id), shard TEXT NOT NULL)",
		"CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)",
		"CREATE TABLE IF NOT EXISTS file_links (file_id TEXT PRIMARY KEY, entity_type TEXT NOT NULL, entity_id TEXT NOT NULL)",
		"CREATE INDEX IF NOT EXISTS links_by_entity ON file_links(entity_type, entity_id)",
	} {
		if _, err = db.Exec(statement); err != nil { db.Close(); return nil, fmt.Errorf("initialize forum catalog: %w", err) }
	}
	s := &Store{catalog:db, dir:dir, files:files}
	// The welcome topic is created only for an empty installation.
	var count int
	if err := db.QueryRow("SELECT count(*) FROM topics").Scan(&count); err != nil { db.Close(); return nil, err }
	if count == 0 { if _, err := s.CreateTopic("公告", "欢迎来到论坛。可以在这里发布第一篇文章。"); err != nil { db.Close(); return nil, err } }
	return s, nil
}

func (s *Store) Close() error { return s.catalog.Close() }

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil { return "", err }
	return hex.EncodeToString(b), nil
}

func (s *Store) Settings() (Settings, error) {
	v := Settings{Title:"我的论坛", Description:"分享想法，记录文章。", AllowGuestPosts:true, AllowGuestReplies:true}
	rows, err := s.catalog.Query("SELECT key, value FROM settings")
	if err != nil { return v, err }
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil { return v, err }
		switch key { case "title": v.Title=value; case "description": v.Description=value; case "allow_guest_posts": v.AllowGuestPosts=value=="true"; case "allow_guest_replies": v.AllowGuestReplies=value=="true" }
	}
	return v, rows.Err()
}

func (s *Store) SaveSettings(v Settings) error {
	v.Title=strings.TrimSpace(v.Title); v.Description=strings.TrimSpace(v.Description)
	if len(v.Title)<1 || len(v.Title)>100 || len(v.Description)>500 { return fmt.Errorf("站点名称需在 1–100 字节，简介不超过 500 字节") }
	s.mu.Lock(); defer s.mu.Unlock()
	tx, err := s.catalog.Begin(); if err != nil { return err }; defer tx.Rollback()
	values := map[string]string{"title":v.Title, "description":v.Description, "allow_guest_posts":fmt.Sprint(v.AllowGuestPosts), "allow_guest_replies":fmt.Sprint(v.AllowGuestReplies)}
	for key, value := range values { if _, err := tx.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value",key,value); err != nil { return err } }
	return tx.Commit()
}

func (s *Store) CreateTopic(name, description string) (Topic, error) {
	name=strings.TrimSpace(name); description=strings.TrimSpace(description)
	if len(name)<1 || len(name)>100 || len(description)>500 { return Topic{}, fmt.Errorf("主题名称需在 1–100 字节，简介不超过 500 字节") }
	id, err := newID(); if err != nil { return Topic{}, err }
	v:=Topic{ID:id,Name:name,Description:description,CreatedAt:time.Now().UTC().Format(time.RFC3339Nano)}
	_, err=s.catalog.Exec("INSERT INTO topics(id,name,description,created_at) VALUES(?,?,?,?)",v.ID,v.Name,v.Description,v.CreatedAt)
	return v,err
}

func (s *Store) Topics() ([]Topic,error) {
	rows,err:=s.catalog.Query("SELECT id,name,description,created_at FROM topics ORDER BY created_at ASC"); if err!=nil{return nil,err}; defer rows.Close()
	out:=[]Topic{}; for rows.Next(){var v Topic; if err:=rows.Scan(&v.ID,&v.Name,&v.Description,&v.CreatedAt);err!=nil{return nil,err};out=append(out,v)}
	return out,rows.Err()
}

func (s *Store) openShard(key string) (*sql.DB,error) {
	// Shard names are generated from timestamps or selected from our own index.
	if len(key)!=7 || key[4]!='-' { return nil,fmt.Errorf("invalid shard") }
	for i,c:=range key { if i!=4 && (c<'0'||c>'9') { return nil,fmt.Errorf("invalid shard") } }
	db,err:=sql.Open("sqlite3",filepath.Join(s.dir,"posts-"+key+".sqlite")); if err!=nil{return nil,err}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE IF NOT EXISTS posts (id TEXT PRIMARY KEY, body TEXT NOT NULL)",
		"CREATE TABLE IF NOT EXISTS replies (id TEXT PRIMARY KEY, post_id TEXT NOT NULL, author TEXT NOT NULL, body TEXT NOT NULL, created_at TEXT NOT NULL)",
		"CREATE INDEX IF NOT EXISTS replies_by_post ON replies(post_id,created_at)",
	} {
		if _,err=db.Exec(statement); err!=nil {db.Close();return nil,err}
	}
	return db,nil
}

func (s *Store) CreatePost(topicID,title,body,author string) (Post,error) {
	title=strings.TrimSpace(title);body=strings.TrimSpace(body);author=strings.TrimSpace(author)
	if len(title)<1||len(title)>200||len(body)<1||len(body)>100000||len(author)<1||len(author)>80 {return Post{},fmt.Errorf("标题、正文或署名长度无效")}
	var exists int
	if err:=s.catalog.QueryRow("SELECT 1 FROM topics WHERE id=?",topicID).Scan(&exists);err!=nil {if errors.Is(err,sql.ErrNoRows){return Post{},ErrNotFound};return Post{},err}
	id,err:=newID();if err!=nil{return Post{},err}
	now:=time.Now().UTC();key:=now.Format("2006-01")
	v:=Post{ID:id,TopicID:topicID,Title:title,Body:body,Author:author,CreatedAt:now.Format(time.RFC3339Nano),Attachments:[]filemanager.File{},Replies:[]Reply{}}
	s.mu.Lock();defer s.mu.Unlock()
	shard,err:=s.openShard(key);if err!=nil{return Post{},err};defer shard.Close()
	if _,err=shard.Exec("INSERT INTO posts(id,body) VALUES(?,?)",id,body);err!=nil{return Post{},err}
	if _,err=s.catalog.Exec("INSERT INTO post_index(id,topic_id,title,author,created_at,shard) VALUES(?,?,?,?,?,?)",id,topicID,title,author,v.CreatedAt,key);err!=nil {shard.Exec("DELETE FROM posts WHERE id=?",id);return Post{},err}
	return v,nil
}

func (s *Store) Posts(topicID string,limit,offset int) ([]Post,error) {
	if limit<1||limit>50 {limit=20};if offset<0||offset>100000 {offset=0}
	query:="SELECT id,topic_id,title,author,created_at,reply_count FROM post_index"
	args:=[]any{};if topicID!="" {query+=" WHERE topic_id=?";args=append(args,topicID)}
	query+=" ORDER BY created_at DESC LIMIT ? OFFSET ?";args=append(args,limit,offset)
	rows,err:=s.catalog.Query(query,args...);if err!=nil{return nil,err};defer rows.Close()
	out:=[]Post{};for rows.Next(){var v Post;if err:=rows.Scan(&v.ID,&v.TopicID,&v.Title,&v.Author,&v.CreatedAt,&v.ReplyCount);err!=nil{return nil,err};out=append(out,v)}
	return out,rows.Err()
}

func (s *Store) Post(id string) (Post,error) {
	var v Post;var key string
	err:=s.catalog.QueryRow("SELECT id,topic_id,title,author,created_at,reply_count,shard FROM post_index WHERE id=?",id).Scan(&v.ID,&v.TopicID,&v.Title,&v.Author,&v.CreatedAt,&v.ReplyCount,&key)
	if errors.Is(err,sql.ErrNoRows){return v,ErrNotFound};if err!=nil{return v,err}
	shard,err:=s.openShard(key);if err!=nil{return v,err};defer shard.Close()
	if err=shard.QueryRow("SELECT body FROM posts WHERE id=?",id).Scan(&v.Body);err!=nil{return v,err}
	v.Attachments,err=s.attachments("post",id);if err!=nil{return v,err}
	rows,err:=shard.Query("SELECT id,post_id,author,body,created_at FROM replies WHERE post_id=? ORDER BY created_at ASC LIMIT 500",id);if err!=nil{return v,err}
	v.Replies=[]Reply{}
	for rows.Next(){var reply Reply;if err=rows.Scan(&reply.ID,&reply.PostID,&reply.Author,&reply.Body,&reply.CreatedAt);err!=nil{rows.Close();return v,err};v.Replies=append(v.Replies,reply)}
	err=rows.Err();rows.Close();if err!=nil{return v,err}
	for i:=range v.Replies {v.Replies[i].Attachments,err=s.attachments("reply",v.Replies[i].ID);if err!=nil{return v,err}}
	return v,nil
}

func (s *Store) CreateReply(postID,body,author string) (Reply,error) {
	body=strings.TrimSpace(body);author=strings.TrimSpace(author)
	if len(body)<1||len(body)>20000||len(author)<1||len(author)>80{return Reply{},fmt.Errorf("回复或署名长度无效")}
	var key string
	if err:=s.catalog.QueryRow("SELECT shard FROM post_index WHERE id=?",postID).Scan(&key);errors.Is(err,sql.ErrNoRows){return Reply{},ErrNotFound}else if err!=nil{return Reply{},err}
	id,err:=newID();if err!=nil{return Reply{},err}
	v:=Reply{ID:id,PostID:postID,Body:body,Author:author,CreatedAt:time.Now().UTC().Format(time.RFC3339Nano),Attachments:[]filemanager.File{}}
	s.mu.Lock();defer s.mu.Unlock()
	shard,err:=s.openShard(key);if err!=nil{return Reply{},err};defer shard.Close()
	if _,err=shard.Exec("INSERT INTO replies(id,post_id,author,body,created_at) VALUES(?,?,?,?,?)",id,postID,author,body,v.CreatedAt);err!=nil{return Reply{},err}
	tx,err:=s.catalog.Begin();if err!=nil{shard.Exec("DELETE FROM replies WHERE id=?",id);return Reply{},err};defer tx.Rollback()
	if _,err=tx.Exec("INSERT INTO reply_index(id,post_id,shard) VALUES(?,?,?)",id,postID,key);err!=nil{shard.Exec("DELETE FROM replies WHERE id=?",id);return Reply{},err}
	if _,err=tx.Exec("UPDATE post_index SET reply_count=reply_count+1 WHERE id=?",postID);err!=nil{shard.Exec("DELETE FROM replies WHERE id=?",id);return Reply{},err}
	if err=tx.Commit();err!=nil{shard.Exec("DELETE FROM replies WHERE id=?",id);return Reply{},err}
	return v,nil
}

func (s *Store) attachments(kind,id string) ([]filemanager.File,error) {
	rows,err:=s.catalog.Query("SELECT file_id FROM file_links WHERE entity_type=? AND entity_id=? ORDER BY rowid",kind,id);if err!=nil{return nil,err}
	ids:=[]string{};for rows.Next(){var fileID string;if err=rows.Scan(&fileID);err!=nil{rows.Close();return nil,err};ids=append(ids,fileID)}
	err=rows.Err();rows.Close();if err!=nil{return nil,err}
	out:=[]filemanager.File{};for _,fileID:=range ids {file,err:=s.files.Get(fileID);if errors.Is(err,filemanager.ErrNotFound){continue};if err!=nil{return nil,err};out=append(out,file)}
	return out,nil
}

// Attach records the file relation in SQLite. The file metadata remains readable
// by older binaries; new content resolves attachments through this index.
func (s *Store) Attach(fileID,kind,id string) error {
	if kind!="post"&&kind!="reply" {return fmt.Errorf("only posts and replies accept attachments")}
	if _,err:=s.files.Get(fileID);err!=nil{return err}
	if kind=="post" {var found int;if err:=s.catalog.QueryRow("SELECT 1 FROM post_index WHERE id=?",id).Scan(&found);errors.Is(err,sql.ErrNoRows){return ErrNotFound}else if err!=nil{return err}}
	if kind=="reply" {var found int;if err:=s.catalog.QueryRow("SELECT 1 FROM reply_index WHERE id=?",id).Scan(&found);errors.Is(err,sql.ErrNoRows){return ErrNotFound}else if err!=nil{return err}}
	s.mu.Lock();defer s.mu.Unlock()
	var oldKind,oldID string
	err:=s.catalog.QueryRow("SELECT entity_type,entity_id FROM file_links WHERE file_id=?",fileID).Scan(&oldKind,&oldID)
	if err==nil && (oldKind!=kind||oldID!=id){return ErrConflict};if err!=nil&&!errors.Is(err,sql.ErrNoRows){return err}
	_,err=s.catalog.Exec("INSERT OR IGNORE INTO file_links(file_id,entity_type,entity_id) VALUES(?,?,?)",fileID,kind,id)
	return err
}

func (s *Store) Detach(fileID string) error {
	_,err:=s.catalog.Exec("DELETE FROM file_links WHERE file_id=?",fileID);return err
}

func (s *Store) AdminToken(configDir string) (string,error) {
	path:=filepath.Join(configDir,"admin-token")
	data,err:=os.ReadFile(path)
	if errors.Is(err,os.ErrNotExist){token,err:=newID();if err!=nil{return "",err};file,err:=os.OpenFile(path,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600);if errors.Is(err,os.ErrExist){return s.AdminToken(configDir)};if err!=nil{return "",err};_,err=file.WriteString(token+"\n");closeErr:=file.Close();if err!=nil{return "",err};if closeErr!=nil{return "",closeErr};return token,nil}
	if err!=nil{return "",err};token:=strings.TrimSpace(string(data));if len(token)!=32{return "",fmt.Errorf("invalid admin token in %s",path)};return token,nil
}
