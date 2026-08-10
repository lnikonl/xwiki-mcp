package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

var version = "dev"

const instructions = `This server exposes an XWiki instance over the Model Context Protocol.

Authentication: the server calls the XWiki REST API on behalf of the user. The user token
must be sent on every HTTP request in the "X-XWiki-Token" header or as "Authorization: Bearer <token>".
If the server was started with a default token (XWIKI_TOKEN / --token) it is used as a fallback.

Nested spaces use the slash or dot notation, e.g. "Manuals/ansible" or "Manuals.ansible".

Wiki syntax: pages are stored in XWiki 2.1 syntax by default.`

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	addr := flag.String("addr", envOr("XWIKI_MCP_ADDR", "localhost:8080"), "HTTP listen address")
	xwikiURL := flag.String("xwiki-url", envOr("XWIKI_BASE_URL", ""), "XWiki REST API base URL, e.g. https://xwiki.example.com/rest/wikis/xwiki")
	token := flag.String("token", envOr("XWIKI_TOKEN", ""), "Default XWiki token used when the client sends none")
	allowDelete := flag.Bool("allow-delete", envBool("XWIKI_MCP_ALLOW_DELETE", false), "Enable the delete_page tool (off by default)")
	timeout := flag.Duration("timeout", envDuration("XWIKI_MCP_TIMEOUT", 60*time.Second), "XWiki HTTP request timeout")
	flag.Parse()

	if *showVersion {
		fmt.Println("xwiki-mcp", version)
		os.Exit(0)
	}

	if *xwikiURL == "" {
		fmt.Fprintln(os.Stderr, "error: --xwiki-url (or XWIKI_BASE_URL) is required")
		flag.Usage()
		os.Exit(1)
	}

	mcpServer := server.NewMCPServer(
		"xwiki-mcp",
		version,
		server.WithToolCapabilities(true),
		server.WithInstructions(instructions),
		server.WithTitle("XWiki MCP"),
		server.WithDescription("Access an XWiki instance: list, read, create, update, search pages."),
	)

	NewTools(ToolConfig{
		BaseURL:      *xwikiURL,
		DefaultToken: *token,
		AllowDelete:  *allowDelete,
		Timeout:      *timeout,
	}).Register(mcpServer)

	httpServer := server.NewStreamableHTTPServer(mcpServer)

	log.Printf("xwiki-mcp %s listening on %s, XWiki base: %s, delete tool: %v",
		version, *addr, *xwikiURL, *allowDelete)
	if err := http.ListenAndServe(*addr, httpServer); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
