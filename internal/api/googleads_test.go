package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type capturedRequest struct {
	method string
	path   string
	body   string
}

func captureServer(t *testing.T) (*httptest.Server, <-chan capturedRequest) {
	t.Helper()
	requests := make(chan capturedRequest, 10)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		requests <- capturedRequest{method: request.Method, path: request.URL.Path, body: string(body)}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	return server, requests
}

func assertRequest(t *testing.T, requests <-chan capturedRequest, method, path, body string) {
	t.Helper()
	got := <-requests
	if got.method != method {
		t.Errorf("method = %q, want %q", got.method, method)
	}
	if got.path != path {
		t.Errorf("path = %q, want %q", got.path, path)
	}
	if got.body != body {
		t.Errorf("body = %s, want %s", got.body, body)
	}
}

func TestSearchSerializesOptionsAndCustomerPath(t *testing.T) {
	t.Parallel()

	server, requests := captureServer(t)
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})

	_, err := client.Search(context.Background(), SearchOptions{
		CustomerID:         "1234567890",
		Query:              "SELECT campaign.id, metrics.cost_micros FROM campaign",
		PageToken:          "next-page",
		ValidateOnly:       true,
		ReturnTotalResults: true,
	})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	assertRequest(t, requests, http.MethodPost, "/v25/customers/1234567890/googleAds:search", `{"pageToken":"next-page","query":"SELECT campaign.id, metrics.cost_micros FROM campaign","returnTotalResults":true,"validateOnly":true}`)
}

func TestSearchOmitsUnsetOptionalFields(t *testing.T) {
	t.Parallel()

	server, requests := captureServer(t)
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})

	if _, err := client.Search(context.Background(), SearchOptions{CustomerID: "123", Query: "SELECT customer.id FROM customer"}); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	assertRequest(t, requests, http.MethodPost, "/v25/customers/123/googleAds:search", `{"query":"SELECT customer.id FROM customer"}`)
}

func TestSearchRequiresCustomerIDWithoutNetwork(t *testing.T) {
	t.Parallel()

	client := newTestClient("https://example.invalid", &staticTokenProvider{token: "token"})
	client.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("network called for invalid Search")
		return nil, nil
	})
	_, err := client.Search(context.Background(), SearchOptions{Query: "SELECT customer.id FROM customer"})
	if err == nil || err.Error() != "customer ID is required" {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchRetriesBecauseItIsReadOnly(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if calls.Add(1) == 1 {
			writer.WriteHeader(http.StatusBadGateway)
			_, _ = writer.Write([]byte(`{"error":{"status":"UNAVAILABLE"}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})

	if _, err := client.Search(context.Background(), SearchOptions{CustomerID: "123", Query: "SELECT customer.id FROM customer"}); err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("calls = %d, want 2", got)
	}
}

func TestSearchStreamSerializesQueryAndCustomerPath(t *testing.T) {
	t.Parallel()

	server, requests := captureServer(t)
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})
	query := "SELECT ad_group.id FROM ad_group"

	if _, err := client.SearchStream(context.Background(), "456", query); err != nil {
		t.Fatalf("SearchStream returned error: %v", err)
	}
	assertRequest(t, requests, http.MethodPost, "/v25/customers/456/googleAds:searchStream", `{"query":"SELECT ad_group.id FROM ad_group"}`)
}

func TestSearchStreamRequiresCustomerID(t *testing.T) {
	t.Parallel()

	client := newTestClient("https://example.invalid", &staticTokenProvider{token: "token"})
	_, err := client.SearchStream(context.Background(), "", "SELECT customer.id FROM customer")
	if err == nil || err.Error() != "customer ID is required" {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchStreamBoundedMergesBatchesAndStopsAtRowLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v25/customers/456/googleAds:searchStream" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := request.Header.Get("developer-token"); got != "developer-secret" {
			t.Errorf("developer-token = %q", got)
		}
		if got := request.Header.Get("login-customer-id"); got != "1234567890" {
			t.Errorf("login-customer-id = %q", got)
		}
		body, _ := io.ReadAll(request.Body)
		if got := string(body); got != `{"query":"SELECT campaign.id FROM campaign"}` {
			t.Errorf("body = %s", got)
		}
		writer.Header().Set("google-ads-request-id", "header-id")
		_, _ = writer.Write([]byte(`[
			{"results":[{"campaign":{"id":9007199254740993}},{"campaign":{"id":2}}],"fieldMask":"campaign.id","requestId":"batch-one"},
			{"results":[{"campaign":{"id":3}},{"campaign":{"id":4}}],"summaryRow":{"metrics":{"clicks":4}},"queryResourceConsumption":"9","requestId":"batch-two"}
		]`))
	}))
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})

	response, err := client.SearchStreamBounded(context.Background(), "456", "SELECT campaign.id FROM campaign", 3)
	if err != nil {
		t.Fatalf("SearchStreamBounded returned error: %v", err)
	}
	if response.RequestID != "batch-two" {
		t.Errorf("request ID = %q, want last decoded batch ID", response.RequestID)
	}
	root := response.Value.(map[string]any)
	rows := root["results"].([]any)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want bounded length 3", len(rows))
	}
	firstID := rows[0].(map[string]any)["campaign"].(map[string]any)["id"].(json.Number)
	if firstID.String() != "9007199254740993" {
		t.Errorf("first ID = %q", firstID)
	}
	if root["fieldMask"] != "campaign.id" {
		t.Errorf("fieldMask = %#v", root["fieldMask"])
	}
	if root["queryResourceConsumption"] != "9" {
		t.Errorf("queryResourceConsumption = %#v", root["queryResourceConsumption"])
	}
}

func TestSearchStreamBoundedValidatesArgumentsWithoutNetwork(t *testing.T) {
	t.Parallel()

	client := newTestClient("https://example.invalid", &staticTokenProvider{token: "token"})
	client.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("network called for invalid bounded stream")
		return nil, nil
	})
	if _, err := client.SearchStreamBounded(context.Background(), "", "query", 1); err == nil || err.Error() != "customer ID is required" {
		t.Errorf("missing customer error = %v", err)
	}
	if _, err := client.SearchStreamBounded(context.Background(), "123", "query", 0); err == nil || err.Error() != "max rows must be positive" {
		t.Errorf("invalid limit error = %v", err)
	}
}

func TestSearchStreamBoundedReturnsStructuredErrorAndRequestID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("request-id", "stream-request-id")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":{"status":"INVALID_ARGUMENT","message":"bad stream query"}}`))
	}))
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})

	response, err := client.SearchStreamBounded(context.Background(), "123", "bad", 10)
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *Error", err)
	}
	if response.RequestID != "stream-request-id" || apiErr.RequestID != "stream-request-id" {
		t.Errorf("response ID = %q, error ID = %q", response.RequestID, apiErr.RequestID)
	}
}

func TestSearchStreamBoundedHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	client := newTestClient("https://example.invalid", &staticTokenProvider{token: "token"})
	client.HTTP.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		cancel()
		<-request.Context().Done()
		return nil, request.Context().Err()
	})

	_, err := client.SearchStreamBounded(ctx, "123", "SELECT customer.id FROM customer", 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestSearchFieldsSerializesPagination(t *testing.T) {
	t.Parallel()

	server, requests := captureServer(t)
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})

	if _, err := client.SearchFields(context.Background(), "campaign.id", "field-page", 100); err != nil {
		t.Fatalf("SearchFields returned error: %v", err)
	}
	assertRequest(t, requests, http.MethodPost, "/v25/googleAdsFields:search", `{"pageSize":100,"pageToken":"field-page","query":"campaign.id"}`)
}

func TestSearchFieldsOmitsUnsetPagination(t *testing.T) {
	t.Parallel()

	server, requests := captureServer(t)
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})

	if _, err := client.SearchFields(context.Background(), "campaign", "", 0); err != nil {
		t.Fatalf("SearchFields returned error: %v", err)
	}
	assertRequest(t, requests, http.MethodPost, "/v25/googleAdsFields:search", `{"query":"campaign"}`)
}

func TestListAccessibleCustomersUsesBodylessGET(t *testing.T) {
	t.Parallel()

	server, requests := captureServer(t)
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})

	if _, err := client.ListAccessibleCustomers(context.Background()); err != nil {
		t.Fatalf("ListAccessibleCustomers returned error: %v", err)
	}
	assertRequest(t, requests, http.MethodGet, "/v25/customers:listAccessibleCustomers", "")
}

func TestReadOperationsAreReplaySafe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(*Client) error
	}{
		{name: "search stream", call: func(client *Client) error {
			_, err := client.SearchStream(context.Background(), "123", "SELECT customer.id FROM customer")
			return err
		}},
		{name: "fields", call: func(client *Client) error {
			_, err := client.SearchFields(context.Background(), "customer.id", "", 0)
			return err
		}},
		{name: "accessible customers", call: func(client *Client) error {
			_, err := client.ListAccessibleCustomers(context.Background())
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			client := newTestClient("https://example.invalid", &staticTokenProvider{token: "token"})
			client.HTTP.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
				status := http.StatusOK
				body := `{}`
				if calls.Add(1) == 1 {
					status = http.StatusGatewayTimeout
					body = `{"error":{"status":"DEADLINE_EXCEEDED"}}`
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: request}, nil
			})
			if err := test.call(client); err != nil {
				t.Fatalf("operation returned error: %v", err)
			}
			if got := calls.Load(); got != 2 {
				t.Errorf("calls = %d, want 2", got)
			}
		})
	}
}

func TestMutateSerializesSafetyFlagsOperationsAndCustomerPath(t *testing.T) {
	t.Parallel()

	server, requests := captureServer(t)
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})
	operations := []any{
		map[string]any{"campaignOperation": map[string]any{"updateMask": "status", "update": map[string]any{"resourceName": "customers/789/campaigns/42", "status": "PAUSED"}}},
		map[string]any{"adGroupOperation": map[string]any{"remove": "customers/789/adGroups/9007199254740993"}},
	}

	if _, err := client.Mutate(context.Background(), "789", operations, true, true); err != nil {
		t.Fatalf("Mutate returned error: %v", err)
	}
	assertRequest(t, requests, http.MethodPost, "/v25/customers/789/googleAds:mutate", `{"mutateOperations":[{"campaignOperation":{"update":{"resourceName":"customers/789/campaigns/42","status":"PAUSED"},"updateMask":"status"}},{"adGroupOperation":{"remove":"customers/789/adGroups/9007199254740993"}}],"partialFailure":true,"responseContentType":"MUTABLE_RESOURCE","validateOnly":true}`)
}

func TestMutateRequiresCustomerID(t *testing.T) {
	t.Parallel()

	client := newTestClient("https://example.invalid", &staticTokenProvider{token: "token"})
	_, err := client.Mutate(context.Background(), "", []any{}, false, false)
	if err == nil || err.Error() != "customer ID is required" {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateOnlyMutationRetries(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if calls.Add(1) == 1 {
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte(`{"error":{"status":"INTERNAL"}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"mutateOperationResponses":[]}`))
	}))
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})

	if _, err := client.Mutate(context.Background(), "123", []any{map[string]any{"campaignOperation": map[string]any{}}}, false, true); err != nil {
		t.Fatalf("validate-only Mutate returned error: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("calls = %d, want 2", got)
	}
}

func TestExecutingMutationNeverRetries(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte(`{"error":{"status":"UNAVAILABLE"}}`))
	}))
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})

	_, err := client.Mutate(context.Background(), "123", []any{map[string]any{"campaignOperation": map[string]any{}}}, false, false)
	if err == nil {
		t.Fatal("executing Mutate returned nil error")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want exactly 1", got)
	}
}

func TestMutateResponsePreservesLargeResourceIdentifiers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`{"mutateOperationResponses":[{"campaignResult":{"resourceName":"customers/123/campaigns/90071992547409931234","id":90071992547409931234}}]}`))
	}))
	defer server.Close()
	client := newTestClient(server.URL, &staticTokenProvider{token: "token"})

	response, err := client.Mutate(context.Background(), "123", []any{}, false, false)
	if err != nil {
		t.Fatalf("Mutate returned error: %v", err)
	}
	root := response.Value.(map[string]any)
	responses := root["mutateOperationResponses"].([]any)
	campaign := responses[0].(map[string]any)["campaignResult"].(map[string]any)
	if got := campaign["id"].(json.Number).String(); got != "90071992547409931234" {
		t.Errorf("id = %q", got)
	}
}
