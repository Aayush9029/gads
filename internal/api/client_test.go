package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type staticTokenProvider struct {
	token string
	err   error
	calls atomic.Int32
}

func (p *staticTokenProvider) AccessToken(context.Context) (string, error) {
	p.calls.Add(1)
	return p.token, p.err
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func newTestClient(endpoint string, tokens TokenProvider) *Client {
	return &Client{
		HTTP:            &http.Client{},
		Tokens:          tokens,
		Endpoint:        endpoint,
		Version:         "v25",
		DeveloperToken:  "developer-secret",
		LoginCustomerID: "1234567890",
		MaxAttempts:     3,
		Sleep: func(context.Context, time.Duration) error {
			return nil
		},
		Jitter: func(duration time.Duration) time.Duration {
			return duration
		},
	}
}

func TestClientDoSendsAuthenticationAndCustomerHeaders(t *testing.T) {
	t.Parallel()

	tokens := &staticTokenProvider{token: "oauth-secret"}
	var gotMethod, gotPath, gotQuery, gotBody string
	var gotHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotMethod = request.Method
		gotPath = request.URL.Path
		gotQuery = request.URL.RawQuery
		gotHeaders = request.Header.Clone()
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		gotBody = string(body)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL+"/", tokens)
	client.LinkedCustomerID = "0987654321"
	response, err := client.Do(context.Background(), Request{
		Method: http.MethodPost,
		Path:   "/customers/111/googleAds:search",
		Query:  url.Values{"z": {"last"}, "a": {"first"}},
		Body:   map[string]any{"query": "SELECT customer.id"},
	})
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}

	if response.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v25/customers/111/googleAds:search" {
		t.Errorf("path = %q", gotPath)
	}
	if gotQuery != "a=first&z=last" {
		t.Errorf("query = %q, want deterministic encoded query", gotQuery)
	}
	if gotBody != `{"query":"SELECT customer.id"}` {
		t.Errorf("body = %s", gotBody)
	}
	if got := gotHeaders.Get("Authorization"); got != "Bearer oauth-secret" {
		t.Errorf("Authorization = %q", got)
	}
	if got := gotHeaders.Get("developer-token"); got != "developer-secret" {
		t.Errorf("developer-token = %q", got)
	}
	if got := gotHeaders.Get("login-customer-id"); got != "1234567890" {
		t.Errorf("login-customer-id = %q", got)
	}
	if got := gotHeaders.Get("linked-customer-id"); got != "0987654321" {
		t.Errorf("linked-customer-id = %q", got)
	}
	if got := gotHeaders.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q", got)
	}
	if got := gotHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := tokens.calls.Load(); got != 1 {
		t.Errorf("token calls = %d, want 1", got)
	}
}

func TestClientDoOmitsOptionalHeadersAndContentTypeForBodylessRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("login-customer-id"); got != "" {
			t.Errorf("login-customer-id = %q, want omitted", got)
		}
		if got := request.Header.Get("linked-customer-id"); got != "" {
			t.Errorf("linked-customer-id = %q, want omitted", got)
		}
		if got := request.Header.Get("Content-Type"); got != "" {
			t.Errorf("Content-Type = %q, want omitted", got)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})
	client.LoginCustomerID = ""
	if _, err := client.Do(context.Background(), Request{Method: http.MethodGet, Path: "customers:listAccessibleCustomers"}); err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
}

func TestClientDoPreservesRequestAndResponseNumberPrecision(t *testing.T) {
	t.Parallel()

	const preciseInteger = "900719925474099312345678901234567890"
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		gotBody = string(body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"resource":{"id":` + preciseInteger + `,"micros":1234567890123456789}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})
	response, err := client.Do(context.Background(), Request{
		Method: http.MethodPost,
		Path:   "precision",
		Body: map[string]any{
			"id":     json.Number(preciseInteger),
			"micros": json.Number("1234567890123456789"),
		},
	})
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if gotBody != `{"id":`+preciseInteger+`,"micros":1234567890123456789}` {
		t.Errorf("body lost precision: %s", gotBody)
	}
	root := response.Value.(map[string]any)
	resource := root["resource"].(map[string]any)
	if got := resource["id"].(json.Number).String(); got != preciseInteger {
		t.Errorf("id = %q, want %q", got, preciseInteger)
	}
	if got := resource["micros"].(json.Number).String(); got != "1234567890123456789" {
		t.Errorf("micros = %q", got)
	}
}

func TestClientDoAcceptsValidRawJSONWithoutReencoding(t *testing.T) {
	t.Parallel()

	const raw = `{"operations":[{"amountMicros":9007199254740993123456789}],"spacing" : true}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if got := string(body); got != raw {
			t.Errorf("raw body = %q, want %q", got, raw)
		}
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})
	if _, err := client.Do(context.Background(), Request{Method: http.MethodPost, Path: "raw", Body: []byte(raw)}); err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
}

func TestClientDoRejectsInvalidRawJSONBeforeAuthenticationOrNetwork(t *testing.T) {
	t.Parallel()

	tokens := &staticTokenProvider{token: "token"}
	var networkCalls atomic.Int32
	client := newTestClient("https://unused.invalid", tokens)
	client.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		networkCalls.Add(1)
		return nil, errors.New("unexpected network call")
	})

	_, err := client.Do(context.Background(), Request{Method: http.MethodPost, Path: "raw", Body: []byte(`{"broken"`)})
	if err == nil || err.Error() != "request body is not valid JSON" {
		t.Fatalf("error = %v", err)
	}
	if got := tokens.calls.Load(); got != 0 {
		t.Errorf("token calls = %d, want 0", got)
	}
	if got := networkCalls.Load(); got != 0 {
		t.Errorf("network calls = %d, want 0", got)
	}
}

func TestClientDoReportsStructuredAPIErrorAndHeaderRequestID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("google-ads-request-id", "header-request-id")
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{
            "error": {
              "code": 400,
              "message": "Request contains an invalid argument.",
              "status": "INVALID_ARGUMENT",
              "details": [{
                "requestId": "body-request-id",
                "errors": [{
                  "errorCode": {"queryError": "UNRECOGNIZED_FIELD"},
                  "message": "Unknown field.",
                  "trigger": {"stringValue": "campaign.unknown"},
                  "location": {"fieldPathElements": [
                    {"fieldName": "query"},
                    {"fieldName": "fields", "index": 2}
                  ]}
                }]
              }]
            }
          }`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})
	response, err := client.Do(context.Background(), Request{Method: http.MethodPost, Path: "bad", Body: map[string]string{"query": "bad"}, ReplaySafe: true})
	if err == nil {
		t.Fatal("Do returned nil error")
	}
	if response.RequestID != "header-request-id" {
		t.Errorf("response request ID = %q", response.RequestID)
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *Error", err)
	}
	if apiErr.HTTPStatus != http.StatusBadRequest || apiErr.Status != "INVALID_ARGUMENT" {
		t.Errorf("API error = %#v", apiErr)
	}
	if apiErr.RequestID != "header-request-id" {
		t.Errorf("request ID = %q, want header request ID", apiErr.RequestID)
	}
	if len(apiErr.Details) != 1 {
		t.Fatalf("details = %#v", apiErr.Details)
	}
	detail := apiErr.Details[0]
	if detail.Code != "queryError.UNRECOGNIZED_FIELD" || detail.Message != "Unknown field." || detail.Trigger != "campaign.unknown" || detail.Location != "query.fields[2]" {
		t.Errorf("detail = %#v", detail)
	}
	for _, fragment := range []string{"INVALID_ARGUMENT", "queryError.UNRECOGNIZED_FIELD", "query.fields[2]", "header-request-id"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("error %q does not contain %q", err, fragment)
		}
	}
}

func TestClientDoUsesRequestIDFromErrorBodyWhenHeaderMissing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
		_, _ = writer.Write([]byte(`{"error":{"status":"PERMISSION_DENIED","message":"denied","details":[{"requestId":"body-id"}]}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})
	_, err := client.Do(context.Background(), Request{Method: http.MethodGet, Path: "denied", ReplaySafe: true})
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *Error", err)
	}
	if apiErr.RequestID != "body-id" {
		t.Errorf("request ID = %q, want body-id", apiErr.RequestID)
	}
}

func TestClientDoRetriesReplaySafeServerFailures(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	var sleeps atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call := calls.Add(1)
		if call < 3 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = writer.Write([]byte(`{"error":{"status":"UNAVAILABLE","message":"try again"}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})
	client.Sleep = func(context.Context, time.Duration) error {
		sleeps.Add(1)
		return nil
	}
	response, err := client.Do(context.Background(), Request{Method: http.MethodGet, Path: "safe", ReplaySafe: true})
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Errorf("status = %d", response.StatusCode)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
	if got := sleeps.Load(); got != 2 {
		t.Errorf("sleeps = %d, want 2", got)
	}
}

func TestClientDoRetriesReplaySafeTransportFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	client := newTestClient("https://example.invalid", &staticTokenProvider{token: "token"})
	client.HTTP.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("temporary transport failure")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request:    request,
		}, nil
	})

	if _, err := client.Do(context.Background(), Request{Method: http.MethodGet, Path: "safe", ReplaySafe: true}); err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("calls = %d, want 2", got)
	}
}

func TestClientDoDoesNotRetryNonReplaySafeRequest(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte(`{"error":{"status":"UNAVAILABLE","message":"try again"}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})
	_, err := client.Do(context.Background(), Request{Method: http.MethodPost, Path: "mutation", Body: map[string]any{"operation": "create"}})
	if err == nil {
		t.Fatal("Do returned nil error")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1", got)
	}
}

func TestClientDoStopsRetryWhenContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte(`{"error":{"status":"UNAVAILABLE"}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})
	client.Sleep = func(ctx context.Context, _ time.Duration) error {
		cancel()
		<-ctx.Done()
		return ctx.Err()
	}
	_, err := client.Do(ctx, Request{Method: http.MethodGet, Path: "safe", ReplaySafe: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1", got)
	}
}

func TestClientDoReturnsContextCancellationFromTransport(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	client := newTestClient("https://example.invalid", &staticTokenProvider{token: "token"})
	client.HTTP.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		cancel()
		<-request.Context().Done()
		return nil, request.Context().Err()
	})

	_, err := client.Do(ctx, Request{Method: http.MethodGet, Path: "cancelled", ReplaySafe: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestClientDoRequiresTokenProvider(t *testing.T) {
	t.Parallel()

	client := newTestClient("https://example.invalid", nil)
	_, err := client.Do(context.Background(), Request{Method: http.MethodGet, Path: "anything", ReplaySafe: true})
	if err == nil || err.Error() != "OAuth token provider is not configured" {
		t.Fatalf("error = %v", err)
	}
}

func TestClientDoPropagatesTokenProviderErrorWithoutNetwork(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("refresh failed")
	tokens := &staticTokenProvider{err: wantErr}
	client := newTestClient("https://example.invalid", tokens)
	client.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("network must not be called after token error")
		return nil, nil
	})
	_, err := client.Do(context.Background(), Request{Method: http.MethodGet, Path: "anything", ReplaySafe: true})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}
