package forum

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/washsky/mygpt-test/internal/filemanager"
	"golang.org/x/net/html"
)

const markdownSample = "## 标题\n\n**粗体** *斜体* ~~删除线~~\n\n- 一项\n- [x] 完成\n\n1. 有序\n\n> 引用\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n~~~go\nfmt.Println(\"<safe>\")\n~~~\n\n[链接](https://example.com) ![图片](/api/files/example/preview)\n\n脚注[^note]\n\n[^note]: 脚注内容\n"

func TestMarkdownRenderingAndSanitization(t *testing.T) {
	rendered, err := renderMarkdown(markdownSample, "scope")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{">标题</h2>", "<strong>粗体</strong>", "<em>斜体</em>", "<del>删除线</del>", "<table>", "<blockquote>", "<ol>", "language-go", "&lt;safe&gt;", "脚注内容", "scope-", "type=\"checkbox\"", "disabled"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("missing %q in %s", want, rendered)
		}
	}
	other, err := renderMarkdown(markdownSample, "another")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(other, "scope-") || !strings.Contains(other, "another-") {
		t.Fatal("footnote IDs must be scoped to each content item")
	}
	parsed, err := html.Parse(strings.NewReader(rendered))
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	fragments := []string{}
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		for _, attr := range n.Attr {
			if attr.Key == "id" {
				ids[attr.Val] = true
			}
			if attr.Key == "href" && strings.HasPrefix(attr.Val, "#") {
				fragments = append(fragments, attr.Val[1:])
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(parsed)
	for _, fragment := range fragments {
		if !ids[fragment] {
			t.Fatalf("sanitization removed a footnote destination: %s", fragment)
		}
	}
	for _, attack := range []string{
		"<script>alert(1)</script>\n\n正文",
		"<img src=x onerror=alert(1)>",
		"[x](javascript:alert(1)) ![x](data:image/svg+xml;base64,PHN2Zz4=)",
		"[x](jav&#x61;script:alert(1))",
		"<iframe src=https://example.com></iframe><svg onload=alert(1)></svg>",
		"<style>body{display:none}</style><form><input autofocus onfocus=alert(1)></form>",
	} {
		out, err := renderMarkdown(attack, "attack")
		if err != nil {
			t.Fatal(err)
		}
		doc, err := html.Parse(strings.NewReader(out))
		if err != nil {
			t.Fatal(err)
		}
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode {
				switch n.Data {
				case "script", "style", "iframe", "svg", "form", "object", "embed":
					t.Fatalf("unsafe element in %s", out)
				}
				for _, attr := range n.Attr {
					if strings.HasPrefix(attr.Key, "on") || attr.Key == "style" {
						t.Fatalf("unsafe attribute in %s", out)
					}
					if attr.Key == "href" || attr.Key == "src" {
						value := strings.ToLower(strings.TrimSpace(attr.Val))
						if strings.HasPrefix(value, "javascript:") || strings.HasPrefix(value, "data:") {
							t.Fatalf("unsafe URL in %s", out)
						}
					}
				}
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				walk(child)
			}
		}
		walk(doc)
	}
}

func TestMarkdownMigrationPersistenceAndFormatEdits(t *testing.T) {
	root := t.TempDir()
	files, err := filemanager.NewStore(root + "/files")
	if err != nil {
		t.Fatal(err)
	}
	store, err := Open(root, files)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if store != nil {
			store.Close()
		}
	}()
	topics, _ := store.Topics()
	legacy, err := store.CreatePost(topics[0].ID, "旧标题", "## 旧纯文本", "访客")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateReply(legacy.ID, "**旧回复**", "访客"); err != nil {
		t.Fatal(err)
	}
	// Reproduce the pre-feature schema, including a monthly reply shard.
	if _, err := store.catalog.Exec("ALTER TABLE post_index DROP COLUMN markdown"); err != nil {
		t.Fatal(err)
	}
	shard, err := store.openShard(legacy.CreatedAt[:7])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := shard.Exec("ALTER TABLE replies DROP COLUMN markdown"); err != nil {
		t.Fatal(err)
	}
	shard.Close()
	store.Close()
	store, err = Open(root, files)
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := store.Settings()
	if settings.DefaultPostMarkdown || settings.DefaultReplyMarkdown {
		t.Fatal("Markdown defaults must start off")
	}
	settings.DefaultPostMarkdown, settings.DefaultReplyMarkdown = true, true
	if err := store.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	old, err := store.Post(legacy.ID)
	if err != nil || old.Markdown || old.Replies[0].Markdown {
		t.Fatalf("migration changed old content: %+v %v", old, err)
	}
	post, err := store.CreatePostForUser(topics[0].ID, "Markdown", markdownSample, "访客", "", ClientInfo{}, true)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := store.CreateReplyForUser(post.ID, "**回复**", "访客", "", ClientInfo{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdatePost(post.ID, post.Title, post.Body); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateReply(reply.ID, reply.Body); err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = Open(root, files)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Post(post.ID)
	if err != nil || !got.Markdown || !got.Replies[0].Markdown || got.Body != strings.TrimSpace(markdownSample) {
		t.Fatalf("format or source lost: %+v %v", got, err)
	}
	found, err := store.SearchPosts("标题", "", 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, item := range found {
		if item.ID == post.ID {
			seen = item.Markdown
		}
	}
	if !seen {
		t.Fatal("search results lost the format")
	}
	if err := store.DeleteReply(reply.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.RestoreReply(reply.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeletePost(post.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.RestorePost(post.ID); err != nil {
		t.Fatal(err)
	}
	got, err = store.Post(post.ID)
	if err != nil || !got.Markdown || !got.Replies[0].Markdown {
		t.Fatal("trash restore changed the format")
	}
	if err := store.UpdatePost(post.ID, post.Title, post.Body, false); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateReply(reply.ID, reply.Body, false); err != nil {
		t.Fatal(err)
	}
	got, err = store.Post(post.ID)
	if err != nil || got.Markdown || got.Replies[0].Markdown {
		t.Fatal("explicit plain-text choice was not saved")
	}
}

func TestMarkdownAPIDefaultsAndOverrides(t *testing.T) {
	root := t.TempDir()
	files, _ := filemanager.NewStore(root + "/files")
	store, err := Open(root, files)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	topics, _ := store.Topics()
	mux := http.NewServeMux()
	(&Handler{Store: store}).Register(mux)
	send := func(path string, in map[string]any, out any) {
		t.Helper()
		body, _ := json.Marshal(in)
		req := httptest.NewRequest("POST", path, strings.NewReader(string(body)))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != 201 {
			t.Fatalf("status %d: %s", res.Code, res.Body.String())
		}
		if err := json.Unmarshal(res.Body.Bytes(), out); err != nil {
			t.Fatal(err)
		}
	}
	for _, defaults := range [][2]bool{{false, false}, {false, true}, {true, false}, {true, true}} {
		settings, _ := store.Settings()
		settings.DefaultPostMarkdown, settings.DefaultReplyMarkdown = defaults[0], defaults[1]
		if err := store.SaveSettings(settings); err != nil {
			t.Fatal(err)
		}
		saved, _ := store.Settings()
		if saved != settings {
			t.Fatal("default switches changed other settings")
		}
		postInput := map[string]any{"topic_id": topics[0].ID, "title": "格式选择", "body": "**正文**", "author": "访客"}
		var post Post
		send("/api/forum/posts", postInput, &post)
		if post.Markdown != defaults[0] || strings.Contains(post.BodyHTML, "<strong>") != defaults[0] {
			t.Fatalf("post default: %+v", post)
		}
		var reply Reply
		send("/api/forum/posts/"+post.ID+"/replies", map[string]any{"body": "**回复**", "author": "访客"}, &reply)
		if reply.Markdown != defaults[1] || strings.Contains(reply.BodyHTML, "<strong>") != defaults[1] {
			t.Fatalf("reply default: %+v", reply)
		}
		postInput["markdown"] = !defaults[0]
		send("/api/forum/posts", postInput, &post)
		if post.Markdown != !defaults[0] {
			t.Fatal("explicit post choice ignored")
		}
		send("/api/forum/posts/"+post.ID+"/replies", map[string]any{"body": "**回复**", "author": "访客", "markdown": !defaults[1]}, &reply)
		if reply.Markdown != !defaults[1] {
			t.Fatal("explicit reply choice ignored")
		}
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, httptest.NewRequest("GET", "/api/forum/posts/"+post.ID, nil))
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		var loaded Post
		if err := json.Unmarshal(res.Body.Bytes(), &loaded); err != nil {
			t.Fatal(err)
		}
		if loaded.Markdown != post.Markdown || loaded.Replies[0].Markdown != reply.Markdown {
			t.Fatal("GET lost item format")
		}
	}
}
