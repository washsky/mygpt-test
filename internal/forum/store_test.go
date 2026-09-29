package forum

import (
	"errors"
	"net/http/httptest"
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
	defaults, err := store.Settings()
	if err != nil { t.Fatal(err) }
	if defaults.EnableSearch || defaults.EnableResourceRendering { t.Fatalf("optional features should default off: %+v",defaults) }
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
	trashedReplies, err := store.TrashReplies()
	if err != nil || len(trashedReplies) != 1 || trashedReplies[0].ID != reply.ID { t.Fatalf("reply recycle bin: %+v, %v",trashedReplies,err) }
	metadata, err := files.Get(file.ID)
	if err != nil { t.Fatal(err) }
	if metadata.Association.ID != reply.ID { t.Fatalf("trashed reply lost attachment association: %+v", metadata.Association) }
	if err := store.RestoreReply(reply.ID); err != nil { t.Fatal(err) }
	updated, err = store.Post(post.ID)
	if err != nil || len(updated.Replies) != 1 || updated.Replies[0].ID != reply.ID { t.Fatalf("restored reply: %+v, %v",updated,err) }
	if err := store.DeleteReply(reply.ID); err != nil { t.Fatal(err) }
	if err := store.PurgeReply(reply.ID); err != nil { t.Fatal(err) }
	metadata, err = files.Get(file.ID)
	if err != nil { t.Fatal(err) }
	if metadata.Association.ID != "" { t.Fatalf("purged reply retained attachment association: %+v", metadata.Association) }
	if err := store.Attach(file.ID, "post", post.ID); err != nil { t.Fatal(err) }
	if err := store.DeletePost(post.ID); err != nil { t.Fatal(err) }
	if _, err := store.Post(post.ID); err != ErrNotFound { t.Fatalf("deleted post lookup error = %v, want %v", err, ErrNotFound) }
	trashedPosts, err := store.TrashPosts()
	if err != nil || len(trashedPosts) != 1 || trashedPosts[0].ID != post.ID { t.Fatalf("post recycle bin: %+v, %v",trashedPosts,err) }
	if err := store.RestorePost(post.ID); err != nil { t.Fatal(err) }
	if _, err := store.Post(post.ID); err != nil { t.Fatalf("restored post lookup error = %v",err) }
	if err := store.DeletePost(post.ID); err != nil { t.Fatal(err) }
	if err := store.PurgePost(post.ID); err != nil { t.Fatal(err) }
	metadata, err = files.Get(file.ID)
	if err != nil { t.Fatal(err) }
	if metadata.Association.ID != "" { t.Fatalf("purged post retained attachment association: %+v", metadata.Association) }
	if _, opened, err := files.Open(file.ID); err != nil { t.Fatal(err) } else { opened.Close() }
}

func TestInteractionSettingsPersist(t *testing.T){
 root:=t.TempDir();files,err:=filemanager.NewStore(root+"/files");if err!=nil{t.Fatal(err)}
 store,err:=Open(root,files);if err!=nil{t.Fatal(err)}
 settings,err:=store.Settings();if err!=nil{t.Fatal(err)}
 if settings.EnableQuoteReplies||settings.EnableLineNumbers||settings.EnableLineCopy||settings.EnableClipboard||settings.EnableRealtime||settings.EnableNetworkResilience{t.Fatal("new features must default off")}
 settings.EnableQuoteReplies=true;settings.EnableLineNumbers=true;settings.EnableLineCopy=true;settings.EnableClipboard=true;settings.EnableRealtime=true;settings.EnableNetworkResilience=true
 if err:=store.SaveSettings(settings);err!=nil{t.Fatal(err)};store.Close()
 store,err=Open(root,files);if err!=nil{t.Fatal(err)};defer store.Close()
 got,err:=store.Settings();if err!=nil{t.Fatal(err)};if got!=settings{t.Fatalf("settings did not persist: %+v",got)}
 got.EnableRealtime=false;got.EnableClipboard=false;if err:=store.SaveSettings(got);err!=nil{t.Fatal(err)}
 disabled,err:=store.Settings();if err!=nil{t.Fatal(err)};if disabled.EnableRealtime||disabled.EnableClipboard||!disabled.EnableLineCopy{t.Fatal("feature switches must be independent")}
}

func TestClientInfoMigrationAndPersistence(t *testing.T) {
 root:=t.TempDir();files,err:=filemanager.NewStore(root+"/files");if err!=nil{t.Fatal(err)}
 store,err:=Open(root,files);if err!=nil{t.Fatal(err)}
 topics,err:=store.Topics();if err!=nil{t.Fatal(err)}
 legacy,err:=store.CreatePost(topics[0].ID,"旧记录","正文","访客");if err!=nil{t.Fatal(err)}
 shard,err:=store.openShard(legacy.CreatedAt[:7]);if err!=nil{t.Fatal(err)}
 if _,err:=shard.Exec("UPDATE posts SET client_info=NULL WHERE id=?",legacy.ID);err!=nil{t.Fatal(err)}
 if err:=shard.Close();err!=nil{t.Fatal(err)}
 client:=ClientInfo{Browser:"Firefox 130",OS:"Linux",Device:"电脑",Language:"zh-CN",Timezone:"Asia/Shanghai"}
 post,err:=store.CreatePostWithClient(topics[0].ID,"新记录","正文","访客",client);if err!=nil{t.Fatal(err)}
 reply,err:=store.CreateReplyWithClient(post.ID,"回复","访客",client);if err!=nil{t.Fatal(err)}
 if err:=store.Close();err!=nil{t.Fatal(err)}
 store,err=Open(root,files);if err!=nil{t.Fatal(err)};defer store.Close()
 old,err:=store.Post(legacy.ID);if err!=nil{t.Fatal(err)};if old.ClientInfo.Browser!=""{t.Fatal("legacy record unexpectedly gained browser information")}
 loaded,err:=store.Post(post.ID);if err!=nil{t.Fatal(err)}
 if loaded.ClientInfo!=client||len(loaded.Replies)!=1||loaded.Replies[0].ID!=reply.ID||loaded.Replies[0].ClientInfo!=client{t.Fatalf("client information lost: %+v",loaded)}
}

func TestDeleteTopicPreservesPostsAndTrash(t *testing.T) {
 root:=t.TempDir();files,err:=filemanager.NewStore(root+"/files");if err!=nil{t.Fatal(err)}
 store,err:=Open(root,files);if err!=nil{t.Fatal(err)};defer store.Close()
 topic,err:=store.CreateTopic("可删除","空主题");if err!=nil{t.Fatal(err)}
 if err:=store.DeleteTopic(topic.ID,"");err!=nil{t.Fatal(err)}
 if err:=store.DeleteTopic(topic.ID,"");!errors.Is(err,ErrNotFound){t.Fatalf("missing topic: %v",err)}
 topic,err=store.CreateTopic("有帖子","不可删除");if err!=nil{t.Fatal(err)}
 post,err:=store.CreatePost(topic.ID,"记录","正文","我");if err!=nil{t.Fatal(err)}
 if err:=store.DeleteTopic(topic.ID,"");!errors.Is(err,ErrConflict){t.Fatalf("active post should block deletion: %v",err)}
 if err:=store.DeletePost(post.ID);err!=nil{t.Fatal(err)}
 if err:=store.DeleteTopic(topic.ID,"");!errors.Is(err,ErrConflict){t.Fatalf("trashed post should block deletion: %v",err)}
 target,err:=store.CreateTopic("接收","保留内容");if err!=nil{t.Fatal(err)}
 if err:=store.DeleteTopic(topic.ID,target.ID);err!=nil{t.Fatal(err)}
 if got,err:=store.Posts(target.ID,20,0);err!=nil||len(got)!=0{t.Fatalf("trashed post should remain hidden: %+v %v",got,err)}
 if err:=store.RestorePost(post.ID);err!=nil{t.Fatal(err)}
 got,err:=store.Post(post.ID);if err!=nil||got.TopicID!=target.ID{t.Fatalf("moved post lost: %+v %v",got,err)}
}

func TestSelectedClientInfo(t *testing.T) {
 settings:=Settings{EnableClientInfo:true,ClientBrowser:true,ClientTimezone:true}
 request:=httptest.NewRequest("POST","/",nil)
 request.Header.Set("User-Agent","Mozilla/5.0 (Windows NT 10.0) Chrome/130.0.0.0")
 request.Header.Set("Accept-Language","zh-CN,zh;q=0.9")
 info:=selectedClientInfo(settings,request,"Asia/Shanghai")
 if info.Browser!="Chrome 130"||info.Timezone!="Asia/Shanghai"||info.OS!=""||info.Device!=""||info.Language!=""{t.Fatalf("selected fields: %+v",info)}
 settings.EnableClientInfo=false
 if got:=selectedClientInfo(settings,request,"Asia/Shanghai");got!=(ClientInfo{}){t.Fatalf("disabled collection: %+v",got)}
}
