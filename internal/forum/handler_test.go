package forum

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/washsky/mygpt-test/internal/filemanager"
)

func TestForumTrashAndSearchAPI(t *testing.T) {
	root := t.TempDir()
	files, err := filemanager.NewStore(root+"/files")
	if err != nil { t.Fatal(err) }
	store, err := Open(root,files)
	if err != nil { t.Fatal(err) }
	defer store.Close()
	topics, err := store.Topics()
	if err != nil { t.Fatal(err) }
	post, err := store.CreatePost(topics[0].ID,"needle title","body has needle","tester")
	if err != nil { t.Fatal(err) }
	mux := http.NewServeMux()
	(&Handler{Store:store,Token:"admin-token"}).Register(mux)
	request := func(method,path,token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method,path,nil)
		if token != "" { req.Header.Set("X-Admin-Token",token) }
		response := httptest.NewRecorder()
		mux.ServeHTTP(response,req)
		return response
	}
	if response := request(http.MethodDelete,"/api/forum/posts/"+post.ID,""); response.Code != http.StatusForbidden { t.Fatalf("unauthenticated delete status = %d",response.Code) }
	if response := request(http.MethodDelete,"/api/forum/posts/"+post.ID,"admin-token"); response.Code != http.StatusNoContent { t.Fatalf("delete status = %d body=%s",response.Code,response.Body.String()) }
	if response := request(http.MethodGet,"/api/forum/posts/"+post.ID,""); response.Code != http.StatusNotFound { t.Fatalf("trashed post lookup status = %d",response.Code) }
	if response := request(http.MethodGet,"/api/forum/trash","admin-token"); response.Code != http.StatusOK || !strings.Contains(response.Body.String(),post.ID) { t.Fatalf("trash response status=%d body=%s",response.Code,response.Body.String()) }
	if response := request(http.MethodPost,"/api/forum/trash/posts/"+post.ID+"/restore","admin-token"); response.Code != http.StatusNoContent { t.Fatalf("restore status = %d body=%s",response.Code,response.Body.String()) }
	if response := request(http.MethodGet,"/api/forum/posts?q=needle",""); response.Code != http.StatusForbidden { t.Fatalf("search while disabled status = %d",response.Code) }
	if err := store.SaveSettings(Settings{Title:"test",EnableSearch:true}); err != nil { t.Fatal(err) }
	if response := request(http.MethodGet,"/api/forum/posts?q=needle",""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(),post.ID) { t.Fatalf("search response status=%d body=%s",response.Code,response.Body.String()) }
}
