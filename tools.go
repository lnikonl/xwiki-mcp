package main

import (
	"context"
	"encoding/base64"
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
		{Tool: toolGetPageVersion, Handler: t.handleGetPageVersion},
		{Tool: toolListPageChildren, Handler: t.handleListPageChildren},
		{Tool: toolSavePage, Handler: t.handleSavePage},
		{Tool: toolSearchPages, Handler: t.handleSearchPages},
		{Tool: toolGetPageHistory, Handler: t.handleGetPageHistory},
		{Tool: toolListAttachments, Handler: t.handleListAttachments},
		{Tool: toolUploadAttachment, Handler: t.handleUploadAttachment},
		{Tool: toolDownloadAttachment, Handler: t.handleDownloadAttachment},
		{Tool: toolListComments, Handler: t.handleListComments},
		{Tool: toolAddComment, Handler: t.handleAddComment},
		{Tool: toolGetModifications, Handler: t.handleGetModifications},
		{Tool: toolListTags, Handler: t.handleListTags},
		{Tool: toolSetPageTags, Handler: t.handleSetPageTags},
		{Tool: toolGetPagesByTag, Handler: t.handleGetPagesByTag},
	}
	if t.cfg.AllowDelete {
		reg = append(reg,
			server.ServerTool{Tool: toolDeletePage, Handler: t.handleDeletePage},
			server.ServerTool{Tool: toolDeleteAttachment, Handler: t.handleDeleteAttachment},
		)
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

func strSliceParam(args map[string]any, key string) []string {
	v, ok := args[key]
	if !ok {
		return nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func dedupStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
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

var toolGetPageVersion = mcp.NewTool("get_page_version",
	mcp.WithDescription("Get the metadata and content of a specific page version (revision), e.g. '2.1'."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
	mcp.WithString("version", mcp.Required(), mcp.Description("Page revision to return, e.g. '2.1'")),
)

var toolListPageChildren = mcp.NewTool("list_page_children",
	mcp.WithDescription("List child pages of a page. Use hierarchy 'nestedpages' to match the XWiki UI hierarchy, 'parentchild' (default) for the legacy one."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
	mcp.WithNumber("start", mcp.Description("Pagination offset"), mcp.DefaultNumber(0)),
	mcp.WithNumber("number", mcp.Description("Maximum number of children to return"), mcp.DefaultNumber(50)),
	mcp.WithString("hierarchy", mcp.Description("'parentchild' (default) or 'nestedpages'")),
	mcp.WithString("search", mcp.Description("Filter children by name or title")),
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

var toolUploadAttachment = mcp.NewTool("upload_attachment",
	mcp.WithDescription("Upload a file as an attachment to a page (PUT, creates or updates). Content must be base64-encoded."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
	mcp.WithString("filename", mcp.Required(), mcp.Description("File name of the attachment, e.g. 'image.png'")),
	mcp.WithString("content_base64", mcp.Required(), mcp.Description("File content encoded as base64")),
)

var toolDownloadAttachment = mcp.NewTool("download_attachment",
	mcp.WithDescription("Download an attachment of a page. Images are returned as rendered image content; other files are returned base64-encoded in text."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
	mcp.WithString("filename", mcp.Required(), mcp.Description("File name of the attachment to download, e.g. 'image.png'")),
)

var toolDeleteAttachment = mcp.NewTool("delete_attachment",
	mcp.WithDescription("Delete an attachment of a page. Available only when the server was started with --allow-delete."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
	mcp.WithString("filename", mcp.Required(), mcp.Description("File name of the attachment to delete")),
)

var toolListComments = mcp.NewTool("list_comments",
	mcp.WithDescription("List comments of a page (id, author, date, text, highlight, replyTo)."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
	mcp.WithNumber("start", mcp.Description("Pagination offset"), mcp.DefaultNumber(0)),
	mcp.WithNumber("number", mcp.Description("Maximum number of comments to return; -1 means no limit"), mcp.DefaultNumber(-1)),
)

var toolAddComment = mcp.NewTool("add_comment",
	mcp.WithDescription("Add a comment to a page. The author is set server-side to the current user."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
	mcp.WithString("text", mcp.Required(), mcp.Description("Comment text (XWiki syntax)")),
	mcp.WithString("highlight", mcp.Description("Optional highlighted text of the page the comment refers to")),
	mcp.WithNumber("reply_to", mcp.Description("Optional id of the comment being replied to")),
)

var toolGetModifications = mcp.NewTool("get_modifications",
	mcp.WithDescription("List the latest modifications (individual document versions) made in the wiki, ordered by modification date. Use to see what changed recently."),
	mcp.WithNumber("start", mcp.Description("Pagination offset"), mcp.DefaultNumber(0)),
	mcp.WithNumber("number", mcp.Description("Maximum number of modifications to return"), mcp.DefaultNumber(25)),
	mcp.WithString("order", mcp.Description("'desc' (most recent first, default) or 'asc'"), mcp.DefaultString("desc")),
	mcp.WithNumber("date", mcp.Description("Only modifications made after this instant, milliseconds since epoch (optional)")),
)

var toolListTags = mcp.NewTool("list_tags",
	mcp.WithDescription("List all tags defined in the wiki, sorted case-insensitively by name."),
)

var toolSetPageTags = mcp.NewTool("set_page_tags",
	mcp.WithDescription("Assign tags to a page (XWiki REST PUT /tags, replaces the page's tag set). Use mode 'replace' to set the exact list of tags (empty list clears all tags); use mode 'add' to merge the given tags with the existing ones."),
	mcp.WithString("space", mcp.Required(), mcp.Description("Space path, e.g. 'Main' or 'Manuals/ansible'")),
	mcp.WithString("page", mcp.Required(), mcp.Description("Page name, e.g. 'WebHome'")),
	mcp.WithArray("tags", mcp.Required(), mcp.WithStringItems(), mcp.Description("Tags to assign to the page")),
	mcp.WithString("mode", mcp.Description("'replace' (default) or 'add'"), mcp.DefaultString("replace")),
	mcp.WithBoolean("minor_revision", mcp.Description("Save the change as a minor revision"), mcp.DefaultBool(false)),
)

var toolGetPagesByTag = mcp.NewTool("get_pages_by_tag",
	mcp.WithDescription("List pages tagged with any of the given tags (logical OR), ordered alphabetically by full name."),
	mcp.WithArray("tags", mcp.Required(), mcp.WithStringItems(), mcp.Description("Tag names to look up")),
	mcp.WithNumber("start", mcp.Description("Pagination offset"), mcp.DefaultNumber(0)),
	mcp.WithNumber("number", mcp.Description("Maximum number of pages to return"), mcp.DefaultNumber(50)),
	mcp.WithBoolean("pretty_names", mcp.Description("Include human-readable display names (slower)"), mcp.DefaultBool(false)),
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

func (t *tools) handleGetPageVersion(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page, version := strParam(args, "space"), strParam(args, "page"), strParam(args, "version")
	if space == "" || page == "" || version == "" {
		return mcp.NewToolResultError("missing required parameters: space, page, version"), nil
	}
	p, err := c.GetPageVersion(ctx, space, page, version)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(prettyJSON(p)), nil
}

func (t *tools) handleListPageChildren(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page := strParam(args, "space"), strParam(args, "page")
	if space == "" || page == "" {
		return mcp.NewToolResultError("missing required parameters: space, page"), nil
	}
	children, err := c.ListPageChildren(ctx, space, page,
		int(nonNegative(intParam(args, "start", 0))), int(nonNegative(intParam(args, "number", 50))),
		strParam(args, "hierarchy"), strParam(args, "search"))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(prettyJSON(children)), nil
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

func (t *tools) handleUploadAttachment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page, name := strParam(args, "space"), strParam(args, "page"), strParam(args, "filename")
	if space == "" || page == "" || name == "" {
		return mcp.NewToolResultError("missing required parameters: space, page, filename"), nil
	}
	content, ok := args["content_base64"].(string)
	if !ok {
		return mcp.NewToolResultError("missing required parameter: content_base64"), nil
	}
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid base64 content: %v", err)), nil
	}
	result, err := c.UploadAttachment(ctx, space, page, name, data)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	msg := fmt.Sprintf("Attachment %s uploaded to %s/%s: %s (%d bytes)", name, space, page, result.Status, len(data))
	if result.Location != "" {
		msg += "\nLocation: " + result.Location
	}
	return mcp.NewToolResultText(msg), nil
}

func (t *tools) handleDownloadAttachment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page, name := strParam(args, "space"), strParam(args, "page"), strParam(args, "filename")
	if space == "" || page == "" || name == "" {
		return mcp.NewToolResultError("missing required parameters: space, page, filename"), nil
	}
	data, mimeType, err := c.DownloadAttachment(ctx, space, page, name)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	mimeType = strings.TrimSpace(strings.Split(mimeType, ";")[0])
	caption := fmt.Sprintf("Attachment %s from %s/%s: %d bytes, %s", name, space, page, len(data), mimeType)
	if strings.HasPrefix(mimeType, "image/") {
		return mcp.NewToolResultImage(caption, base64.StdEncoding.EncodeToString(data), mimeType), nil
	}
	return mcp.NewToolResultText(caption + "\nContent (base64):\n" + base64.StdEncoding.EncodeToString(data)), nil
}

func (t *tools) handleDeleteAttachment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page, name := strParam(args, "space"), strParam(args, "page"), strParam(args, "filename")
	if space == "" || page == "" || name == "" {
		return mcp.NewToolResultError("missing required parameters: space, page, filename"), nil
	}
	if err := c.DeleteAttachment(ctx, space, page, name); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Attachment %s deleted from %s/%s", name, space, page)), nil
}

func (t *tools) handleListComments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page := strParam(args, "space"), strParam(args, "page")
	if space == "" || page == "" {
		return mcp.NewToolResultError("missing required parameters: space, page"), nil
	}
	comments, err := c.ListComments(ctx, space, page,
		int(nonNegative(intParam(args, "start", 0))), int(intParam(args, "number", -1)))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(prettyJSON(comments)), nil
}

func (t *tools) handleAddComment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page := strParam(args, "space"), strParam(args, "page")
	text := strParam(args, "text")
	if space == "" || page == "" || text == "" {
		return mcp.NewToolResultError("missing required parameters: space, page, text"), nil
	}
	var replyTo *int
	if v := intParam(args, "reply_to", -1); v >= 0 {
		n := int(v)
		replyTo = &n
	}
	result, err := c.AddComment(ctx, space, page, text, strParam(args, "highlight"), replyTo)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	msg := fmt.Sprintf("Comment added to %s/%s: %s", space, page, result.Status)
	if result.Location != "" {
		msg += "\nLocation: " + result.Location
	}
	return mcp.NewToolResultText(msg), nil
}

func (t *tools) handleGetModifications(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	order := strParam(args, "order")
	if order == "" {
		order = "desc"
	}
	mods, err := c.GetModifications(ctx,
		int(nonNegative(intParam(args, "start", 0))), int(nonNegative(intParam(args, "number", 25))),
		order, int64(intParam(args, "date", 0)))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(prettyJSON(mods)), nil
}

func (t *tools) handleListTags(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	tags, err := c.ListTags(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(prettyJSON(tags)), nil
}

func (t *tools) handleSetPageTags(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	space, page := strParam(args, "space"), strParam(args, "page")
	if space == "" || page == "" {
		return mcp.NewToolResultError("missing required parameters: space, page"), nil
	}
	tags := dedupStrings(strSliceParam(args, "tags"))
	mode := strParam(args, "mode")
	if mode == "" {
		mode = "replace"
	}
	switch mode {
	case "replace", "add":
	default:
		return mcp.NewToolResultError(fmt.Sprintf("invalid mode %q: expected 'replace' or 'add'", mode)), nil
	}
	minorRevision := false
	if v, ok := args["minor_revision"].(bool); ok {
		minorRevision = v
	}
	if mode == "add" && len(tags) > 0 {
		existing, err := c.GetPageTags(ctx, space, page)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		for _, t := range existing {
			tags = append(tags, t.Name)
		}
		tags = dedupStrings(tags)
	}
	result, err := c.SetPageTags(ctx, space, page, tags, minorRevision)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	msg := fmt.Sprintf("Tags assigned to %s/%s: %s (%s)", space, page, strings.Join(tags, ", "), result.Status)
	if result.Location != "" {
		msg += "\nLocation: " + result.Location
	}
	return mcp.NewToolResultText(msg), nil
}

func (t *tools) handleGetPagesByTag(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, err := t.client(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := request.GetArguments()
	tags := strSliceParam(args, "tags")
	if len(tags) == 0 {
		return mcp.NewToolResultError("missing required parameter: tags"), nil
	}
	prettyNames := false
	if v, ok := args["pretty_names"].(bool); ok {
		prettyNames = v
	}
	pages, err := c.GetPagesByTag(ctx, tags, int(nonNegative(intParam(args, "start", 0))), int(nonNegative(intParam(args, "number", 50))), prettyNames)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(prettyJSON(pages)), nil
}
