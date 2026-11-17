// Package seal0 is the Go client for the Seal0 entitlement API.
//
// It is a thin HTTP client: every entitlement decision is made by the API,
// never by the SDK. Request and response types are generated from the API's
// OpenAPI contract (types.gen.go).
//
//	client, err := seal0.New(os.Getenv("SEAL0_API_KEY"))
//	ev, err := client.Evaluate(ctx, "cus_123", "max_projects")
//	if err != nil {
//		// Unknown customer or feature, auth or network failure: fail closed.
//	}
//	if n, unlimited, ok := ev.Entitlement.Value.Limit(); ok { ... }
//
// Keep API keys on the server; never ship them to browsers or mobile apps.
package seal0

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Version is the SDK version, sent in the User-Agent.
const Version = "0.1.0"

// DefaultBaseURL is the production API endpoint.
const DefaultBaseURL = "https://api.seal0.app"

// Client calls the Seal0 API. It is safe for concurrent use.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int
	backoff    time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API endpoint (for example a local server).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient sets the HTTP client, including its timeout and transport.
// The default client times out after 10 seconds.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithMaxRetries sets how many times a failed request is retried (default 2).
// Network errors, 429, 502, 503, 504 and in-progress idempotent requests are
// retried; nothing else is.
func WithMaxRetries(n int) Option { return func(c *Client) { c.maxRetries = max(n, 0) } }

// New creates a client for an API key (s0_dev_… or s0_prod_…).
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("seal0: API key is required")
	}
	c := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		maxRetries: 2,
		backoff:    200 * time.Millisecond,
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

// RequestOption configures a single call.
type RequestOption func(*requestConfig)

type requestConfig struct {
	idempotencyKey string
}

// WithIdempotencyKey sets the Idempotency-Key for a mutation. Without it, the
// SDK generates one per call and reuses it across that call's retries.
func WithIdempotencyKey(key string) RequestOption {
	return func(rc *requestConfig) { rc.idempotencyKey = key }
}

// request describes one API call.
type request struct {
	method   string
	path     string
	query    url.Values
	body     any  // JSON-encoded when non-nil
	mutation bool // sends an Idempotency-Key
}

// response is a successful (2xx) reply.
type response struct {
	status int
	body   []byte
}

// do sends req with retries and decodes a 2xx body into out (if non-nil).
func (c *Client) do(ctx context.Context, req request, out any, opts []RequestOption) (response, error) {
	var rc requestConfig
	for _, o := range opts {
		o(&rc)
	}
	var payload []byte
	if req.body != nil {
		var err error
		if payload, err = json.Marshal(req.body); err != nil {
			return response{}, fmt.Errorf("seal0: encode request: %w", err)
		}
	}
	key := rc.idempotencyKey
	if req.mutation && key == "" {
		key = newIdempotencyKey()
	}
	target := c.baseURL + req.path
	if len(req.query) > 0 {
		target += "?" + req.query.Encode()
	}

	for attempt := 0; ; attempt++ {
		res, retryAfter, err := c.send(ctx, req.method, target, payload, key)
		if err == nil {
			if out != nil && len(res.body) > 0 {
				if err := json.Unmarshal(res.body, out); err != nil {
					return res, fmt.Errorf("seal0: decode response: %w", err)
				}
			}
			return res, nil
		}
		if attempt >= c.maxRetries || !retryable(err) || ctx.Err() != nil {
			return response{}, err
		}
		if err := sleep(ctx, c.delay(attempt, retryAfter)); err != nil {
			return response{}, err
		}
	}
}

// send performs one HTTP attempt.
func (c *Client) send(ctx context.Context, method, target string, payload []byte, key string) (response, time.Duration, error) {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	hreq, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return response{}, 0, fmt.Errorf("seal0: build request: %w", err)
	}
	hreq.Header.Set("Authorization", "Bearer "+c.apiKey)
	hreq.Header.Set("Accept", "application/json")
	hreq.Header.Set("User-Agent", "seal0-go/"+Version)
	if payload != nil {
		hreq.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		hreq.Header.Set("Idempotency-Key", key)
	}
	hres, err := c.httpClient.Do(hreq)
	if err != nil {
		return response{}, 0, &transportError{err: err}
	}
	defer hres.Body.Close()
	data, err := io.ReadAll(io.LimitReader(hres.Body, 8<<20))
	if err != nil {
		return response{}, 0, &transportError{err: err}
	}
	if hres.StatusCode >= 200 && hres.StatusCode < 300 {
		return response{status: hres.StatusCode, body: data}, 0, nil
	}
	return response{}, retryAfter(hres.Header.Get("Retry-After")), decodeError(hres, data)
}

// transportError wraps a network failure (no HTTP response).
type transportError struct{ err error }

func (e *transportError) Error() string { return "seal0: request failed: " + e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }

func decodeError(res *http.Response, data []byte) error {
	var env struct {
		Error struct {
			Code      string  `json:"code"`
			Message   string  `json:"message"`
			Field     *string `json:"field"`
			RequestID string  `json:"request_id"`
		} `json:"error"`
	}
	e := &APIError{Status: res.StatusCode, RequestID: res.Header.Get("X-Request-Id")}
	if json.Unmarshal(data, &env) == nil && env.Error.Code != "" {
		e.Code, e.Message = env.Error.Code, env.Error.Message
		if env.Error.Field != nil {
			e.Field = *env.Error.Field
		}
		if env.Error.RequestID != "" {
			e.RequestID = env.Error.RequestID
		}
		return e
	}
	e.Code = "http_" + strconv.Itoa(res.StatusCode)
	e.Message = http.StatusText(res.StatusCode)
	return e
}

// retryable reports whether a failed attempt may be repeated. Every request
// the SDK sends is safe to repeat: reads and evaluation are read-only, and
// mutations carry an idempotency key.
func retryable(err error) bool {
	var te *transportError
	if errors.As(err, &te) {
		return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
	}
	var ae *APIError
	if !errors.As(err, &ae) {
		return false
	}
	switch ae.Status {
	case http.StatusTooManyRequests:
		return ae.Code != "usage_limit_exceeded" // lasts until the period resets
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	case http.StatusConflict:
		return ae.Code == "idempotency_in_progress"
	}
	return false
}

// delay returns the wait before retry attempt+1: Retry-After when given,
// otherwise exponential backoff with full jitter, capped at 5 seconds.
func (c *Client) delay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return min(retryAfter, 30*time.Second)
	}
	ceiling := min(c.backoff<<attempt, 5*time.Second)
	return time.Duration(rand.Int64N(int64(ceiling) + 1))
}

func retryAfter(h string) time.Duration {
	if secs, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func newIdempotencyKey() string {
	b := make([]byte, 16)
	_, _ = crand.Read(b) // crypto/rand.Read never fails on supported platforms
	return "sdk_" + hex.EncodeToString(b)
}
