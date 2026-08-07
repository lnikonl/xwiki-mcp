package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	HeaderAuthorization = "Authorization"
	HeaderXWikiToken    = "X-XWiki-Token"
)

type Page struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
	Space    string `json:"space"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Version  string `json:"version"`
	Author   string `json:"author"`
	Content  string `json:"content"`
	Syntax   string `json:"syntax"`
	URL      string `json:"url"`
}

type PageSummary struct {
	ID       string `json:"id"`
	FullName string `json:"fullName"`
	Space    string `json:"space"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Version  string `json:"version"`
	Author   string `json:"author"`
	URL      string `json:"url"`
}

type Space struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Home string `json:"home"`
	URL  string `json:"url"`
}

type SaveResult struct {
	Status   string
	Location string
}

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func NewClient(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: timeout},
	}
}

func spaceSegments(space string) []string {
	fields := strings.FieldsFunc(space, func(r rune) bool { return r == '/' || r == '.' })
	segs := make([]string, 0, len(fields)*2)
	for _, f := range fields {
		if f != "" {
			segs = append(segs, "spaces", url.PathEscape(f))
		}
	}
	return segs
}

func pagePath(space, page string) string {
	segs := spaceSegments(space)
	return strings.Join(append(segs, "pages", url.PathEscape(page)), "/")
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+"/"+strings.TrimLeft(path, "/"), reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.Token != "" {
		req.Header.Set(HeaderAuthorization, "Bearer "+c.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("xwiki %s %s failed: %s: %s", method, path, resp.Status, strings.TrimSpace(string(b)))
	}
	return resp, nil
}

func (c *Client) decode(ctx context.Context, method, path string, out any) error {
	resp, err := c.do(ctx, method, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode xwiki %s %s response: %w", method, path, err)
	}
	return nil
}

func (c *Client) ListSpaces(ctx context.Context, start, number int) ([]Space, error) {
	var out struct {
		Spaces []Space `json:"spaces"`
	}
	err := c.decode(ctx, http.MethodGet, fmt.Sprintf("/spaces?start=%d&number=%d", start, number), &out)
	return out.Spaces, err
}

func (c *Client) ListPages(ctx context.Context, space string, start, number int) ([]PageSummary, error) {
	var out struct {
		PageSummaries []PageSummary `json:"pageSummaries"`
	}
	err := c.decode(ctx, http.MethodGet,
		fmt.Sprintf("%s?start=%d&number=%d", strings.Join(spaceSegments(space), "/")+"/pages", start, number), &out)
	return out.PageSummaries, err
}

func (c *Client) GetPage(ctx context.Context, space, page string) (*Page, error) {
	var out Page
	err := c.decode(ctx, http.MethodGet, pagePath(space, page), &out)
	return &out, err
}

func (c *Client) SavePage(ctx context.Context, space, page string, body map[string]any) (*SaveResult, error) {
	resp, err := c.do(ctx, http.MethodPut, pagePath(space, page), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return &SaveResult{
		Status:   resp.Status,
		Location: resp.Header.Get("Location"),
	}, nil
}

func (c *Client) DeletePage(ctx context.Context, space, page string) error {
	resp, err := c.do(ctx, http.MethodDelete, pagePath(space, page), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) Search(ctx context.Context, query string, start, number int) ([]map[string]any, error) {
	var out struct {
		SearchResults []map[string]any `json:"searchResults"`
	}
	err := c.decode(ctx, http.MethodGet,
		fmt.Sprintf("/search?q=%s&start=%d&number=%d", url.QueryEscape(query), start, number), &out)
	return out.SearchResults, err
}

func (c *Client) ListPageHistory(ctx context.Context, space, page string, start, number int) ([]map[string]any, error) {
	var out struct {
		PageVersions []map[string]any `json:"pageVersions"`
	}
	err := c.decode(ctx, http.MethodGet,
		fmt.Sprintf("%s/history?start=%d&number=%d", pagePath(space, page), start, number), &out)
	return out.PageVersions, err
}

func (c *Client) ListAttachments(ctx context.Context, space, page string, start, number int) ([]map[string]any, error) {
	var out struct {
		Attachments []map[string]any `json:"attachments"`
	}
	err := c.decode(ctx, http.MethodGet,
		fmt.Sprintf("%s/attachments?start=%d&number=%d", pagePath(space, page), start, number), &out)
	return out.Attachments, err
}
