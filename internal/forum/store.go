package forum

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	EnableResourceRendering bool `json:"enable_resource_rendering"`
	EnableSearch bool `json:"enable_search"`
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
	DeletedAt string `json:"deleted_at,omitempty"`
	Attachments []filemanager.File `json:"attachments"`
	Replies []Reply `json:"replies"`
}

type Reply struct {
	ID string `json:"id"`
	PostID string `json:"post_id"`
	Body string `json:"body"`
	Author string `json:"author"`
	CreatedAt string `json:"created_at"`
	DeletedAt string `json:"deleted_at,omitempty"`
	Attachments []filemanager.File `json:"attachments"`
}

type TrashPost struct {
	ID string `json:"id"`
	TopicID string `json:"topic_id"`
	TopicName string `json:"topic_name"`
	Title string `json:"title"`
	Body string `json:"body"`
	Author string `json:"author"`
	CreatedAt string `json:"created_at"`
	DeletedAt string `json:"deleted_at"`
	ReplyCount int `json:"reply_count"`
}

type TrashReply struct {
	ID string `json:"id"`
	PostID string `json:"post_id"`
	PostTitle string `json:"post_title"`
	Body string `json:"body"`
	Author string `json:"author"`
	CreatedAt string `json:"created_at"`
	DeletedAt string `json:"deleted_at"`
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
		"CREATE TABLE IF NOT EXISTS post_index (id TEXT PRIMARY KEY, topic_id TEXT NOT NULL REFERENCES topics(id), title TEXT NOT NULL, author TEXT NOT NULL, created_at TEXT NOT NULL, shard TEXT NOT NULL, reply_count INTEGER NOT NULL DEFAULT 0, deleted_at TEXT)",
		"CREATE INDEX IF NOT EXISTS posts_by_topic ON post_index(topic_id, created_at DESC)",
		"CREATE TABLE IF NOT EXISTS reply_index (id TEXT PRIMARY KEY, post_id TEXT NOT NULL REFERENCES post_index(id), shard TEXT NOT NULL)",
		"CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)",
		"CREATE TABLE IF NOT EXISTS file_links (file_id TEXT PRIMARY KEY, entity_type TEXT NOT NULL, entity_id TEXT NOT NULL)",
		"CREATE INDEX IF NOT EXISTS links_by_entity ON file_links(entity_type, entity_id)",
	} {
		if _, err = db.Exec(statement); err != nil { db.Close(); return nil, fmt.Errorf("initialize forum catalog: %w", err) }
	}
	if err := ensureColumn(db, "post_index", "deleted_at"); err != nil { db.Close(); return nil, fmt.Errorf("upgrade forum catalog: %w", err) }
	if _, err = db.Exec("CREATE INDEX IF NOT EXISTS posts_by_topic_active ON post_index(topic_id, deleted_at, created_at DESC)"); err != nil { db.Close(); return nil, err }
	s := &Store{catalog:db, dir:dir, files:files}
	// The welcome topic is created only for an empty installation.
	var count int
	if err := db.QueryRow("SELECT count(*) FROM topics").Scan(&count); err != nil { db.Close(); return nil, err }
	if count == 0 { if _, err := s.CreateTopic("公告", "欢迎来到论坛。可以在这里发布第一篇文章。"); err != nil { db.Close(); return nil, err } }
	return s, nil
}

func ensureColumn(db *sql.DB, table, column string) error {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil { return err }
	found := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notnull, &defaultValue, &pk); err != nil { rows.Close(); return err }
		if name == column { found = true }
	}
	if err := rows.Err(); err != nil { rows.Close(); return err }
	if err := rows.Close(); err != nil { return err }
	if !found { _, err = db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " TEXT") }
	return err
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
		switch key { case "title": v.Title=value; case "description": v.Description=value; case "allow_guest_posts": v.AllowGuestPosts=value=="true"; case "allow_guest_replies": v.AllowGuestReplies=value=="true"; case "enable_resource_rendering": v.EnableResourceRendering=value=="true"; case "enable_search": v.EnableSearch=value=="true" }
	}
	return v, rows.Err()
}

func (s *Store) SaveSettings(v Settings) error {
	v.Title=strings.TrimSpace(v.Title); v.Description=strings.TrimSpace(v.Description)
	if len(v.Title)<1 || len(v.Title)>100 || len(v.Description)>500 { return fmt.Errorf("站点名称需在 1–100 字节，简介不超过 500 字节") }
	s.mu.Lock(); defer s.mu.Unlock()
	tx, err := s.catalog.Begin(); if err != nil { return err }; defer tx.Rollback()
	values := map[string]string{"title":v.Title, "description":v.Description, "allow_guest_posts":fmt.Sprint(v.AllowGuestPosts), "allow_guest_replies":fmt.Sprint(v.AllowGuestReplies), "enable_resource_rendering":fmt.Sprint(v.EnableResourceRendering), "enable_search":fmt.Sprint(v.EnableSearch)}
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
		"CREATE TABLE IF NOT EXISTS replies (id TEXT PRIMARY KEY, post_id TEXT NOT NULL, author TEXT NOT NULL, body TEXT NOT NULL, created_at TEXT NOT NULL, deleted_at TEXT)",
		"CREATE INDEX IF NOT EXISTS replies_by_post ON replies(post_id,created_at)",
	} {
		if _,err=db.Exec(statement); err!=nil {db.Close();return nil,err}
	}
	if err := ensureColumn(db, "replies", "deleted_at"); err != nil { db.Close(); return nil, err }
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
	query:="SELECT id,topic_id,title,author,created_at,reply_count FROM post_index WHERE deleted_at IS NULL"
	args:=[]any{};if topicID!="" {query+=" AND topic_id=?";args=append(args,topicID)}
	query+=" ORDER BY created_at DESC LIMIT ? OFFSET ?";args=append(args,limit,offset)
	rows,err:=s.catalog.Query(query,args...);if err!=nil{return nil,err};defer rows.Close()
	out:=[]Post{};for rows.Next(){var v Post;if err:=rows.Scan(&v.ID,&v.TopicID,&v.Title,&v.Author,&v.CreatedAt,&v.ReplyCount);err!=nil{return nil,err};out=append(out,v)}
	return out,rows.Err()
}

func (s *Store) SearchPosts(query,topicID string,limit,offset int) ([]Post,error) {
	query = strings.TrimSpace(query)
	if query == "" { return s.Posts(topicID, limit, offset) }
	if len(query) > 200 { return nil, fmt.Errorf("搜索词不能超过 200 字节") }
	if limit < 1 || limit > 50 { limit = 20 }
	if offset < 0 || offset > 100000 { offset = 0 }
	rows, err := s.catalog.Query("SELECT DISTINCT shard FROM post_index WHERE deleted_at IS NULL ORDER BY shard DESC")
	if err != nil { return nil, err }
	shards := []string{}
	for rows.Next() { var key string; if err := rows.Scan(&key); err != nil { rows.Close(); return nil, err }; shards = append(shards, key) }
	if err := rows.Err(); err != nil { rows.Close(); return nil, err }
	if err := rows.Close(); err != nil { return nil, err }
	pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(query) + "%"
	found := []Post{}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range shards {
		err := s.withShardTx(key, func(tx *sql.Tx) error {
			statement := "SELECT p.id,p.topic_id,p.title,p.author,p.created_at,p.reply_count,b.body FROM post_index p JOIN forum_shard.posts b ON b.id=p.id WHERE p.shard=? AND p.deleted_at IS NULL AND (p.title LIKE ? ESCAPE '\\' OR p.author LIKE ? ESCAPE '\\' OR b.body LIKE ? ESCAPE '\\')"
			args := []any{key,pattern,pattern,pattern}
			if topicID != "" { statement += " AND p.topic_id=?"; args = append(args,topicID) }
			matches, err := tx.Query(statement,args...)
			if err != nil { return err }
			defer matches.Close()
			for matches.Next() {
				var post Post
				if err := matches.Scan(&post.ID,&post.TopicID,&post.Title,&post.Author,&post.CreatedAt,&post.ReplyCount,&post.Body); err != nil { return err }
				post.Attachments = []filemanager.File{}
				post.Replies = []Reply{}
				found = append(found, post)
			}
			return matches.Err()
		})
		if err != nil { return nil, err }
	}
	sort.Slice(found, func(i,j int) bool { return found[i].CreatedAt > found[j].CreatedAt })
	if offset >= len(found) { return []Post{}, nil }
	end := offset + limit
	if end > len(found) { end = len(found) }
	return found[offset:end], nil
}

func (s *Store) Post(id string) (Post,error) {
	var v Post;var key string
	err:=s.catalog.QueryRow("SELECT id,topic_id,title,author,created_at,reply_count,shard FROM post_index WHERE id=? AND deleted_at IS NULL",id).Scan(&v.ID,&v.TopicID,&v.Title,&v.Author,&v.CreatedAt,&v.ReplyCount,&key)
	if errors.Is(err,sql.ErrNoRows){return v,ErrNotFound};if err!=nil{return v,err}
	shard,err:=s.openShard(key);if err!=nil{return v,err};defer shard.Close()
	if err=shard.QueryRow("SELECT body FROM posts WHERE id=?",id).Scan(&v.Body);err!=nil{return v,err}
	v.Attachments,err=s.attachments("post",id);if err!=nil{return v,err}
	rows,err:=shard.Query("SELECT id,post_id,author,body,created_at FROM replies WHERE post_id=? AND deleted_at IS NULL ORDER BY created_at ASC LIMIT 500",id);if err!=nil{return v,err}
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
	if err:=s.catalog.QueryRow("SELECT shard FROM post_index WHERE id=? AND deleted_at IS NULL",postID).Scan(&key);errors.Is(err,sql.ErrNoRows){return Reply{},ErrNotFound}else if err!=nil{return Reply{},err}
	id,err:=newID();if err!=nil{return Reply{},err}
	v:=Reply{ID:id,PostID:postID,Body:body,Author:author,CreatedAt:time.Now().UTC().Format(time.RFC3339Nano),Attachments:[]filemanager.File{}}
	s.mu.Lock();defer s.mu.Unlock()
	shard,err:=s.openShard(key);if err!=nil{return Reply{},err};defer shard.Close()
	if _,err=shard.Exec("INSERT INTO replies(id,post_id,author,body,created_at) VALUES(?,?,?,?,?)",id,postID,author,body,v.CreatedAt);err!=nil{return Reply{},err}
	tx,err:=s.catalog.Begin();if err!=nil{shard.Exec("DELETE FROM replies WHERE id=?",id);return Reply{},err};defer tx.Rollback()
	if _,err=tx.Exec("INSERT INTO reply_index(id,post_id,shard) VALUES(?,?,?)",id,postID,key);err!=nil{shard.Exec("DELETE FROM replies WHERE id=?",id);return Reply{},err}
	result,err:=tx.Exec("UPDATE post_index SET reply_count=reply_count+1 WHERE id=? AND deleted_at IS NULL",postID);if err!=nil{shard.Exec("DELETE FROM replies WHERE id=?",id);return Reply{},err};if count,_:=result.RowsAffected();count==0{shard.Exec("DELETE FROM replies WHERE id=?",id);return Reply{},ErrNotFound}
	if err=tx.Commit();err!=nil{shard.Exec("DELETE FROM replies WHERE id=?",id);return Reply{},err}
	return v,nil
}

func (s *Store) UpdatePost(id, title, body string) error {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if len(title) < 1 || len(title) > 200 || len(body) < 1 || len(body) > 100000 {
		return fmt.Errorf("标题或正文长度无效")
	}
	var shard string
	if err := s.catalog.QueryRow("SELECT shard FROM post_index WHERE id=? AND deleted_at IS NULL", id).Scan(&shard); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil { return err }
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.withShardTx(shard, func(tx *sql.Tx) error {
		result, err := tx.Exec("UPDATE forum_shard.posts SET body=? WHERE id=?", body, id)
		if err != nil { return err }
		if count, _ := result.RowsAffected(); count == 0 { return ErrNotFound }
		result, err = tx.Exec("UPDATE post_index SET title=? WHERE id=? AND deleted_at IS NULL", title, id)
		if err != nil { return err }
		if count, _ := result.RowsAffected(); count == 0 { return ErrNotFound }
		return nil
	})
}

func (s *Store) DeletePost(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.catalog.Exec("UPDATE post_index SET deleted_at=? WHERE id=? AND deleted_at IS NULL", time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil { return err }
	if count, _ := result.RowsAffected(); count == 0 { return ErrNotFound }
	return nil
}

func (s *Store) RestorePost(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.catalog.Exec("UPDATE post_index SET deleted_at=NULL WHERE id=? AND deleted_at IS NOT NULL", id)
	if err != nil { return err }
	if count, _ := result.RowsAffected(); count == 0 { return ErrNotFound }
	return nil
}

func (s *Store) PurgePost(id string) error {
	var shard string
	if err := s.catalog.QueryRow("SELECT shard FROM post_index WHERE id=? AND deleted_at IS NOT NULL", id).Scan(&shard); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil { return err }
	s.mu.Lock()
	defer s.mu.Unlock()
	fileIDs := []string{}
	err := s.withShardTx(shard, func(tx *sql.Tx) error {
		rows, err := tx.Query("SELECT file_id FROM file_links WHERE (entity_type='post' AND entity_id=?) OR (entity_type='reply' AND entity_id IN (SELECT id FROM reply_index WHERE post_id=?))", id, id)
		if err != nil { return err }
		for rows.Next() { var fileID string; if err := rows.Scan(&fileID); err != nil { rows.Close(); return err }; fileIDs = append(fileIDs, fileID) }
		if err := rows.Err(); err != nil { rows.Close(); return err }; rows.Close()
		if _, err := tx.Exec("DELETE FROM forum_shard.replies WHERE post_id=?", id); err != nil { return err }
		if _, err := tx.Exec("DELETE FROM forum_shard.posts WHERE id=?", id); err != nil { return err }
		if _, err := tx.Exec("DELETE FROM file_links WHERE (entity_type='post' AND entity_id=?) OR (entity_type='reply' AND entity_id IN (SELECT id FROM reply_index WHERE post_id=?))", id, id); err != nil { return err }
		if _, err := tx.Exec("DELETE FROM reply_index WHERE post_id=?", id); err != nil { return err }
		result, err := tx.Exec("DELETE FROM post_index WHERE id=? AND deleted_at IS NOT NULL", id)
		if err != nil { return err }
		if count, _ := result.RowsAffected(); count == 0 { return ErrNotFound }
		return nil
	})
	if err != nil { return err }
	return s.clearFileAssociations(fileIDs)
}

func (s *Store) UpdateReply(id, body string) error {
	body = strings.TrimSpace(body)
	if len(body) < 1 || len(body) > 20000 { return fmt.Errorf("回复长度无效") }
	var shard string
	if err := s.catalog.QueryRow("SELECT shard FROM reply_index WHERE id=?", id).Scan(&shard); errors.Is(err, sql.ErrNoRows) { return ErrNotFound } else if err != nil { return err }
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.withShardTx(shard, func(tx *sql.Tx) error {
		result, err := tx.Exec("UPDATE forum_shard.replies SET body=? WHERE id=? AND deleted_at IS NULL", body, id)
		if err != nil { return err }
		if count, _ := result.RowsAffected(); count == 0 { return ErrNotFound }
		return nil
	})
}

func (s *Store) DeleteReply(id string) error {
	var shard, postID string
	if err := s.catalog.QueryRow("SELECT r.shard,r.post_id FROM reply_index r JOIN post_index p ON p.id=r.post_id WHERE r.id=? AND p.deleted_at IS NULL", id).Scan(&shard, &postID); errors.Is(err, sql.ErrNoRows) { return ErrNotFound } else if err != nil { return err }
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.withShardTx(shard, func(tx *sql.Tx) error {
		result, err := tx.Exec("UPDATE forum_shard.replies SET deleted_at=? WHERE id=? AND deleted_at IS NULL", time.Now().UTC().Format(time.RFC3339Nano), id)
		if err != nil { return err }
		if count, _ := result.RowsAffected(); count == 0 { return ErrNotFound }
		result, err = tx.Exec("UPDATE post_index SET reply_count=MAX(reply_count-1,0) WHERE id=? AND deleted_at IS NULL", postID)
		if err != nil { return err }
		if count, _ := result.RowsAffected(); count == 0 { return ErrNotFound }
		return nil
	})
	return err
}

func (s *Store) RestoreReply(id string) error {
	var shard, postID string
	if err := s.catalog.QueryRow("SELECT r.shard,r.post_id FROM reply_index r JOIN post_index p ON p.id=r.post_id WHERE r.id=? AND p.deleted_at IS NULL", id).Scan(&shard,&postID); errors.Is(err, sql.ErrNoRows) { return ErrNotFound } else if err != nil { return err }
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.withShardTx(shard, func(tx *sql.Tx) error {
		result, err := tx.Exec("UPDATE forum_shard.replies SET deleted_at=NULL WHERE id=? AND deleted_at IS NOT NULL", id)
		if err != nil { return err }
		if count, _ := result.RowsAffected(); count == 0 { return ErrNotFound }
		result, err = tx.Exec("UPDATE post_index SET reply_count=reply_count+1 WHERE id=? AND deleted_at IS NULL", postID)
		if err != nil { return err }
		if count, _ := result.RowsAffected(); count == 0 { return ErrNotFound }
		return nil
	})
}

func (s *Store) PurgeReply(id string) error {
	var shard, postID string
	if err := s.catalog.QueryRow("SELECT shard,post_id FROM reply_index WHERE id=?", id).Scan(&shard,&postID); errors.Is(err, sql.ErrNoRows) { return ErrNotFound } else if err != nil { return err }
	s.mu.Lock()
	defer s.mu.Unlock()
	fileIDs := []string{}
	err := s.withShardTx(shard, func(tx *sql.Tx) error {
		var deletedAt sql.NullString
		if err := tx.QueryRow("SELECT deleted_at FROM forum_shard.replies WHERE id=?", id).Scan(&deletedAt); errors.Is(err,sql.ErrNoRows) { return ErrNotFound } else if err != nil { return err }
		if !deletedAt.Valid { return fmt.Errorf("回复必须先移入回收站") }
		rows, err := tx.Query("SELECT file_id FROM file_links WHERE entity_type='reply' AND entity_id=?", id)
		if err != nil { return err }
		for rows.Next() { var fileID string; if err := rows.Scan(&fileID); err != nil { rows.Close(); return err }; fileIDs = append(fileIDs,fileID) }
		if err := rows.Err(); err != nil { rows.Close(); return err }; rows.Close()
		if _, err := tx.Exec("DELETE FROM forum_shard.replies WHERE id=?", id); err != nil { return err }
		if _, err := tx.Exec("DELETE FROM file_links WHERE entity_type='reply' AND entity_id=?", id); err != nil { return err }
		_, err = tx.Exec("DELETE FROM reply_index WHERE id=?", id)
		return err
	})
	if err != nil { return err }
	return s.clearFileAssociations(fileIDs)
}

func (s *Store) TrashPosts() ([]TrashPost, error) {
	type storedPost struct { item TrashPost; shard string }
	rows, err := s.catalog.Query("SELECT p.id,p.topic_id,t.name,p.title,p.author,p.created_at,p.deleted_at,p.reply_count,p.shard FROM post_index p JOIN topics t ON t.id=p.topic_id WHERE p.deleted_at IS NOT NULL ORDER BY p.deleted_at DESC")
	if err != nil { return nil, err }
	stored := []storedPost{}
	for rows.Next() {
		var v storedPost
		if err := rows.Scan(&v.item.ID,&v.item.TopicID,&v.item.TopicName,&v.item.Title,&v.item.Author,&v.item.CreatedAt,&v.item.DeletedAt,&v.item.ReplyCount,&v.shard); err != nil { rows.Close(); return nil, err }
		stored = append(stored,v)
	}
	if err := rows.Err(); err != nil { rows.Close(); return nil, err }
	if err := rows.Close(); err != nil { return nil, err }
	out := make([]TrashPost,0,len(stored))
	for _, v := range stored {
		shard, err := s.openShard(v.shard)
		if err != nil { return nil, err }
		err = shard.QueryRow("SELECT body FROM posts WHERE id=?",v.item.ID).Scan(&v.item.Body)
		closeErr := shard.Close()
		if err != nil { return nil, err }
		if closeErr != nil { return nil, closeErr }
		out = append(out,v.item)
	}
	return out,nil
}

func (s *Store) TrashReplies() ([]TrashReply, error) {
	rows, err := s.catalog.Query("SELECT DISTINCT p.shard FROM reply_index r JOIN post_index p ON p.id=r.post_id WHERE p.deleted_at IS NULL ORDER BY p.shard DESC")
	if err != nil { return nil, err }
	shards := []string{}
	for rows.Next() { var key string; if err := rows.Scan(&key); err != nil { rows.Close(); return nil, err }; shards = append(shards,key) }
	if err := rows.Err(); err != nil { rows.Close(); return nil, err }
	if err := rows.Close(); err != nil { return nil, err }
	out := []TrashReply{}
	for _, key := range shards {
		shard, err := s.openShard(key)
		if err != nil { return nil, err }
		deleted, err := shard.Query("SELECT id,post_id,author,body,created_at,deleted_at FROM replies WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC")
		if err != nil { shard.Close(); return nil, err }
		for deleted.Next() {
			var v TrashReply
			if err := deleted.Scan(&v.ID,&v.PostID,&v.Author,&v.Body,&v.CreatedAt,&v.DeletedAt); err != nil { deleted.Close(); shard.Close(); return nil, err }
			if err := s.catalog.QueryRow("SELECT title FROM post_index WHERE id=? AND deleted_at IS NULL",v.PostID).Scan(&v.PostTitle); errors.Is(err,sql.ErrNoRows) { continue } else if err != nil { deleted.Close(); shard.Close(); return nil, err }
			out = append(out,v)
		}
		if err := deleted.Err(); err != nil { deleted.Close(); shard.Close(); return nil, err }
		if err := deleted.Close(); err != nil { shard.Close(); return nil, err }
		if err := shard.Close(); err != nil { return nil, err }
	}
	sort.Slice(out,func(i,j int) bool { return out[i].DeletedAt > out[j].DeletedAt })
	return out,nil
}

func (s *Store) EmptyTrash() error {
	replies, err := s.TrashReplies()
	if err != nil { return err }
	for _, reply := range replies { if err := s.PurgeReply(reply.ID); err != nil && !errors.Is(err,ErrNotFound) { return err } }
	posts, err := s.TrashPosts()
	if err != nil { return err }
	for _, post := range posts { if err := s.PurgePost(post.ID); err != nil && !errors.Is(err,ErrNotFound) { return err } }
	return nil
}

func (s *Store) withShardTx(key string, fn func(*sql.Tx) error) error {
	shard, err := s.openShard(key)
	if err != nil { return err }
	if err := shard.Close(); err != nil { return err }
	conn, err := s.catalog.Conn(context.Background())
	if err != nil { return err }
	defer conn.Close()
	path := filepath.Join(s.dir, "posts-"+key+".sqlite")
	if _, err := conn.ExecContext(context.Background(), "ATTACH DATABASE ? AS forum_shard", path); err != nil { return err }
	defer func() { _, _ = conn.ExecContext(context.Background(), "DETACH DATABASE forum_shard") }()
	tx, err := conn.BeginTx(context.Background(), nil)
	if err != nil { return err }
	defer tx.Rollback()
	if err := fn(tx); err != nil { return err }
	return tx.Commit()
}

func (s *Store) clearFileAssociations(ids []string) error {
	for _, id := range ids {
		if _, err := s.files.SetAssociation(id, filemanager.Association{}); err != nil && !errors.Is(err, filemanager.ErrNotFound) {
			return fmt.Errorf("content removed but file %s could not be unlinked: %w", id, err)
		}
	}
	return nil
}

func (s *Store) attachments(kind,id string) ([]filemanager.File,error) {
	rows,err:=s.catalog.Query("SELECT file_id FROM file_links WHERE entity_type=? AND entity_id=? ORDER BY rowid",kind,id);if err!=nil{return nil,err}
	ids:=[]string{};for rows.Next(){var fileID string;if err=rows.Scan(&fileID);err!=nil{rows.Close();return nil,err};ids=append(ids,fileID)}
	err=rows.Err();rows.Close();if err!=nil{return nil,err}
	out:=[]filemanager.File{};for _,fileID:=range ids {file,err:=s.files.Get(fileID);if errors.Is(err,filemanager.ErrNotFound){continue};if err!=nil{return nil,err};out=append(out,file)}
	return out,nil
}

// Attach records the relation in SQLite and mirrors it into the file metadata for
// compatibility with older binaries.
func (s *Store) Attach(fileID, kind, id string) error {
	if kind != "post" && kind != "reply" {
		return fmt.Errorf("only posts and replies accept attachments")
	}
	metadata, err := s.files.Get(fileID)
	if err != nil {
		return err
	}
	if kind == "post" {
		var found int
		if err := s.catalog.QueryRow("SELECT 1 FROM post_index WHERE id=?", id).Scan(&found); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
	}
	if kind == "reply" {
		var found int
		if err := s.catalog.QueryRow("SELECT 1 FROM reply_index WHERE id=?", id).Scan(&found); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var oldKind, oldID string
	err = s.catalog.QueryRow("SELECT entity_type,entity_id FROM file_links WHERE file_id=?", fileID).Scan(&oldKind, &oldID)
	if err == nil && (oldKind != kind || oldID != id) {
		return ErrConflict
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	association := filemanager.Association{Type: kind, ID: id}
	if _, err := s.files.SetAssociation(fileID, association); err != nil {
		return err
	}
	if _, err := s.catalog.Exec("INSERT OR IGNORE INTO file_links(file_id,entity_type,entity_id) VALUES(?,?,?)", fileID, kind, id); err != nil {
		if _, restoreErr := s.files.SetAssociation(fileID, metadata.Association); restoreErr != nil {
			return fmt.Errorf("record attachment: %w (restore file metadata: %v)", err, restoreErr)
		}
		return err
	}
	return nil
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
