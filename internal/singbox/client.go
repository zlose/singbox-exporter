package singbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	secret  string
	http    *http.Client
}

type StatusError struct {
	Code int
}

func (e StatusError) Error() string {
	return fmt.Sprintf("sing-box API returned HTTP %d", e.Code)
}

func (e StatusError) IsAuthFailure() bool {
	return e.Code == http.StatusUnauthorized || e.Code == http.StatusForbidden
}

func NewClient(baseURL, secret string, requestTimeout time.Duration, maxIdleConnsPerHost int) *Client {
	maxIdleConns := max(maxIdleConnsPerHost, 256)
	transport := &http.Transport{
		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: maxIdleConns,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		secret:  secret,
		http: &http.Client{
			Transport: transport,
			Timeout:   requestTimeout,
		},
	}
}

func (c *Client) ListProxies(ctx context.Context) ([]Proxy, error) {
	var response ProxiesResponse
	if err := c.getJSON(ctx, "/proxies", &response); err != nil {
		return nil, err
	}

	proxies := make([]Proxy, 0, len(response.Proxies))
	for name, proxy := range response.Proxies {
		proxy.Name = name
		proxies = append(proxies, proxy)
	}
	return proxies, nil
}

func (c *Client) ProbeDelay(ctx context.Context, name, probeURL string, timeout time.Duration) (int, error) {
	query := url.Values{}
	query.Set("timeout", fmt.Sprintf("%d", timeout.Milliseconds()))
	query.Set("url", probeURL)
	path := "/proxies/" + url.PathEscape(name) + "/delay?" + query.Encode()

	var response DelayResponse
	if err := c.getJSON(ctx, path, &response); err != nil {
		return 0, err
	}
	return response.Delay, nil
}

func (c *Client) getJSON(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return StatusError{Code: resp.StatusCode}
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode sing-box response: %w", err)
	}
	return nil
}
