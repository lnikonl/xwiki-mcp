# xwiki-mcp

MCP server (Go, [mcp-go](https://github.com/mark3labs/mcp-go) v0.55.0) that exposes an XWiki instance over HTTP (Streamable HTTP transport).

Flow: LLM sends its XWiki user token in the HTTP header -> xwiki-mcp authenticates to XWiki REST API on behalf of the user -> XWiki.

## Authentication

The client sends its token on every request in one of:

- `X-XWiki-Token: <token>`
- `Authorization: Bearer <token>`

If neither is present, the default token configured at startup (`--token` / `XWIKI_TOKEN`) is used.

## Configuration

| Flag            | Env                    | Default             | Description                                          |
| --------------- | ---------------------- | ------------------- | ---------------------------------------------------- |
| `-addr`         | `XWIKI_MCP_ADDR`       | `localhost:8080`    | HTTP listen address                                  |
| `-xwiki-url`    | `XWIKI_BASE_URL`       | — (required)        | XWiki REST base, e.g. `https://xwiki.example.com/rest/wikis/xwiki` |
| `-token`        | `XWIKI_TOKEN`          | —                   | Default token fallback                               |
| `-allow-delete` | `XWIKI_MCP_ALLOW_DELETE` | `false`           | Register the `delete_page` tool                      |
| `-timeout`      | `XWIKI_MCP_TIMEOUT`    | `60s`               | XWiki HTTP request timeout                           |

## Run

```bash
go build -o xwiki-mcp .
./xwiki-mcp -addr :8080 -xwiki-url https://xwiki.example.com/rest/wikis/xwiki
```

## Tools

- `list_spaces` — list all (nested) spaces
- `list_pages` — list pages in a space; nested spaces as `Manuals/ansible` or `Manuals.ansible`
- `get_page` — read page (title, content, version, author, syntax)
- `save_page` — create/update page (PUT, idempotent; content in XWiki 2.1 syntax)
- `search_pages` — full-text search
- `get_page_history` — page versions
- `list_attachments` — page attachments
- `delete_page` — only when `-allow-delete` is set
