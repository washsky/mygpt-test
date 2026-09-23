package forum

import (
	"strings"
	"testing"

	"github.com/washsky/mygpt-test/internal/filemanager"
)

func TestPersistenceAndAttachment(t *testing.T) {
	root := t.TempDir()
	files, err := filemanager.NewStore(root + "/files")
	if err != nil { t.Fatal(err) }
	store, err := Open(root, files)
	if err != nil { t.Fatal(err) }
	topics, err := store.Topics()
	if err != nil || len(topics) != 1 { t.Fatalf("initial topic: %v, %v", topics, err) }
	post, err := store.CreatePost(topics[0].ID, "测试文章", "正文", "作者")
	if err != nil { t.Fatal(err) }
	reply, err := store.CreateReply(post.ID, "回复内容", "访客")
	if err != nil { t.Fatal(err) }
	file, err := files.Save("note.txt", "text/plain", filemanager.Association{}, strings.NewReader("attachment"))
	if err != nil { t.Fatal(err) }
	if err := store.Attach(file.ID, "reply", reply.ID); err != nil { t.Fatal(err) }
	if err := store.UpdatePost(post.ID, "修改后的标题", "修改后的正文"); err != nil { t.Fatal(err) }
	if err := store.UpdateReply(reply.ID, "修改后的回复"); err != nil { t.Fatal(err) }
	if err := store.Close(); err != nil { t.Fatal(err) }
	store, err = Open(root, files)
	if err != nil { t.Fatal(err) }
	defer store.Close()
	loaded, err := store.Post(post.ID)
	if err != nil { t.Fatal(err) }
	if loaded.Title != "修改后的标题" || loaded.Body != "修改后的正文" || loaded.ReplyCount != 1 || len(loaded.Replies) != 1 || loaded.Replies[0].Body != "修改后的回复" || len(loaded.Replies[0].Attachments) != 1 || loaded.Replies[0].Attachments[0].ID != file.ID {
		t.Fatalf("persisted post lost data: %+v", loaded)
	}
	if err := store.DeleteReply(reply.ID); err != nil { t.Fatal(err) }
	updated, err := store.Post(post.ID)
	if err != nil { t.Fatal(err) }
	if updated.ReplyCount != 0 || len(updated.Replies) != 0 { t.Fatalf("reply was not removed: %+v", updated) }
	metadata, err := files.Get(file.ID)
	if err != nil { t.Fatal(err) }
	if metadata.Association.ID != "" { t.Fatalf("deleted reply retained attachment association: %+v", metadata.Association) }
	if err := store.Attach(file.ID, "post", post.ID); err != nil { t.Fatal(err) }
	if err := store.DeletePost(post.ID); err != nil { t.Fatal(err) }
	if _, err := store.Post(post.ID); err != ErrNotFound { t.Fatalf("deleted post lookup error = %v, want %v", err, ErrNotFound) }
	if _, opened, err := files.Open(file.ID); err != nil { t.Fatal(err) } else { opened.Close() }
}
