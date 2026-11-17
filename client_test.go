package seal0

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const testKey = "s0_dev_TESTKEY00000_0000000000000000000000000000000000000000000"

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(testKey, WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	c.backoff = time.Millisecond // keep retry tests fast
	return c
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func TestValueEncoding(t *testing.T) {
	for _, tt := range []struct {
		v    Value
		json string
	}{
		{BoolValue(true), `true`},
		{NumberValue(0), `0`},
		{NumberValue(MaxNumber), `9007199254740991`},
		{UnlimitedValue(), `"unlimited"`},
		{EnumValue("csv"), `"csv"`},
	} {
		b, err := json.Marshal(tt.v)
		if err != nil || string(b) != tt.json {
			t.Errorf("Marshal(%v) = %s, %v; want %s", tt.v, b, err, tt.json)
		}
		var back Value
		if err := json.Unmarshal(b, &back); err != nil || back != tt.v {
			t.Errorf("Unmarshal(%s) = %v, %v", b, back, err)
		}
	}
	for _, bad := range []string{`1.5`, `{}`, `null`, `[1]`} {
		var v Value
		if err := json.Unmarshal([]byte(bad), &v); err == nil {
			t.Errorf("Unmarshal(%s) succeeded", bad)
		}
	}
	if _, err := json.Marshal(Value{}); err == nil {
		t.Error("zero Value marshalled")
	}
	n, unlimited, ok := UnlimitedValue().Limit()
	if n != 0 || !unlimited || !ok {
		t.Error("Limit() on unlimited")
	}
	if _, _, ok := BoolValue(true).Limit(); ok {
		t.Error("Limit() on boolean reported ok")
	}
}

func TestRequestShapeAndHeaders(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testKey || r.Header.Get("Content-Type") != "application/json" ||
			!strings.HasPrefix(r.Header.Get("User-Agent"), "seal0-go/") {
			t.Errorf("headers = %v", r.Header)
		}
		if r.Header.Get("Idempotency-Key") != "" {
			t.Error("evaluation must not send an Idempotency-Key")
		}
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/v1/entitlements/evaluate" || string(body) != `{"customer_id":"cus_1","feature_key":"max_projects"}` {
			t.Errorf("request = %s %s", r.URL.Path, body)
		}
		writeJSON(w, 200, `{"customer_id":"cus_1","evaluated_at":"2026-01-01T00:00:00Z","subscription":null,
			"entitlement":{"feature_key":"max_projects","feature_type":"number","granted":true,"value":"unlimited",
			"source":"override","reason":"customer_override","override_id":"o1","valid_until":null,"future_field":1}}`)
	})
	ev, err := c.Evaluate(context.Background(), "cus_1", "max_projects")
	if err != nil {
		t.Fatal(err)
	}
	if !ev.Entitlement.Value.IsUnlimited() || ev.Entitlement.Source != SourceOverride || ev.Subscription != nil {
		t.Fatalf("evaluation = %+v", ev)
	}
}

func TestPathEscaping(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/v1/customers/org:42%2Fx" {
			t.Errorf("path = %s", r.URL.EscapedPath())
		}
		writeJSON(w, 200, `{"id":"1","external_customer_id":"org:42/x","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`)
	})
	if _, err := c.GetCustomer(context.Background(), "org:42/x"); err != nil {
		t.Fatal(err)
	}
}

func TestAPIErrors(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req_header")
		switch r.URL.Path {
		case "/v1/customers/missing":
			writeJSON(w, 404, `{"error":{"code":"customer_not_found","message":"customer was not found","field":null,"request_id":"req_1"}}`)
		default:
			w.WriteHeader(418)
		}
	})
	_, err := c.GetCustomer(context.Background(), "missing")
	var ae *APIError
	if !errors.Is(err, ErrCustomerNotFound) || !errors.As(err, &ae) || ae.Status != 404 || ae.RequestID != "req_1" {
		t.Fatalf("err = %v", err)
	}
	if errors.Is(err, ErrFeatureNotFound) {
		t.Error("matched the wrong sentinel")
	}
	_, err = c.GetCustomer(context.Background(), "teapot")
	if !errors.As(err, &ae) || ae.Code != "http_418" || ae.RequestID != "req_header" {
		t.Fatalf("non-JSON error = %v", err)
	}
}

func TestRetriesReuseIdempotencyKey(t *testing.T) {
	var (
		mu   sync.Mutex
		keys []string
	)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		n := len(keys)
		mu.Unlock()
		switch n {
		case 1:
			writeJSON(w, 503, `{"error":{"code":"not_ready","message":"x","field":null,"request_id":"r"}}`)
		case 2:
			writeJSON(w, 409, `{"error":{"code":"idempotency_in_progress","message":"x","field":null,"request_id":"r"}}`)
		default:
			writeJSON(w, 201, `{"id":"p1","key":"pro","name":"Pro","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`)
		}
	})
	p, err := c.CreatePlan(context.Background(), CreatePlanRequest{Key: "pro", Name: "Pro"})
	if err != nil || p.Key != "pro" {
		t.Fatalf("CreatePlan = %+v, %v", p, err)
	}
	if len(keys) != 3 || keys[0] == "" || keys[0] != keys[1] || keys[1] != keys[2] {
		t.Fatalf("idempotency keys across retries = %q", keys)
	}
}

func TestExplicitIdempotencyKey(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") != "mine-1" {
			t.Errorf("key = %q", r.Header.Get("Idempotency-Key"))
		}
		writeJSON(w, 201, `{"id":"p1","key":"pro","name":"Pro","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`)
	})
	if _, err := c.CreatePlan(context.Background(), CreatePlanRequest{Key: "pro", Name: "Pro"}, WithIdempotencyKey("mine-1")); err != nil {
		t.Fatal(err)
	}
}

func TestNoRetryOnClientErrors(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(w, 422, `{"error":{"code":"invalid_key","message":"x","field":"key","request_id":"r"}}`)
	})
	_, err := c.CreatePlan(context.Background(), CreatePlanRequest{Key: "Bad", Name: "x"})
	var ae *APIError
	if calls != 1 || !errors.As(err, &ae) || ae.Field != "key" {
		t.Fatalf("calls = %d, err = %v", calls, err)
	}
}

func TestUsageLimitExceededIsNotRetried(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeJSON(w, 429, `{"error":{"code":"usage_limit_exceeded","message":"x","field":null,"request_id":"r"}}`)
	})
	_, err := c.Evaluate(context.Background(), "cus_test_0001", "api_calls")
	if calls != 1 || !errors.Is(err, ErrUsageLimitExceeded) {
		t.Fatalf("calls = %d, err = %v", calls, err)
	}

	// Other 429s (rate limits) are still retried.
	calls = 0
	c = newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			writeJSON(w, 429, `{"error":{"code":"rate_limited","message":"x","field":null,"request_id":"r"}}`)
			return
		}
		writeJSON(w, 200, `{"customer_id":"cus_test_0001","feature_key":"api_calls","used":1,"period_start":"2026-01-01T00:00:00Z","period_end":"2026-02-01T00:00:00Z"}`)
	})
	u, err := c.RecordUsage(context.Background(), "cus_test_0001", "api_calls", 1)
	if err != nil || calls != 2 || u.Used != 1 {
		t.Fatalf("calls = %d, usage = %+v, err = %v", calls, u, err)
	}
}

func TestRetriesGiveUpAndRespectContext(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(502)
	})
	_, err := c.GetPlan(context.Background(), "pro")
	if calls != 3 || err == nil {
		t.Fatalf("calls = %d (want 1 + 2 retries), err = %v", calls, err)
	}

	slow, err := New(testKey, WithBaseURL(c.baseURL), WithMaxRetries(5))
	if err != nil {
		t.Fatal(err)
	}
	slow.backoff = time.Hour
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := slow.GetPlan(ctx, "pro"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("retry backoff ignored context cancellation")
	}
}

func TestPatchEncoding(t *testing.T) {
	name := "Renamed"
	b, err := json.Marshal(FeaturePatch{Name: &name, Description: Null[string](), DefaultValue: Set(NumberValue(3))})
	if err != nil || string(b) != `{"default_value":3,"description":null,"name":"Renamed"}` {
		t.Fatalf("FeaturePatch = %s, %v", b, err)
	}
	b, _ = json.Marshal(FeaturePatch{})
	if string(b) != `{}` {
		t.Fatalf("empty FeaturePatch = %s", b)
	}
	status := StatusActive
	at := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	b, _ = json.Marshal(SubscriptionPatch{Status: &status, CancelAt: Null[time.Time](), CurrentPeriodEnd: Set(at)})
	if string(b) != `{"cancel_at":null,"current_period_end":"2026-02-01T00:00:00Z","status":"active"}` {
		t.Fatalf("SubscriptionPatch = %s", b)
	}
}

func TestUpsertCustomerCreatedFlag(t *testing.T) {
	status := 201
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, status, `{"id":"1","external_customer_id":"cus_1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`)
	})
	if _, created, err := c.UpsertCustomer(context.Background(), "cus_1"); err != nil || !created {
		t.Fatalf("created = %t, %v", created, err)
	}
	status = 200
	if _, created, err := c.UpsertCustomer(context.Background(), "cus_1"); err != nil || created {
		t.Fatalf("created = %t, %v", created, err)
	}
}

func TestIteratorFollowsCursors(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		feature := func(k string) string {
			return `{"id":"` + k + `","key":"` + k + `","name":"x","description":null,"value_type":"boolean","enum_options":null,"default_value":null,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`
		}
		if r.URL.Query().Get("cursor") == "" {
			writeJSON(w, 200, `{"data":[`+feature("a")+`,`+feature("b")+`],"next_cursor":"c1"}`)
			return
		}
		writeJSON(w, 200, `{"data":[`+feature("c")+`],"next_cursor":null}`)
	})
	var keys []string
	for f, err := range c.Features(context.Background()) {
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, f.Key)
	}
	if strings.Join(keys, ",") != "a,b,c" {
		t.Fatalf("keys = %v", keys)
	}
}

func TestNewRequiresKey(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("New accepted an empty key")
	}
}
