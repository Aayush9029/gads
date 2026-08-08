package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Aayush9029/gads/internal/config"
)

func TestTokenManagerRefreshesAndCachesToken(t *testing.T) {
	t.Parallel()

	clock := &fakeClock{now: time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestNumber := requests.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		wantForm := url.Values{
			"client_id":     {"client"},
			"client_secret": {"secret"},
			"refresh_token": {"refresh"},
			"grant_type":    {"refresh_token"},
		}
		if r.Form.Encode() != wantForm.Encode() {
			t.Errorf("form = %q, want %q", r.Form.Encode(), wantForm.Encode())
		}
		_, _ = fmt.Fprintf(w, `{"access_token":"access-%d","expires_in":3600}`, requestNumber)
	}))
	defer server.Close()

	credentials := config.Credentials{
		ClientID:       "client",
		ClientSecret:   "secret",
		RefreshToken:   "refresh",
		DeveloperToken: "not-sent",
	}
	manager := &TokenManager{
		HTTP: server.Client(), Credentials: &credentials, Clock: clock, TokenURL: server.URL,
	}

	token, err := manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("first AccessToken: %v", err)
	}
	if token != "access-1" {
		t.Fatalf("first token = %q", token)
	}
	clock.Set(clock.Now().Add(54 * time.Minute))
	token, err = manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("cached AccessToken: %v", err)
	}
	if token != "access-1" || requests.Load() != 1 {
		t.Fatalf("cached token = %q, requests = %d", token, requests.Load())
	}

	clock.Set(clock.Now().Add(time.Minute))
	token, err = manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("refreshed AccessToken: %v", err)
	}
	if token != "access-2" || requests.Load() != 2 {
		t.Fatalf("refreshed token = %q, requests = %d", token, requests.Load())
	}
}

func TestTokenManagerSerializesConcurrentRefreshes(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, `{"access_token":"shared","expires_in":3600}`)
	}))
	defer server.Close()

	credentials := config.Credentials{ClientID: "client", ClientSecret: "secret", RefreshToken: "refresh"}
	manager := &TokenManager{HTTP: server.Client(), Credentials: &credentials, TokenURL: server.URL}
	const callers = 24
	start := make(chan struct{})
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			token, err := manager.AccessToken(context.Background())
			if err == nil && token != "shared" {
				err = fmt.Errorf("token = %q", token)
			}
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("AccessToken: %v", err)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("refresh requests = %d, want 1", got)
	}
}

func TestTokenManagerCredentialPrecedence(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		credentials: map[string]config.Credentials{
			"work": {ClientID: "stored-client", ClientSecret: "stored-secret", RefreshToken: "stored-refresh"},
		},
	}
	t.Run("explicit credentials bypass store", func(t *testing.T) {
		var got url.Values
		server := tokenServer(t, func(form url.Values) { got = form }, `{"access_token":"token","expires_in":3600}`, http.StatusOK)
		defer server.Close()
		explicit := config.Credentials{ClientID: "explicit-client", ClientSecret: "explicit-secret", RefreshToken: "explicit-refresh"}
		manager := TokenManager{HTTP: server.Client(), Store: store, Profile: "work", Credentials: &explicit, TokenURL: server.URL}
		if _, err := manager.AccessToken(context.Background()); err != nil {
			t.Fatalf("AccessToken: %v", err)
		}
		if got.Get("client_id") != "explicit-client" || got.Get("refresh_token") != "explicit-refresh" {
			t.Fatalf("form used wrong credentials: %v", got)
		}
		if store.LoadCount() != 0 {
			t.Fatalf("store loads = %d, want 0", store.LoadCount())
		}
	})

	t.Run("store selected by profile", func(t *testing.T) {
		store.ResetLoadCount()
		var got url.Values
		server := tokenServer(t, func(form url.Values) { got = form }, `{"access_token":"token","expires_in":3600}`, http.StatusOK)
		defer server.Close()
		manager := TokenManager{HTTP: server.Client(), Store: store, Profile: "work", TokenURL: server.URL}
		if _, err := manager.AccessToken(context.Background()); err != nil {
			t.Fatalf("AccessToken: %v", err)
		}
		if got.Get("client_id") != "stored-client" || got.Get("refresh_token") != "stored-refresh" {
			t.Fatalf("form used wrong credentials: %v", got)
		}
		if store.LoadCount() != 1 {
			t.Fatalf("store loads = %d, want 1", store.LoadCount())
		}
	})
}

func TestTokenManagerConfigurationFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		manager *TokenManager
		want    string
	}{
		{name: "no store", manager: &TokenManager{}, want: "credential store is not configured"},
		{
			name:    "store failure",
			manager: &TokenManager{Store: &memoryStore{loadErr: errors.New("disk unavailable")}, Profile: "work"},
			want:    "load OAuth credentials: disk unavailable",
		},
		{
			name:    "no refresh token",
			manager: &TokenManager{Credentials: &config.Credentials{ClientID: "client"}},
			want:    "no refresh token",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.manager.AccessToken(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("AccessToken error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestTokenManagerOAuthFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		body       string
		want       string
		wantDecode bool
	}{
		{name: "provider error", status: http.StatusBadRequest, body: `{"error":"invalid_grant","error_description":"refresh token revoked"}`, want: "OAuth refresh failed: invalid_grant: refresh token revoked"},
		{name: "error without description", status: http.StatusUnauthorized, body: `{"error":"invalid_client"}`, want: "OAuth refresh failed: invalid_client"},
		{name: "empty success", status: http.StatusOK, body: `{}`, want: "OAuth refresh failed: 200 OK"},
		{name: "malformed response", status: http.StatusBadGateway, body: `not-json`, want: "decode OAuth response", wantDecode: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tokenServer(t, nil, tt.body, tt.status)
			defer server.Close()
			credentials := config.Credentials{RefreshToken: "refresh"}
			manager := TokenManager{HTTP: server.Client(), Credentials: &credentials, TokenURL: server.URL}
			_, err := manager.AccessToken(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("AccessToken error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestTokenManagerCancellation(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		close(started)
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	credentials := config.Credentials{RefreshToken: "refresh"}
	manager := TokenManager{HTTP: client, Credentials: &credentials, TokenURL: "https://oauth.invalid/token"}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := manager.AccessToken(ctx)
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "refresh OAuth token") {
			t.Fatalf("AccessToken error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("AccessToken did not stop after cancellation")
	}
}

func TestTokenManagerDoesNotCacheImmediatelyExpiringTokens(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := requests.Add(1)
		_, _ = fmt.Fprintf(w, `{"access_token":"token-%d","expires_in":0}`, n)
	}))
	defer server.Close()
	credentials := config.Credentials{RefreshToken: "refresh"}
	manager := TokenManager{HTTP: server.Client(), Credentials: &credentials, TokenURL: server.URL}
	first, err := manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("first AccessToken: %v", err)
	}
	second, err := manager.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("second AccessToken: %v", err)
	}
	if first != "token-1" || second != "token-2" || requests.Load() != 2 {
		t.Fatalf("tokens = %q, %q; requests = %d", first, second, requests.Load())
	}
}

func tokenServer(t *testing.T, inspect func(url.Values), body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		if inspect != nil {
			inspect(r.Form)
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Set(now time.Time) {
	c.mu.Lock()
	c.now = now
	c.mu.Unlock()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type memoryStore struct {
	mu          sync.Mutex
	current     string
	profiles    map[string]config.Profile
	credentials map[string]config.Credentials
	loadErr     error
	saveErr     error
	loadCount   int
}

func (s *memoryStore) CurrentProfile() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current, nil
}

func (s *memoryStore) SetCurrentProfile(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = name
	return nil
}

func (s *memoryStore) ListProfiles() (map[string]config.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.profiles, nil
}

func (s *memoryStore) LoadProfile(name string) (config.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile, ok := s.profiles[name]
	if !ok {
		return config.Profile{}, errors.New("profile not found")
	}
	return profile, nil
}

func (s *memoryStore) SaveProfile(name string, profile config.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.profiles == nil {
		s.profiles = map[string]config.Profile{}
	}
	s.profiles[name] = profile
	return nil
}

func (s *memoryStore) LoadCredentials(name string) (config.Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadCount++
	if s.loadErr != nil {
		return config.Credentials{}, s.loadErr
	}
	credentials, ok := s.credentials[name]
	if !ok {
		return config.Credentials{}, errors.New("credentials not found")
	}
	return credentials, nil
}

func (s *memoryStore) SaveCredentials(name string, credentials config.Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return s.saveErr
	}
	if s.credentials == nil {
		s.credentials = map[string]config.Credentials{}
	}
	s.credentials[name] = credentials
	return nil
}

func (s *memoryStore) DeleteCredentials(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.credentials, name)
	return nil
}

func (s *memoryStore) LoadCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadCount
}

func (s *memoryStore) ResetLoadCount() {
	s.mu.Lock()
	s.loadCount = 0
	s.mu.Unlock()
}
