package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ToolConfig struct {
	BaseURL      string
	DefaultToken string
	AllowDelete  bool
	Timeout      time.Duration
}

type tools struct {
	cfg ToolConfig
}

func NewTools(cfg ToolConfig) *tools {
	return &tools{cfg: cfg}
}

func (t *tools) Register(mcpServer *server.MCPServer) {
	reg := []server.ServerTool{
		{Tool: toolListSpaces, Handler: t.handleListSpaces},
		{Tool: toolListPages, Handler: t.handleListPages},
		{Tool: toolGetPage, Handler: t.handleGetPage},
		{Tool: toolSavePage, Handler: t.handleSavePage},
		{Tool: toolSearchPages, Handler: t.handleSearchPages},
		{Tool: toolGetPageHistory, Handler: t.handleGetPageHistory},
		{Tool: toolListAttachments, Handler: t.handleListAttachments},
	}
	if t.cfg.AllowDelete {
		reg = append(reg, server.ServerTool{Tool: toolDeletePage, Handler: t.handleDeletePage})
	}
	mcpServer.AddTools(reg...)
}

func (t *tools) client(request mcp.CallToolRequest) (*Client, error) {
	token := request.Header.Get(HeaderXWikiToken)
	if token == "" {
		auth := request.Header.Get(HeaderAuthorization)
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if token == "" {
		token = t.cfg.DefaultToken
	}
	if token == "" {
		return nil, fmt.Errorf("no XWiki token provided: send it in the %q or %q HTTP header, or configure a default with XWIKI_TOKEN/--token",
			HeaderXWikiToken, HeaderAuthorization)
	}
	return NewClient(t.cfg.BaseURL, token, t.cfg.Timeout), nil
}

func prettyJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func strParam(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func intParam(args map[string]any, key string, def int64) int64 {
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return def
}

func nonNegative(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}

var toolListSpaces = mcp.NewTool("list_spaces",
	mcp.WithDescription("List all spaces in the wiki, including nested ones (flat list). Use to discover the space structure before working with pages."),
	mcp.WithNumber("start", mcp.Description("Pagination offset"), mcp.DefaultNumber(0)),
	mcp.WithNumber("number", mcp.Description("Maximum number of spaces to return"), mcp.DefaultNumber(200)),
)

var toolListPages = mcp.NewTool("list_pages",
	mcp.WithDescription("List pages in a space. Nested spaces are supported, e.g. 'Manuals/ansible' or 'Manuals.ansible'. Use WebHome to find a space's home page."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithNumber("start", mcp.Description("Pagination offset"), mcp.DefaultNumber(0)),
	mcp.WithNumber("number", mcp.Description("Maximum number of pages to return"), mcp.DefaultNumber(50)),
)

var toolGetPage = mcp.NewTool("get_page",
	mcp.WithDescription("Get a single page (title, content, version, author, syntax). Use to read a page before editing it."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
)

var toolSavePage = mcp.NewTool("save_page",
	mcp.WithDescription("Create or update a page (PUT, idempotent). Creates the page if it does not exist, updates it otherwise. Content uses XWiki 2.1 wiki syntax."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
	mcp.WithString("title", mcp.Description("Page title. Keeps existing title if omitted on update.")),
	mcp.WithString("content", mcp.Description("Page content in XWiki syntax (xwiki/2.1 by default)")),
	mcp.WithString("syntax", mcp.Description("Content syntax"), mcp.DefaultString("xwiki/2.1")),
	mcp.WithString("comment", mcp.Description("Edit comment stored in page history")),
)

var toolDeletePage = mcp.NewTool("delete_page",
	mcp.WithDescription("Delete a page. Available only when the server was started with --allow-delete."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
)

var toolSearchPages = mcp.NewTool("search_pages",
	mcp.WithDescription("Full-text search across the wiki. Use to find pages by keywords in title or content."),
	mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
	mcp.WithNumber("start", mcp.Description("Pagination offset"), mcp.DefaultNumber(0)),
	mcp.WithNumber("number", mcp.Description("Maximum number of results to return"), mcp.DefaultNumber(20)),
)

var toolGetPageHistory = mcp.NewTool("get_page_history",
	mcp.WithDescription("List page versions (version, author, date, comment)."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
	mcp.WithNumber("start", mcp.Description("Pagination offset"), mcp.DefaultNumber(0)),
	mcp.WithNumber("number", mcp.Description("Maximum number of versions to return"), mcp.DefaultNumber(50)),
)

var toolListAttachments = mcp.NewTool("list_attachments",
	mcp.WithDescription("List attachments of a page (name, size, version, author, date)."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
	mcp.WithNumber("start", mcp.Description("Pagination offset"), mcp.DefaultNumber(0)),
	mcp.WithNumber("number", mcp.Description("Maximum number of attachments to return"), mcp.DefaultNumber(50)),
)

func (t *tools) handleListSpaces(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	spaces, err := c.ListSpaces(ctx, int(nonNegative(intParam(args, "start", 0))), int(nonNegative(intParam(args, "number", 200))))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(prettyJSON(spaces)), nil
}

func (t *tools) handleListPages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space := strParam(args, "space")
	if space == "" {
		return mcp.NewToolResultError("missing required parameter: space"), nil
	}
	pages, err := c.ListPages(ctx, space, int(nonNegative(intParam(args, "start", 0))), int(nonNegative(intParam(args, "number", 50))))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(prettyJSON(pages)), nil
}

func (t *tools) handleGetPage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page := strParam(args, "space"), strParam(args, "page")
	if space == "" || page == "" {
		return mcp.NewToolResultError("missing required parameters: space, page"), nil
	}
	p, err := c.GetPage(ctx, space, page)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(prettyJSON(p)), nil
}

func (t *tools) handleSavePage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page := strParam(args, "space"), strParam(args, "page")
	if space == "" || page == "" {
		return mcp.NewToolResultError("missing required parameters: space, page"), nil
	}
	body := map[string]any{}
	if title := strParam(args, "title"); title != "" {
		body["title"] = title
	}
	if content, ok := args["content"]; ok && content != nil {
		body["content"] = content
	}
	syntax := strParam(args, "syntax")
	if syntax == "" {
		syntax = "xwiki/2.1"
	}
	body["syntax"] = syntax
	if comment := strParam(args, "comment"); comment != "" {
		body["comment"] = comment
	}
	result, err := c.SavePage(ctx, space, page, body)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	msg := fmt.Sprintf("Page %s/%s saved: %s", space, page, result.Status)
	if result.Location != "" {
		msg += "\nLocation: " + result.Location
	}
	return mcp.NewToolResultText(msg), nil
}

func (t *tools) handleDeletePage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page := strParam(args, "space"), strParam(args, "page")
	if space == "" || page == "" {
		return mcp.NewToolResultError("missing required parameters: space, page"), nil
	}
	if err := c.DeletePage(ctx, space, page); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Page %s/%s deleted", space, page)), nil
}

func (t *tools) handleSearchPages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	query := strParam(args, "query")
	if query == "" {
		return mcp.NewToolResultError("missing required parameter: query"), nil
	}
	results, err := c.Search(ctx, query, int(nonNegative(intParam(args, "start", 0))), int(nonNegative(intParam(args, "number", 20))))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(prettyJSON(results)), nil
}

func (t *tools) handleGetPageHistory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page := strParam(args, "space"), strParam(args, "page")
	if space == "" || page == "" {
		return mcp.NewToolResultError("missing required parameters: space, page"), nil
	}
	versions, err := c.ListPageHistory(ctx, space, page, int(nonNegative(intParam(args, "start", 0))), int(nonNegative(intParam(args, "number", 50))))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(prettyJSON(versions)), nil
}

func (t *tools) handleListAttachments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page := strParam(args, "space"), strParam(args, "page")
	if space == "" || page == "" {
		return mcp.NewToolResultError("missing required parameters: space, page"), nil
	}
	attachments, err := c.ListAttachments(ctx, space, page, int(nonNegative(intParam(args, "start", 0))), int(nonNegative(intParam(args, "number", 50))))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(prettyJSON(attachments)), nil
}
