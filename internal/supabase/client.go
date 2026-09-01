package supabase

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

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http: &http.Client{
			Timeout: 8 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        256,
				MaxIdleConnsPerHost: 128,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (c *Client) Select(ctx context.Context, table string, query url.Values, dst any) error {
	endpoint := c.baseURL + "/rest/v1/" + url.PathEscape(table)
	if encoded := query.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	c.authorize(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("supabase select %s: %w", table, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError("select", table, resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode supabase %s: %w", table, err)
	}
	return nil
}

func (c *Client) Insert(ctx context.Context, table string, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rest/v1/"+url.PathEscape(table), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	c.authorize(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=minimal")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("supabase insert %s: %w", table, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError("insert", table, resp)
	}
	return nil
}

func (c *Client) authorize(req *http.Request) {
	req.Header.Set("apikey", c.apiKey)
	// Supabase's modern sb_secret_* keys are opaque API keys, not JWT bearer tokens.
	if !strings.HasPrefix(c.apiKey, "sb_secret_") && !strings.HasPrefix(c.apiKey, "sb_publishable_") {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

func responseError(operation, table string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	return fmt.Errorf("supabase %s %s returned %s: %s", operation, table, resp.Status, strings.TrimSpace(string(body)))
}
