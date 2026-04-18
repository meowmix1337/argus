package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultBodyLimit int64 = 1 << 20 // 1 MB

// HTTPError represents a non-2xx response from an upstream API.
type HTTPError struct {
	StatusCode int
	Body       string // first 512 bytes of response body for diagnostics
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http %d: %s", e.StatusCode, e.Body)
}

// HTTPClient is the interface for making HTTP requests. Mockable in tests.
type HTTPClient interface {
	Get(ctx context.Context, url string, result any, opts ...RequestOption) error
	Post(ctx context.Context, url string, body any, result any, opts ...RequestOption) error
	Put(ctx context.Context, url string, body any, result any, opts ...RequestOption) error
	Delete(ctx context.Context, url string, result any, opts ...RequestOption) error
	GetBytes(ctx context.Context, url string, opts ...RequestOption) ([]byte, error)
}

// ClientOption configures the client at construction time.
type ClientOption func(*client)

// WithDefaultHeaders sets headers applied to every request.
func WithDefaultHeaders(h map[string]string) ClientOption {
	return func(c *client) { c.defaultHeaders = h }
}

// WithBodyLimit sets the maximum response body size in bytes.
func WithBodyLimit(n int64) ClientOption {
	return func(c *client) { c.bodyLimit = n }
}

// RequestOption configures a single request.
type RequestOption func(*requestConfig)

type requestConfig struct {
	headers     map[string]string
	queryParams map[string]string
}

// WithHeaders sets multiple headers on a single request.
func WithHeaders(h map[string]string) RequestOption {
	return func(rc *requestConfig) { rc.headers = h }
}

// WithHeader sets a single header on a request.
func WithHeader(key, value string) RequestOption {
	return func(rc *requestConfig) {
		if rc.headers == nil {
			rc.headers = make(map[string]string)
		}
		rc.headers[key] = value
	}
}

// WithQueryParams adds query parameters to a request.
func WithQueryParams(p map[string]string) RequestOption {
	return func(rc *requestConfig) { rc.queryParams = p }
}

type client struct {
	http           *http.Client
	defaultHeaders map[string]string
	bodyLimit      int64
}

// New creates an HTTPClient wrapping the given *http.Client.
func New(httpClient *http.Client, opts ...ClientOption) HTTPClient {
	c := &client{
		http:      httpClient,
		bodyLimit: defaultBodyLimit,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *client) Get(ctx context.Context, url string, result any, opts ...RequestOption) error {
	return c.doRequest(ctx, http.MethodGet, url, nil, result, opts...)
}

func (c *client) Post(ctx context.Context, url string, body any, result any, opts ...RequestOption) error {
	return c.doRequest(ctx, http.MethodPost, url, body, result, opts...)
}

func (c *client) Put(ctx context.Context, url string, body any, result any, opts ...RequestOption) error {
	return c.doRequest(ctx, http.MethodPut, url, body, result, opts...)
}

func (c *client) Delete(ctx context.Context, url string, result any, opts ...RequestOption) error {
	return c.doRequest(ctx, http.MethodDelete, url, nil, result, opts...)
}

// GetBytes fetches a URL and returns the raw response body as bytes.
// Used for non-JSON responses (e.g., ICS calendar feeds).
func (c *client) GetBytes(ctx context.Context, url string, opts ...RequestOption) ([]byte, error) {
	rc := c.buildRequestConfig(opts)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req, rc)
	c.applyQueryParams(req, rc)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.bodyLimit))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.newHTTPError(resp.StatusCode, body)
	}

	return body, nil
}

func (c *client) doRequest(ctx context.Context, method, url string, body any, result any, opts ...RequestOption) error {
	rc := c.buildRequestConfig(opts)

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.applyHeaders(req, rc)
	c.applyQueryParams(req, rc)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, c.bodyLimit))
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.newHTTPError(resp.StatusCode, respBody)
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

func (c *client) buildRequestConfig(opts []RequestOption) requestConfig {
	var rc requestConfig
	for _, opt := range opts {
		opt(&rc)
	}
	return rc
}

func (c *client) applyHeaders(req *http.Request, rc requestConfig) {
	for k, v := range c.defaultHeaders {
		req.Header.Set(k, v)
	}
	for k, v := range rc.headers {
		req.Header.Set(k, v)
	}
}

func (c *client) applyQueryParams(req *http.Request, rc requestConfig) {
	if len(rc.queryParams) > 0 {
		q := req.URL.Query()
		for k, v := range rc.queryParams {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}
}

func (c *client) newHTTPError(statusCode int, body []byte) *HTTPError {
	snippet := string(body)
	if len(snippet) > 512 {
		snippet = snippet[:512]
	}
	return &HTTPError{
		StatusCode: statusCode,
		Body:       snippet,
	}
}
