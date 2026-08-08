package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Aayush9029/gads/internal/api"
	"github.com/Aayush9029/gads/internal/config"
	"github.com/Aayush9029/gads/internal/plan"
)

type memoryStore struct {
	mu          sync.Mutex
	current     string
	profiles    map[string]config.Profile
	credentials map[string]config.Credentials
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		current:     "default",
		profiles:    map[string]config.Profile{},
		credentials: map[string]config.Credentials{},
	}
}

func (s *memoryStore) CurrentProfile() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == "" {
		return "default", nil
	}
	return s.current, nil
}

func (s *memoryStore) SetCurrentProfile(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.profiles[name]; !ok {
		return errors.New("profile does not exist")
	}
	s.current = name
	return nil
}

func (s *memoryStore) ListProfiles() (map[string]config.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profiles := make(map[string]config.Profile, len(s.profiles))
	for name, profile := range s.profiles {
		profiles[name] = profile
	}
	return profiles, nil
}

func (s *memoryStore) LoadProfile(name string) (config.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile, ok := s.profiles[name]
	if !ok {
		return config.Profile{}, errors.New("profile does not exist")
	}
	return profile, nil
}

func (s *memoryStore) SaveProfile(name string, profile config.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles[name] = profile
	if s.current == "" {
		s.current = name
	}
	return nil
}

func (s *memoryStore) LoadCredentials(name string) (config.Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	credentials, ok := s.credentials[name]
	if !ok {
		return config.Credentials{}, errors.New("credentials do not exist")
	}
	return credentials, nil
}

func (s *memoryStore) SaveCredentials(name string, credentials config.Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.credentials[name] = credentials
	return nil
}

func (s *memoryStore) DeleteCredentials(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.credentials, name)
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type invocation struct {
	stdout bytes.Buffer
	stderr bytes.Buffer
	err    error
}

var fixedNow = time.Date(2026, time.August, 7, 12, 34, 56, 0, time.UTC)

func runCLI(t *testing.T, store config.Store, client *http.Client, args ...string) invocation {
	t.Helper()
	for _, key := range []string{
		"GADS_PROFILE", "GADS_CUSTOMER_ID", "GADS_LOGIN_CUSTOMER_ID", "GADS_LINKED_CUSTOMER_ID",
		"GADS_API_VERSION", "GADS_OUTPUT", "GADS_CLIENT_ID", "GADS_CLIENT_SECRET",
		"GADS_DEVELOPER_TOKEN", "GADS_REFRESH_TOKEN", "GADS_API_ENDPOINT",
	} {
		t.Setenv(key, "")
	}
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), args, Dependencies{
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Version:   "1.2.3-test",
		ConfigDir: t.TempDir(),
		Store:     store,
		HTTP:      client,
		Now:       func() time.Time { return fixedNow },
		Sleep:     func(context.Context, time.Duration) error { return nil },
	})
	return invocation{stdout: stdout, stderr: stderr, err: err}
}

func authenticatedStore(profile config.Profile) *memoryStore {
	store := newMemoryStore()
	store.profiles["default"] = profile
	store.credentials["default"] = config.Credentials{
		ClientID:       "client-id",
		ClientSecret:   "client-secret",
		DeveloperToken: "developer-token",
		RefreshToken:   "refresh-token",
	}
	return store
}

func jsonResponse(status int, requestID, body string) *http.Response {
	header := make(http.Header)
	if requestID != "" {
		header.Set("request-id", requestID)
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func tokenThenAPI(t *testing.T, handler func(*http.Request, []byte) *http.Response) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if request.URL.Host == "oauth2.googleapis.com" {
			values, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("parse OAuth form: %v", err)
			}
			if got := values.Get("refresh_token"); got != "refresh-token" {
				t.Errorf("refresh_token = %q, want refresh-token", got)
			}
			return jsonResponse(http.StatusOK, "", `{"access_token":"access-token","expires_in":3600}`), nil
		}
		return handler(request, body), nil
	})}
}

func decodeObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode JSON object: %v\n%s", err, data)
	}
	return value
}

func decodePlan(t *testing.T, data []byte) plan.Plan {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value plan.Plan
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decode plan: %v\n%s", err, data)
	}
	return value
}

func TestHelpVersionAndExitCodes(t *testing.T) {
	t.Run("empty invocation prints help to stdout", func(t *testing.T) {
		result := runCLI(t, newMemoryStore(), nil)
		if result.err != nil {
			t.Fatalf("Run() error = %v", result.err)
		}
		if !strings.Contains(result.stdout.String(), "Usage:") || !strings.Contains(result.stdout.String(), "Write safety:") {
			t.Fatalf("help output is incomplete:\n%s", result.stdout.String())
		}
		if result.stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want empty", result.stderr.String())
		}
	})

	t.Run("markdown help contains command inventory", func(t *testing.T) {
		result := runCLI(t, newMemoryStore(), nil, "help", "--format", "markdown")
		if result.err != nil {
			t.Fatalf("Run() error = %v", result.err)
		}
		for _, command := range []string{"`gads query run`", "`gads campaign activate`", "`gads mutate execute`"} {
			if !strings.Contains(result.stdout.String(), command) {
				t.Errorf("markdown help does not contain %s", command)
			}
		}
	})

	t.Run("version is machine clean", func(t *testing.T) {
		result := runCLI(t, newMemoryStore(), nil, "--version")
		if result.err != nil {
			t.Fatalf("Run() error = %v", result.err)
		}
		if got, want := result.stdout.String(), "gads 1.2.3-test\n"; got != want {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
		if result.stderr.Len() != 0 {
			t.Fatalf("stderr = %q, want empty", result.stderr.String())
		}
	})

	t.Run("usage and API errors have stable exit codes", func(t *testing.T) {
		result := runCLI(t, newMemoryStore(), nil, "unknown", "command")
		if got := ExitCode(result.err); got != 2 {
			t.Fatalf("ExitCode(%v) = %d, want 2", result.err, got)
		}
		if result.stdout.Len() != 0 || result.stderr.Len() != 0 {
			t.Fatalf("failed Run wrote stdout=%q stderr=%q", result.stdout.String(), result.stderr.String())
		}
		if got := ExitCode(context.Canceled); got != 130 {
			t.Fatalf("canceled exit code = %d, want 130", got)
		}
		if got := ExitCode(&api.Error{HTTPStatus: 400}); got != 4 {
			t.Fatalf("API exit code = %d, want 4", got)
		}
	})

	t.Run("missing credentials is an authentication error", func(t *testing.T) {
		store := newMemoryStore()
		store.profiles["default"] = config.Profile{CustomerID: "1234567890"}
		result := runCLI(t, store, nil, "query", "run", "--query", "SELECT customer.id FROM customer")
		if got := ExitCode(result.err); got != 3 {
			t.Fatalf("ExitCode(%v) = %d, want 3", result.err, got)
		}
		if !strings.Contains(result.err.Error(), "developer token is missing") {
			t.Fatalf("error = %q", result.err)
		}
	})
}

func TestProfilePrecedenceAndRequestHeaders(t *testing.T) {
	t.Run("flag-selected profile overrides environment and current profile", func(t *testing.T) {
		store := newMemoryStore()
		store.current = "current"
		store.profiles["current"] = config.Profile{CustomerID: "1111111111"}
		store.profiles["environment"] = config.Profile{CustomerID: "2222222222"}
		store.profiles["flag"] = config.Profile{CustomerID: "3333333333"}
		t.Setenv("GADS_PROFILE", "environment")
		var stdout bytes.Buffer
		err := Run(context.Background(), []string{"profile", "show", "--profile", "flag", "--output", "json"}, Dependencies{
			Stdout: &stdout, Stderr: io.Discard, Store: store, ConfigDir: t.TempDir(),
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		value := decodeObject(t, stdout.Bytes())
		if got := value["name"]; got != "flag" {
			t.Fatalf("selected profile = %v, want flag", got)
		}
		profile, _ := value["profile"].(map[string]any)
		if got := profile["customer_id"]; got != "3333333333" {
			t.Fatalf("profile customer = %v", got)
		}
	})

	store := authenticatedStore(config.Profile{
		CustomerID:       "1111111111",
		LoginCustomerID:  "2222222222",
		LinkedCustomerID: "3333333333",
		APIVersion:       "v23",
		Output:           "table",
	})
	client := tokenThenAPI(t, func(request *http.Request, body []byte) *http.Response {
		if got, want := request.URL.Path, "/v25/customers/5555555555/googleAds:search"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		if got, want := request.Header.Get("login-customer-id"), "7777777777"; got != want {
			t.Errorf("login-customer-id = %q, want %q", got, want)
		}
		if got, want := request.Header.Get("linked-customer-id"), "6666666666"; got != want {
			t.Errorf("linked-customer-id = %q, want %q", got, want)
		}
		if got, want := request.Header.Get("developer-token"), "env-developer-token"; got != want {
			t.Errorf("developer-token = %q, want %q", got, want)
		}
		if got, want := request.Header.Get("Authorization"), "Bearer access-token"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		if got, want := request.Header.Get("Content-Type"), "application/json"; got != want {
			t.Errorf("Content-Type = %q, want %q", got, want)
		}
		payload := decodeObject(t, body)
		if got, want := payload["query"], "SELECT customer.id FROM customer"; got != want {
			t.Errorf("query body = %v, want %q", got, want)
		}
		return jsonResponse(http.StatusOK, "request-profile", `{"results":[{"customer":{"id":"5555555555"}}]}`)
	})

	for _, keyValue := range [][2]string{
		{"GADS_CUSTOMER_ID", "4444444444"},
		{"GADS_LOGIN_CUSTOMER_ID", "8888888888"},
		{"GADS_LINKED_CUSTOMER_ID", "6666666666"},
		{"GADS_API_VERSION", "v24"},
		{"GADS_OUTPUT", "csv"},
		{"GADS_DEVELOPER_TOKEN", "env-developer-token"},
	} {
		t.Setenv(keyValue[0], keyValue[1])
	}
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{
		"query", "run", "--query", "SELECT customer.id FROM customer",
		"--customer", "555-555-5555", "--login-customer", "777-777-7777",
		"--api-version", "v25", "--output", "json",
	}, Dependencies{
		Stdout: &stdout, Stderr: &stderr, Store: store, HTTP: client, ConfigDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"id": "5555555555"`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestAuthStatusNeverPrintsSecrets(t *testing.T) {
	store := authenticatedStore(config.Profile{APIVersion: "v25", Output: "json"})
	result := runCLI(t, store, nil, "auth", "status", "--output", "json")
	if result.err != nil {
		t.Fatalf("Run() error = %v", result.err)
	}
	output := result.stdout.String()
	for _, secret := range []string{"client-id", "client-secret", "developer-token", "refresh-token"} {
		if strings.Contains(output, secret) {
			t.Errorf("auth status contains %q: %s", secret, output)
		}
	}
	for _, field := range []string{`"connected": true`, `"client_id_configured": true`, `"developer_token_configured": true`} {
		if !strings.Contains(output, field) {
			t.Errorf("auth status missing %s: %s", field, output)
		}
	}
}

func TestOAuthLoginWithFakeBrowserAndExchange(t *testing.T) {
	store := newMemoryStore()
	var exchangeForm url.Values
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read exchange body: %v", err)
		}
		exchangeForm, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse exchange body: %v", err)
		}
		return jsonResponse(http.StatusOK, "", `{"refresh_token":"oauth-refresh"}`), nil
	})}
	browser := func(ctx context.Context, target string) error {
		parsed, err := url.Parse(target)
		if err != nil {
			return err
		}
		query := parsed.Query()
		if query.Get("scope") != "https://www.googleapis.com/auth/adwords" {
			t.Errorf("scope = %q", query.Get("scope"))
		}
		if query.Get("access_type") != "offline" || query.Get("prompt") != "consent" {
			t.Errorf("OAuth URL does not request durable consent: %s", target)
		}
		callback, err := url.Parse(query.Get("redirect_uri"))
		if err != nil {
			return err
		}
		values := callback.Query()
		values.Set("state", query.Get("state"))
		values.Set("code", "authorization-code")
		callback.RawQuery = values.Encode()
		response, err := http.DefaultClient.Do((&http.Request{Method: http.MethodGet, URL: callback}).WithContext(ctx))
		if err != nil {
			return err
		}
		response.Body.Close()
		return nil
	}
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{
		"auth", "login", "--client-id", "oauth-client", "--client-secret", "oauth-secret",
		"--developer-token", "oauth-developer", "--output", "json",
	}, Dependencies{
		Stdout: &stdout, Stderr: &stderr, Store: store, HTTP: client, Browser: browser, ConfigDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	credentials, err := store.LoadCredentials("default")
	if err != nil {
		t.Fatalf("LoadCredentials() error = %v", err)
	}
	if credentials.RefreshToken != "oauth-refresh" || credentials.DeveloperToken != "oauth-developer" {
		t.Fatalf("stored credentials = %+v", credentials)
	}
	if got := exchangeForm.Get("code"); got != "authorization-code" {
		t.Fatalf("exchange code = %q", got)
	}
	if got := exchangeForm.Get("grant_type"); got != "authorization_code" {
		t.Fatalf("grant_type = %q", got)
	}
	if !strings.Contains(stdout.String(), `"connected": true`) || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestGAQLValidationFieldsAndPrecision(t *testing.T) {
	t.Run("validation sends validateOnly", func(t *testing.T) {
		store := authenticatedStore(config.Profile{CustomerID: "1234567890", APIVersion: "v25", Output: "json"})
		client := tokenThenAPI(t, func(request *http.Request, body []byte) *http.Response {
			payload := decodeObject(t, body)
			if payload["query"] != "SELECT campaign.id FROM campaign" {
				t.Errorf("query = %v", payload["query"])
			}
			if payload["validateOnly"] != true {
				t.Errorf("validateOnly = %v, want true", payload["validateOnly"])
			}
			return jsonResponse(http.StatusOK, "gaql-request", `{}`)
		})
		result := runCLI(t, store, client, "query", "validate", "--query", "SELECT campaign.id FROM campaign", "--output", "json")
		if result.err != nil {
			t.Fatalf("Run() error = %v", result.err)
		}
		if !strings.Contains(result.stdout.String(), `"request_id": "gaql-request"`) || !strings.Contains(result.stdout.String(), `"valid": true`) {
			t.Fatalf("stdout = %s", result.stdout.String())
		}
	})

	t.Run("field lookup escapes quoted input", func(t *testing.T) {
		store := authenticatedStore(config.Profile{APIVersion: "v25", Output: "json"})
		client := tokenThenAPI(t, func(request *http.Request, body []byte) *http.Response {
			if got, want := request.URL.Path, "/v25/googleAdsFields:search"; got != want {
				t.Errorf("path = %q, want %q", got, want)
			}
			query, _ := decodeObject(t, body)["query"].(string)
			if !strings.Contains(query, `name = 'campaign.name\'s'`) {
				t.Errorf("field query did not escape quote: %q", query)
			}
			return jsonResponse(http.StatusOK, "fields-request", `{"googleAdsFields":[{"name":"campaign.name","selectable":true}]}`)
		})
		result := runCLI(t, store, client, "fields", "show", "--name", "campaign.name's", "--output", "json")
		if result.err != nil {
			t.Fatalf("Run() error = %v", result.err)
		}
		if !strings.Contains(result.stdout.String(), "campaign.name") {
			t.Fatalf("stdout = %s", result.stdout.String())
		}
	})

	t.Run("large numeric values retain precision", func(t *testing.T) {
		store := authenticatedStore(config.Profile{CustomerID: "1234567890", APIVersion: "v25", Output: "json"})
		client := tokenThenAPI(t, func(*http.Request, []byte) *http.Response {
			return jsonResponse(http.StatusOK, "precision-request", `{"results":[{"metrics":{"costMicros":900719925474099312345}}]}`)
		})
		result := runCLI(t, store, client, "query", "run", "--query", "SELECT metrics.cost_micros FROM campaign", "--output", "json")
		if result.err != nil {
			t.Fatalf("Run() error = %v", result.err)
		}
		if !strings.Contains(result.stdout.String(), "900719925474099312345") {
			t.Fatalf("numeric precision was lost: %s", result.stdout.String())
		}
	})
}

func TestPaginationAndStreamingAreBounded(t *testing.T) {
	t.Run("pagination passes tokens and stops at max rows", func(t *testing.T) {
		store := authenticatedStore(config.Profile{CustomerID: "1234567890", APIVersion: "v25", Output: "json"})
		var requests int
		client := tokenThenAPI(t, func(_ *http.Request, body []byte) *http.Response {
			requests++
			payload := decodeObject(t, body)
			switch requests {
			case 1:
				if _, ok := payload["pageToken"]; ok {
					t.Errorf("first request unexpectedly has pageToken")
				}
				return jsonResponse(http.StatusOK, "page-1", `{"results":[{"id":"1"},{"id":"2"}],"nextPageToken":"next","totalResultsCount":"4"}`)
			case 2:
				if payload["pageToken"] != "next" {
					t.Errorf("pageToken = %v, want next", payload["pageToken"])
				}
				return jsonResponse(http.StatusOK, "page-2", `{"results":[{"id":"3"},{"id":"4"}],"nextPageToken":"unused"}`)
			default:
				t.Fatalf("unexpected request %d", requests)
				return nil
			}
		})
		result := runCLI(t, store, client, "query", "run", "--query", "SELECT customer.id FROM customer", "--max-pages", "9", "--max-rows", "3", "--return-total", "--output", "json")
		if result.err != nil {
			t.Fatalf("Run() error = %v", result.err)
		}
		if requests != 2 {
			t.Fatalf("requests = %d, want 2", requests)
		}
		output := result.stdout.String()
		if !strings.Contains(output, `"id": "3"`) || strings.Contains(output, `"id": "4"`) {
			t.Fatalf("bounded output = %s", output)
		}
	})

	t.Run("streaming merges chunks and stops at max rows", func(t *testing.T) {
		store := authenticatedStore(config.Profile{CustomerID: "1234567890", APIVersion: "v25", Output: "json"})
		client := tokenThenAPI(t, func(request *http.Request, _ []byte) *http.Response {
			if got, want := request.URL.Path, "/v25/customers/1234567890/googleAds:searchStream"; got != want {
				t.Errorf("path = %q, want %q", got, want)
			}
			return jsonResponse(http.StatusOK, "stream-request", `[{"results":[{"id":"1"},{"id":"2"}]},{"results":[{"id":"3"}],"summaryRow":{"metrics":{"clicks":"7"}}}]`)
		})
		result := runCLI(t, store, client, "query", "run", "--query", "SELECT customer.id FROM customer", "--stream", "--max-rows", "2", "--output", "json")
		if result.err != nil {
			t.Fatalf("Run() error = %v", result.err)
		}
		if strings.Contains(result.stdout.String(), `"id": "3"`) {
			t.Fatalf("stream exceeded max rows: %s", result.stdout.String())
		}
	})
}

func TestCanceledRequestReturnsSignalExitCode(t *testing.T) {
	store := authenticatedStore(config.Profile{CustomerID: "1234567890", APIVersion: "v25", Output: "json"})
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, request.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	err := Run(ctx, []string{"query", "run", "--query", "SELECT customer.id FROM customer", "--output", "json"}, Dependencies{
		Stdout: &stdout, Stderr: &stderr, Store: store, HTTP: client, ConfigDir: t.TempDir(),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if got := ExitCode(err); got != 130 {
		t.Fatalf("ExitCode() = %d, want 130", got)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("canceled request wrote stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestCampaignSafetyAndActivation(t *testing.T) {
	store := newMemoryStore()
	store.profiles["default"] = config.Profile{CustomerID: "1234567890", APIVersion: "v25", Output: "json"}

	t.Run("create always plans PAUSED without network", func(t *testing.T) {
		result := runCLI(t, store, nil, "campaign", "create", "--body", `{"name":"Safe launch"}`, "--output", "json")
		if result.err != nil {
			t.Fatalf("Run() error = %v", result.err)
		}
		created := decodePlan(t, result.stdout.Bytes())
		operation := created.Operations[0].(map[string]any)["campaignOperation"].(map[string]any)
		body := operation["create"].(map[string]any)
		if body["status"] != "PAUSED" {
			t.Fatalf("campaign status = %v, want PAUSED", body["status"])
		}
		if err := created.Verify(); err != nil {
			t.Fatalf("plan verification failed: %v", err)
		}
	})

	t.Run("enabled create is rejected", func(t *testing.T) {
		result := runCLI(t, store, nil, "campaign", "create", "--body", `{"name":"Unsafe","status":"ENABLED"}`)
		if ExitCode(result.err) != 2 || !strings.Contains(result.err.Error(), "cannot be enabled") {
			t.Fatalf("error = %v, exit = %d", result.err, ExitCode(result.err))
		}
		if result.stdout.Len() != 0 {
			t.Fatalf("unsafe create wrote stdout: %s", result.stdout.String())
		}
	})

	t.Run("activation is a separate status-only plan", func(t *testing.T) {
		result := runCLI(t, store, nil, "campaign", "activate", "--resource-name", "customers/1234567890/campaigns/42", "--output", "json")
		if result.err != nil {
			t.Fatalf("Run() error = %v", result.err)
		}
		activated := decodePlan(t, result.stdout.Bytes())
		operation := activated.Operations[0].(map[string]any)["campaignOperation"].(map[string]any)
		if operation["updateMask"] != "status" {
			t.Fatalf("updateMask = %v", operation["updateMask"])
		}
		update := operation["update"].(map[string]any)
		if update["status"] != "ENABLED" {
			t.Fatalf("activation status = %v", update["status"])
		}
	})
}

func TestPlanValidateExecuteGatesAndGenericMutations(t *testing.T) {
	configDir := t.TempDir()
	store := authenticatedStore(config.Profile{CustomerID: "1234567890", APIVersion: "v25", Output: "json"})
	operations := `[{"campaignBudgetOperation":{"create":{"name":"Daily","amountMicros":1000000}}}]`
	planPath := filepath.Join(t.TempDir(), "mutation-plan.json")
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"mutate", "plan", "--file", "-", "--plan-out", planPath, "--partial-failure", "--output", "json"}, Dependencies{
		Stdin: strings.NewReader(operations), Stdout: &stdout, Stderr: &stderr, Store: store,
		ConfigDir: configDir, Now: func() time.Time { return fixedNow },
	})
	if err != nil {
		t.Fatalf("plan Run() error = %v", err)
	}
	created, err := plan.Read(planPath)
	if err != nil {
		t.Fatalf("read plan: %v", err)
	}
	if !created.PartialFailure || !strings.HasPrefix(created.Hash, "sha256:") {
		t.Fatalf("created plan = %+v", created)
	}
	if stdout.Len() == 0 || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	t.Run("execute subcommand requires explicit execute flag", func(t *testing.T) {
		result := runCLI(t, store, nil, "mutate", "execute", "--file", planPath, "--output", "json")
		if ExitCode(result.err) != 2 || !strings.Contains(result.err.Error(), "explicit --execute") {
			t.Fatalf("error = %v, exit = %d", result.err, ExitCode(result.err))
		}
	})

	t.Run("validate is replay-safe and sends validateOnly", func(t *testing.T) {
		var calls int
		client := tokenThenAPI(t, func(request *http.Request, body []byte) *http.Response {
			calls++
			if got, want := request.URL.Path, "/v25/customers/1234567890/googleAds:mutate"; got != want {
				t.Errorf("path = %q, want %q", got, want)
			}
			payload := decodeObject(t, body)
			if payload["validateOnly"] != true || payload["partialFailure"] != true {
				t.Errorf("validation payload = %v", payload)
			}
			return jsonResponse(http.StatusOK, "validate-request", `{}`)
		})
		result := runCLI(t, store, client, "mutate", "validate", "--file", planPath, "--output", "json")
		if result.err != nil {
			t.Fatalf("Run() error = %v", result.err)
		}
		if calls != 1 || !strings.Contains(result.stdout.String(), `"valid": true`) || !strings.Contains(result.stdout.String(), created.Hash) {
			t.Fatalf("calls=%d stdout=%s", calls, result.stdout.String())
		}
	})

	t.Run("execute sends the verified generic operation", func(t *testing.T) {
		var calls int
		client := tokenThenAPI(t, func(_ *http.Request, body []byte) *http.Response {
			calls++
			payload := decodeObject(t, body)
			if payload["validateOnly"] != false {
				t.Errorf("validateOnly = %v, want false", payload["validateOnly"])
			}
			operations, _ := payload["mutateOperations"].([]any)
			if len(operations) != 1 {
				t.Fatalf("mutateOperations = %v", payload["mutateOperations"])
			}
			return jsonResponse(http.StatusOK, "execute-request", `{"mutateOperationResponses":[{"campaignBudgetResult":{"resourceName":"customers/1234567890/campaignBudgets/7"}}]}`)
		})
		result := runCLI(t, store, client, "mutate", "execute", "--file", planPath, "--execute", "--output", "json")
		if result.err != nil {
			t.Fatalf("Run() error = %v", result.err)
		}
		if calls != 1 || !strings.Contains(result.stdout.String(), "campaignBudgets/7") {
			t.Fatalf("calls=%d stdout=%s", calls, result.stdout.String())
		}
	})

	t.Run("validate and execute are mutually exclusive on typed writes", func(t *testing.T) {
		result := runCLI(t, store, nil, "budget", "create", "--body", `{"name":"Daily","amountMicros":1000000}`, "--validate", "--execute")
		if ExitCode(result.err) != 2 || !strings.Contains(result.err.Error(), "choose either") {
			t.Fatalf("error = %v, exit = %d", result.err, ExitCode(result.err))
		}
	})

	t.Run("partial failure rejects temporary resource dependencies", func(t *testing.T) {
		input := `[{"campaignOperation":{"create":{"resourceName":"customers/1234567890/campaigns/-1"}}}]`
		var out, errOut bytes.Buffer
		err := Run(context.Background(), []string{"mutate", "plan", "--file", "-", "--partial-failure"}, Dependencies{
			Stdin: strings.NewReader(input), Stdout: &out, Stderr: &errOut, Store: store, ConfigDir: t.TempDir(),
		})
		if ExitCode(err) != 2 || !strings.Contains(err.Error(), "temporary resource dependencies") {
			t.Fatalf("error = %v, exit = %d", err, ExitCode(err))
		}
	})
}

func TestAuditLogIsSanitizedAndStdoutRemainsClean(t *testing.T) {
	configDir := t.TempDir()
	store := authenticatedStore(config.Profile{CustomerID: "1234567890", APIVersion: "v25", Output: "json"})
	mutationPlan, err := plan.New("v25", "1234567890", false, []any{
		map[string]any{"campaignOperation": map[string]any{"remove": "customers/1234567890/campaigns/42"}},
	}, fixedNow)
	if err != nil {
		t.Fatalf("plan.New() error = %v", err)
	}
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := plan.Write(planPath, mutationPlan); err != nil {
		t.Fatalf("plan.Write() error = %v", err)
	}
	secretMessage := "Bearer refresh-token developer-token: developer-token client_secret=client-secret"
	client := tokenThenAPI(t, func(*http.Request, []byte) *http.Response {
		body := `{"error":{"status":"INVALID_ARGUMENT","message":` + string(mustJSON(t, secretMessage)) + `,"details":[{"requestId":"error-request","errors":[]}]}}`
		return jsonResponse(http.StatusBadRequest, "error-request", body)
	})
	var stdout, stderr bytes.Buffer
	err = Run(context.Background(), []string{"mutate", "execute", "--file", planPath, "--execute", "--output", "json"}, Dependencies{
		Stdout: &stdout, Stderr: &stderr, Store: store, HTTP: client, ConfigDir: configDir,
		Now: func() time.Time { return fixedNow },
	})
	if err == nil {
		t.Fatal("Run() error = nil, want API error")
	}
	if ExitCode(err) != 4 {
		t.Fatalf("ExitCode(%v) = %d, want 4", err, ExitCode(err))
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("failed mutation polluted output: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	auditBytes, readErr := os.ReadFile(filepath.Join(configDir, "audit.jsonl"))
	if readErr != nil {
		t.Fatalf("read audit: %v", readErr)
	}
	auditText := string(auditBytes)
	for _, secret := range []string{"refresh-token", "developer-token", "client-secret"} {
		if strings.Contains(auditText, secret) {
			t.Errorf("audit contains secret %q: %s", secret, auditText)
		}
	}
	if !strings.Contains(auditText, "[REDACTED]") || !strings.Contains(auditText, `"request_id":"error-request"`) {
		t.Fatalf("audit was not useful and sanitized: %s", auditText)
	}
	info, err := os.Stat(filepath.Join(configDir, "audit.jsonl"))
	if err != nil {
		t.Fatalf("stat audit: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("audit permissions = %04o, want 0600", got)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}
