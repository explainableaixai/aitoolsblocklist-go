// Package aitoolsblocklist provides a client for AI Tools Blocklist.
package aitoolsblocklist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Result map[string]any
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string { return fmt.Sprintf("API request failed with status %d", e.Status) }

type Client struct {
	APIKey, BaseURL string
	HTTPClient      *http.Client
}

func New(key string) *Client {
	return &Client{APIKey: key, BaseURL: "https://www.aitoolsblocklist.com/api", HTTPClient: &http.Client{Timeout: 30 * time.Second}}
}
func (c *Client) lookup(ctx context.Context, value string) (Result, error) {
	if strings.TrimSpace(c.APIKey) == "" || strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("API key and input are required")
	}
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/check"
	var req *http.Request
	var err error
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("domain", value)
	u.RawQuery = q.Encode()
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err == nil {
		req.Header.Set("X-API-Key", c.APIKey)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 400 {
		return nil, &APIError{Status: resp.StatusCode, Body: string(raw)}
	}
	var result Result
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}
func (c *Client) Check(ctx context.Context, value string) (Result, error) {
	return c.lookup(ctx, value)
}
