// Package hello demonstrates an outbound JSON API client. It is not wired into startup.
package hello

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"goempty/internal/entity"
)

const maxResponseBytes = 1 << 20

type Config struct {
	BaseURL string
	Timeout time.Duration
}

type Repository interface {
	Hello(context.Context) (entity.HelloResponse, error)
}

// Client reuses its HTTP client across calls; construct it once per upstream.
type Client struct {
	http     *http.Client
	endpoint string
}

var _ Repository = (*Client)(nil)

func New(cfg Config) (*Client, error) {
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery || base.Fragment != "" {
		return nil, errors.New("hello upstream: invalid base URL")
	}
	if cfg.Timeout < 0 {
		return nil, errors.New("hello upstream: timeout must not be negative")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &Client{http: &http.Client{Timeout: cfg.Timeout}, endpoint: strings.TrimRight(base.String(), "/") + "/hello"}, nil
}

// Hello expects GET /hello to return {"message":"hello"}, without a JSON envelope.
// The example does not call this application's enveloped /hello endpoint.
func (c *Client) Hello(ctx context.Context) (entity.HelloResponse, error) {
	var result entity.HelloResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return result, errors.New("hello upstream: create request failed")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		// Preserve cancellation/timeout identity without including the request URL.
		var requestErr *url.Error
		if errors.As(err, &requestErr) {
			err = requestErr.Err
		}
		return result, fmt.Errorf("hello upstream: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("hello upstream: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return result, fmt.Errorf("hello upstream: read response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return result, errors.New("hello upstream: response too large")
	}
	if json.Unmarshal(body, &result) != nil || result.Message == "" {
		return entity.HelloResponse{}, errors.New("hello upstream: invalid response")
	}
	return result, nil
}
