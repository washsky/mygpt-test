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
	if err := store.Close(); err != nil { t.Fatal(err) }
	store, err = Open(root, files)
	if err != nil { t.Fatal(err) }
	defer store.Close()
	loaded, err := store.Post(post.ID)
	if err != nil { t.Fatal(err) }
	if loaded.Body != "正文" || loaded.ReplyCount != 1 || len(loaded.Replies) != 1 || len(loaded.Replies[0].Attachments) != 1 || loaded.Replies[0].Attachments[0].ID != file.ID {
		t.Fatalf("persisted post lost data: %+v", loaded)
	}
}
