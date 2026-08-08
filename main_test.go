package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type fakeXWiki struct {
	mu         sync.Mutex
	auths      []string
	paths      []string
	putBodies  []map[string]any
	postBodies []map[string]any
	deleted    []string
	tagForm    []string
	tags       []string
	uploaded   []fakeUpload
}

type fakeUpload struct {
	name string
	data []byte
}

func (f *fakeXWiki) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.auths = append(f.auths, r.Header.Get("Authorization"))
		f.paths = append(f.paths, r.URL.RequestURI())
		f.mu.Unlock()

		writeJSON := func(v any) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(v)
		}

		if r.URL.Path == "/search" {
			writeJSON(map[string]any{"searchResults": []map[string]any{
				{"id": "xwiki:Main.WebHome", "pageFullName": "Main.WebHome", "pageTitle": "Home", "score": 95.5, "snippet": "hello world"},
			}})
			return
		}

		if r.URL.Path == "/tags" {
			writeJSON(map[string]any{"tags": []map[string]any{
				{"name": "ansible"},
				{"name": "linux"},
			}})
			return
		}

		if strings.HasPrefix(r.URL.Path, "/tags/") {
			if r.Method != http.MethodGet {
				http.Error(w, "unexpected", http.StatusBadRequest)
				return
			}
			writeJSON(map[string]any{"pageSummaries": []map[string]any{
				{"id": "xwiki:Manuals.ansible.WebHome", "fullName": "Manuals.ansible.WebHome", "name": "WebHome", "title": "Ansible", "version": "3.1"},
				{"id": "xwiki:Main.Terms", "fullName": "Main.Terms", "name": "Terms", "title": "Terms", "version": "1.0"},
			}})
			return
		}

		if r.URL.Path == "/modifications" {
			writeJSON(map[string]any{"pageVersions": []map[string]any{
				{"id": "xwiki:Main.WebHome", "fullName": "Main.WebHome", "version": "1.2", "author": "XWiki.bot", "date": "2026-01-01T10:00:00Z", "comment": "edit"},
			}})
			return
		}

		segs := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		i := 0
		var spaceParts []string
		for i+1 < len(segs) && segs[i] == "spaces" {
			spaceParts = append(spaceParts, segs[i+1])
			i += 2
		}
		space := strings.Join(spaceParts, ".")
		rest := segs[i:]

		if len(rest) == 1 && rest[0] == "spaces" && len(spaceParts) == 0 {
			writeJSON(map[string]any{"spaces": []map[string]any{
				{"id": "xwiki:Main", "name": "Main", "home": "/rest/wikis/xwiki/spaces/Main/pages/WebHome", "url": "/rest/wikis/xwiki/spaces/Main"},
			}})
			return
		}

		if len(rest) >= 1 && rest[0] == "pages" {
			pageName := ""
			if len(rest) >= 2 {
				pageName = rest[1]
			}
			suffix := ""
			if len(rest) >= 3 {
				suffix = rest[2]
			}
			fullName := space + "." + pageName
			switch r.Method {
			case http.MethodGet:
				switch {
				case pageName == "":
					writeJSON(map[string]any{"pageSummaries": []map[string]any{
						{"id": "xwiki:" + space + ".WebHome", "fullName": space + ".WebHome", "name": "WebHome", "title": "Home", "version": "1.1", "author": "XWiki.bot"},
					}})
				case len(rest) == 4 && rest[2] == "history":
					writeJSON(map[string]any{
						"id": "xwiki:" + fullName, "fullName": fullName, "space": space, "name": pageName,
						"title": "Home", "version": rest[3], "author": "XWiki.bot",
						"content": "= Old content =", "syntax": "xwiki/2.1",
					})
				case suffix == "history":
					writeJSON(map[string]any{"pageVersions": []map[string]any{
						{"version": "1.2", "author": "XWiki.bot", "date": "2026-01-01", "comment": "edit"},
					}})
				case len(rest) == 4 && rest[2] == "attachments":
					if strings.HasSuffix(rest[3], ".png") {
						w.Header().Set("Content-Type", "image/png")
						w.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
						return
					}
					w.Header().Set("Content-Type", "text/plain")
					w.Write([]byte("hello attachment"))
				case suffix == "attachments":
					writeJSON(map[string]any{"attachments": []map[string]any{
						{"id": fullName + "@img.png", "name": "img.png", "size": 1024, "version": "1.0", "author": "XWiki.bot", "date": "2026-01-01"},
					}})
				case suffix == "tags":
					writeJSON(map[string]any{"tags": []map[string]any{
						{"name": "ansible"},
						{"name": "linux"},
					}})
				case suffix == "children":
					writeJSON(map[string]any{"pageSummaries": []map[string]any{
						{"id": "xwiki:" + space + ".Child1", "fullName": space + ".Child1", "name": "Child1", "title": "Child 1", "version": "1.0"},
					}})
				case suffix == "comments":
					writeJSON(map[string]any{"comments": []map[string]any{
						{"id": 0, "author": "XWiki.bot", "authorName": "Bot", "date": "2026-01-01T10:00:00Z", "text": "Nice page"},
					}})
				default:
					writeJSON(map[string]any{
						"id": "xwiki:" + fullName, "fullName": fullName, "space": space, "name": pageName,
						"title": "Home", "version": "1.2", "author": "XWiki.bot",
						"content": "= Hello =", "syntax": "xwiki/2.1",
					})
				}
			case http.MethodPut:
				if suffix == "tags" {
					if err := r.ParseForm(); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					f.mu.Lock()
					f.tagForm = r.Form["tag"]
					f.mu.Unlock()
					w.WriteHeader(http.StatusAccepted)
					return
				}
				if len(rest) == 4 && rest[2] == "attachments" {
					data, _ := io.ReadAll(r.Body)
					f.mu.Lock()
					f.uploaded = append(f.uploaded, fakeUpload{name: rest[3], data: data})
					f.mu.Unlock()
					w.WriteHeader(http.StatusCreated)
					return
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t := http.StatusBadRequest
					http.Error(w, err.Error(), t)
					return
				}
				f.mu.Lock()
				f.putBodies = append(f.putBodies, body)
				f.mu.Unlock()
				w.Header().Set("Location", "/rest/wikis/xwiki/spaces/"+strings.Join(spaceParts, "/")+"/pages/"+pageName)
				w.WriteHeader(http.StatusCreated)
			case http.MethodPost:
				if suffix == "comments" {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					f.mu.Lock()
					f.postBodies = append(f.postBodies, body)
					f.mu.Unlock()
					w.Header().Set("Location", "/rest/wikis/xwiki/spaces/"+strings.Join(spaceParts, "/")+"/pages/"+pageName+"/comments/0")
					w.WriteHeader(http.StatusCreated)
					return
				}
				http.Error(w, "unexpected POST", http.StatusBadRequest)
			case http.MethodDelete:
				if len(rest) == 4 && rest[2] == "attachments" {
					f.mu.Lock()
					f.deleted = append(f.deleted, fullName+"@"+rest[3])
					f.mu.Unlock()
					w.WriteHeader(http.StatusNoContent)
					return
				}
				f.mu.Lock()
				f.deleted = append(f.deleted, fullName)
				f.mu.Unlock()
				w.WriteHeader(http.StatusNoContent)
			default:
				http.Error(w, "unexpected", http.StatusBadRequest)
			}
			return
		}

		http.Error(w, fmt.Sprintf("unhandled path %s", r.URL.Path), http.StatusNotFound)
	})
}

func (f *fakeXWiki) lastAuth() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.auths) == 0 {
		return ""
	}
	return f.auths[len(f.auths)-1]
}

func (f *fakeXWiki) lastPath() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.paths) == 0 {
		return ""
	}
	return f.paths[len(f.paths)-1]
}

type testEnv struct {
	xwiki  *fakeXWiki
	client *client.Client
	srv    *httptest.Server
}

func setup(t *testing.T, opts ...func(*ToolConfig)) *testEnv {
	t.Helper()
	xwiki := &fakeXWiki{}
	xwikiSrv := httptest.NewServer(xwiki.handler())
	t.Cleanup(xwikiSrv.Close)

	cfg := ToolConfig{BaseURL: xwikiSrv.URL, Timeout: 5 * time.Second}
	for _, o := range opts {
		o(&cfg)
	}

	mcpServer := server.NewMCPServer("xwiki-mcp", "test", server.WithToolCapabilities(true))
	NewTools(cfg).Register(mcpServer)
	srv := httptest.NewServer(server.NewStreamableHTTPServer(mcpServer))
	t.Cleanup(srv.Close)

	c, err := client.NewStreamableHttpClient(srv.URL,
		transport.WithHTTPHeaders(map[string]string{HeaderXWikiToken: "test-token"}))
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	if _, err := c.Initialize(context.Background(), mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "test", Version: "0.0.0"},
		},
	}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return &testEnv{xwiki: xwiki, client: c, srv: srv}
}

func callTool(t *testing.T, c *client.Client, name string, args map[string]any) string {
	t.Helper()
	res, err := c.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: name, Arguments: args},
	})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("tool %s returned error: %s", name, textContent(res))
	}
	return textContent(res)
}

func textContent(res *mcp.CallToolResult) string {
	if len(res.Content) == 0 {
		return ""
	}
	if t, ok := res.Content[0].(mcp.TextContent); ok {
		return t.Text
	}
	return fmt.Sprintf("%v", res.Content[0])
}

func TestListSpacesSendsBearerToken(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "list_spaces", nil)
	if !strings.Contains(out, "Main") {
		t.Fatalf("expected spaces list to contain Main, got: %s", out)
	}
	if got := env.xwiki.lastAuth(); got != "Bearer test-token" {
		t.Fatalf("expected Authorization 'Bearer test-token', got %q", got)
	}
	if got := env.xwiki.lastPath(); got != "/spaces?start=0&number=200" {
		t.Fatalf("unexpected path %q", got)
	}
}

func TestGetPage(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "get_page", map[string]any{"space": "Main", "page": "WebHome"})
	if !strings.Contains(out, "= Hello =") {
		t.Fatalf("expected page content, got: %s", out)
	}
	if got := env.xwiki.lastPath(); got != "/spaces/Main/pages/WebHome" {
		t.Fatalf("unexpected path %q", got)
	}
}

func TestListPagesNestedSpace(t *testing.T) {
	env := setup(t)
	callTool(t, env.client, "list_pages", map[string]any{"space": "Manuals/ansible"})
	if got := env.xwiki.lastPath(); got != "/spaces/Manuals/spaces/ansible/pages?start=0&number=50" {
		t.Fatalf("unexpected path %q", got)
	}
	env2 := setup(t)
	callTool(t, env2.client, "list_pages", map[string]any{"space": "Manuals.ansible"})
	if got := env2.xwiki.lastPath(); got != "/spaces/Manuals/spaces/ansible/pages?start=0&number=50" {
		t.Fatalf("dot notation failed, path %q", got)
	}
}

func TestSavePage(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "save_page", map[string]any{
		"space": "Main", "page": "NewPage",
		"title": "New", "content": "**bold**", "comment": "created",
	})
	if !strings.Contains(out, "saved") {
		t.Fatalf("unexpected result: %s", out)
	}
	env.xwiki.mu.Lock()
	defer env.xwiki.mu.Unlock()
	if len(env.xwiki.putBodies) != 1 {
		t.Fatalf("expected one PUT, got %d", len(env.xwiki.putBodies))
	}
	body := env.xwiki.putBodies[0]
	if body["title"] != "New" || body["content"] != "**bold**" || body["syntax"] != "xwiki/2.1" || body["comment"] != "created" {
		t.Fatalf("unexpected PUT body: %v", body)
	}
}

func TestSearch(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "search_pages", map[string]any{"query": "hello"})
	if !strings.Contains(out, "hello world") {
		t.Fatalf("expected search snippet, got: %s", out)
	}
	if got := env.xwiki.lastPath(); got != "/search?q=hello&start=0&number=20" {
		t.Fatalf("unexpected path %q", got)
	}
}

func TestHistoryAndAttachments(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "get_page_history", map[string]any{"space": "Main", "page": "WebHome"})
	if !strings.Contains(out, "1.2") {
		t.Fatalf("expected version, got: %s", out)
	}
	out = callTool(t, env.client, "list_attachments", map[string]any{"space": "Main", "page": "WebHome"})
	if !strings.Contains(out, "img.png") {
		t.Fatalf("expected attachment, got: %s", out)
	}
}

func TestDeletePageDisabledByDefault(t *testing.T) {
	env := setup(t)
	res, err := env.client.ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "delete_page" {
			t.Fatalf("delete_page should not be registered without --allow-delete")
		}
	}
}

func TestDeletePageEnabled(t *testing.T) {
	env := setup(t, func(c *ToolConfig) { c.AllowDelete = true })
	callTool(t, env.client, "delete_page", map[string]any{"space": "Main", "page": "WebHome"})
	env.xwiki.mu.Lock()
	defer env.xwiki.mu.Unlock()
	if len(env.xwiki.deleted) != 1 || env.xwiki.deleted[0] != "Main.WebHome" {
		t.Fatalf("expected Main.WebHome deleted, got %v", env.xwiki.deleted)
	}
}

func TestTokenFromAuthorizationHeader(t *testing.T) {
	env := setup(t)
	c, err := client.NewStreamableHttpClient(env.srv.URL,
		transport.WithHTTPHeaders(map[string]string{HeaderAuthorization: "Bearer custom-token"}))
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	if _, err := c.Initialize(context.Background(), mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "test", Version: "0.0.0"},
		},
	}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	callTool(t, c, "list_spaces", nil)
	if got := env.xwiki.lastAuth(); got != "Bearer custom-token" {
		t.Fatalf("expected 'Bearer custom-token', got %q", got)
	}
}

func TestDefaultTokenFallback(t *testing.T) {
	xwiki := &fakeXWiki{}
	xwikiSrv := httptest.NewServer(xwiki.handler())
	t.Cleanup(xwikiSrv.Close)

	mcpServer := server.NewMCPServer("xwiki-mcp", "test", server.WithToolCapabilities(true))
	NewTools(ToolConfig{BaseURL: xwikiSrv.URL, DefaultToken: "default-token", Timeout: 5 * time.Second}).Register(mcpServer)
	srv := httptest.NewServer(server.NewStreamableHTTPServer(mcpServer))
	t.Cleanup(srv.Close)

	c, err := client.NewStreamableHttpClient(srv.URL)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	if _, err := c.Initialize(context.Background(), mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "test", Version: "0.0.0"},
		},
	}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	callTool(t, c, "list_spaces", nil)
	if got := xwiki.lastAuth(); got != "Bearer default-token" {
		t.Fatalf("expected fallback 'Bearer default-token', got %q", got)
	}
}

func TestListTags(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "list_tags", nil)
	if !strings.Contains(out, "ansible") || !strings.Contains(out, "linux") {
		t.Fatalf("expected tags, got: %s", out)
	}
	if got := env.xwiki.lastPath(); got != "/tags" {
		t.Fatalf("unexpected path %q", got)
	}
}

func TestGetPagesByTag(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "get_pages_by_tag", map[string]any{"tags": []any{"ansible"}})
	if !strings.Contains(out, "Manuals.ansible.WebHome") {
		t.Fatalf("expected tagged pages, got: %s", out)
	}
	if got := env.xwiki.lastPath(); got != "/tags/ansible?start=0&number=50&prettyNames=false" {
		t.Fatalf("unexpected path %q", got)
	}
}

func TestSetPageTagsReplace(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "set_page_tags", map[string]any{
		"space": "Main", "page": "WebHome", "tags": []any{"ansible", "linux"},
	})
	if !strings.Contains(out, "ansible, linux") {
		t.Fatalf("unexpected result: %s", out)
	}
	env.xwiki.mu.Lock()
	defer env.xwiki.mu.Unlock()
	if len(env.xwiki.tagForm) != 2 || env.xwiki.tagForm[0] != "ansible" || env.xwiki.tagForm[1] != "linux" {
		t.Fatalf("expected form tags [ansible linux], got %v", env.xwiki.tagForm)
	}
}

func TestSetPageTagsAddModeMergesExisting(t *testing.T) {
	env := setup(t)
	callTool(t, env.client, "set_page_tags", map[string]any{
		"space": "Main", "page": "WebHome",
		"tags": []any{"nginx"}, "mode": "add",
	})
	env.xwiki.mu.Lock()
	defer env.xwiki.mu.Unlock()
	want := []string{"nginx", "ansible", "linux"}
	if len(env.xwiki.tagForm) != len(want) {
		t.Fatalf("expected merged tags %v, got %v", want, env.xwiki.tagForm)
	}
	for i := range want {
		if env.xwiki.tagForm[i] != want[i] {
			t.Fatalf("expected merged tags %v, got %v", want, env.xwiki.tagForm)
		}
	}
}

func TestSetPageTagsInvalidMode(t *testing.T) {
	env := setup(t)
	res, err := env.client.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "set_page_tags", Arguments: map[string]any{
			"space": "Main", "page": "WebHome", "tags": []any{"x"}, "mode": "upsert",
		}},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError || !strings.Contains(textContent(res), "invalid mode") {
		t.Fatalf("expected invalid mode error, got: %s", textContent(res))
	}
}

func TestGetPageVersion(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "get_page_version", map[string]any{"space": "Main", "page": "WebHome", "version": "1.1"})
	if !strings.Contains(out, "Old content") || !strings.Contains(out, "\"version\": \"1.1\"") {
		t.Fatalf("expected old version content, got: %s", out)
	}
	if got := env.xwiki.lastPath(); got != "/spaces/Main/pages/WebHome/history/1.1" {
		t.Fatalf("unexpected path %q", got)
	}
}

func TestListPageChildren(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "list_page_children", map[string]any{
		"space": "Main", "page": "WebHome", "hierarchy": "nestedpages", "search": "child",
	})
	if !strings.Contains(out, "Child1") {
		t.Fatalf("expected children, got: %s", out)
	}
	if got := env.xwiki.lastPath(); got != "/spaces/Main/pages/WebHome/children?start=0&number=50&hierarchy=nestedpages&search=child" {
		t.Fatalf("unexpected path %q", got)
	}
}

func TestListCommentsAndAddComment(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "list_comments", map[string]any{"space": "Main", "page": "WebHome"})
	if !strings.Contains(out, "Nice page") {
		t.Fatalf("expected comments, got: %s", out)
	}
	if got := env.xwiki.lastPath(); got != "/spaces/Main/pages/WebHome/comments?start=0&number=-1" {
		t.Fatalf("unexpected path %q", got)
	}
	out = callTool(t, env.client, "add_comment", map[string]any{
		"space": "Main", "page": "WebHome", "text": "My comment", "reply_to": 0,
	})
	if !strings.Contains(out, "added") {
		t.Fatalf("unexpected result: %s", out)
	}
	env.xwiki.mu.Lock()
	defer env.xwiki.mu.Unlock()
	if len(env.xwiki.postBodies) != 1 {
		t.Fatalf("expected one POST, got %d", len(env.xwiki.postBodies))
	}
	body := env.xwiki.postBodies[0]
	if body["text"] != "My comment" || body["replyTo"] != float64(0) {
		t.Fatalf("unexpected comment body: %v", body)
	}
}

func TestGetModifications(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "get_modifications", map[string]any{"number": 10, "order": "asc", "date": 1704067200000})
	if !strings.Contains(out, "Main.WebHome") {
		t.Fatalf("expected modifications, got: %s", out)
	}
	if got := env.xwiki.lastPath(); got != "/modifications?start=0&number=10&order=asc&date=1704067200000" {
		t.Fatalf("unexpected path %q", got)
	}
}

func TestUploadAttachment(t *testing.T) {
	env := setup(t)
	content := base64.StdEncoding.EncodeToString([]byte("hello attachment"))
	out := callTool(t, env.client, "upload_attachment", map[string]any{
		"space": "Main", "page": "WebHome", "filename": "note.txt", "content_base64": content,
	})
	if !strings.Contains(out, "note.txt") || !strings.Contains(out, "16 bytes") {
		t.Fatalf("unexpected result: %s", out)
	}
	env.xwiki.mu.Lock()
	defer env.xwiki.mu.Unlock()
	if len(env.xwiki.uploaded) != 1 || env.xwiki.uploaded[0].name != "note.txt" || string(env.xwiki.uploaded[0].data) != "hello attachment" {
		t.Fatalf("unexpected upload: %+v", env.xwiki.uploaded)
	}
}

func TestUploadAttachmentInvalidBase64(t *testing.T) {
	env := setup(t)
	res, err := env.client.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "upload_attachment", Arguments: map[string]any{
			"space": "Main", "page": "WebHome", "filename": "note.txt", "content_base64": "!!!not-base64!!!",
		}},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError || !strings.Contains(textContent(res), "invalid base64") {
		t.Fatalf("expected base64 error, got: %s", textContent(res))
	}
}

func TestDownloadAttachmentImage(t *testing.T) {
	env := setup(t)
	res, err := env.client.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "download_attachment", Arguments: map[string]any{
			"space": "Main", "page": "WebHome", "filename": "img.png",
		}},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", textContent(res))
	}
	if len(res.Content) != 2 {
		t.Fatalf("expected text + image content, got %d items", len(res.Content))
	}
	img, ok := res.Content[1].(mcp.ImageContent)
	if !ok {
		t.Fatalf("expected ImageContent, got %T", res.Content[1])
	}
	if img.MIMEType != "image/png" {
		t.Fatalf("expected mime image/png, got %q", img.MIMEType)
	}
	data, err := base64.StdEncoding.DecodeString(img.Data)
	if err != nil || len(data) != 8 {
		t.Fatalf("unexpected image data: %v, %v", len(data), err)
	}
	if !strings.Contains(textContent(res), "8 bytes") {
		t.Fatalf("unexpected caption: %s", textContent(res))
	}
	if got := env.xwiki.lastPath(); got != "/spaces/Main/pages/WebHome/attachments/img.png" {
		t.Fatalf("unexpected path %q", got)
	}
}

func TestDownloadAttachmentTextFile(t *testing.T) {
	env := setup(t)
	out := callTool(t, env.client, "download_attachment", map[string]any{
		"space": "Main", "page": "WebHome", "filename": "note.txt",
	})
	if !strings.Contains(out, "aGVsbG8gYXR0YWNobWVudA==") {
		t.Fatalf("expected base64 content, got: %s", out)
	}
	if !strings.Contains(out, "text/plain") {
		t.Fatalf("expected mime in caption, got: %s", out)
	}
}

func TestDeleteAttachmentEnabled(t *testing.T) {
	env := setup(t, func(c *ToolConfig) { c.AllowDelete = true })
	callTool(t, env.client, "delete_attachment", map[string]any{
		"space": "Main", "page": "WebHome", "filename": "img.png",
	})
	env.xwiki.mu.Lock()
	defer env.xwiki.mu.Unlock()
	if len(env.xwiki.deleted) != 1 || env.xwiki.deleted[0] != "Main.WebHome@img.png" {
		t.Fatalf("expected attachment deleted, got %v", env.xwiki.deleted)
	}
}

func TestDeleteAttachmentDisabledByDefault(t *testing.T) {
	env := setup(t)
	res, err := env.client.ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "delete_attachment" {
			t.Fatalf("delete_attachment should not be registered without --allow-delete")
		}
	}
}

func TestNoTokenError(t *testing.T) {
	xwiki := &fakeXWiki{}
	xwikiSrv := httptest.NewServer(xwiki.handler())
	t.Cleanup(xwikiSrv.Close)

	mcpServer := server.NewMCPServer("xwiki-mcp", "test", server.WithToolCapabilities(true))
	NewTools(ToolConfig{BaseURL: xwikiSrv.URL, Timeout: 5 * time.Second}).Register(mcpServer)
	srv := httptest.NewServer(server.NewStreamableHTTPServer(mcpServer))
	t.Cleanup(srv.Close)

	c, err := client.NewStreamableHttpClient(srv.URL)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	if _, err := c.Initialize(context.Background(), mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo:      mcp.Implementation{Name: "test", Version: "0.0.0"},
		},
	}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	res, err := c.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "list_spaces", Arguments: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected tool error without token, got: %s", textContent(res))
	}
	if !strings.Contains(textContent(res), "no XWiki token") {
		t.Fatalf("unexpected error text: %s", textContent(res))
	}
}
