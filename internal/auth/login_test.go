package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Aayush9029/gads/internal/config"
)

func TestLoginFlowCompletesAuthorizationCodeExchange(t *testing.T) {
	t.Parallel()

	store := &memoryStore{}
	stateBytes := bytes.Repeat([]byte{0x5a}, 32)
	wantState := base64.RawURLEncoding.EncodeToString(stateBytes)
	var authRedirectURI string
	var exchangeRequests atomic.Int32
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchangeRequests.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("exchange method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		want := url.Values{
			"client_id":     {"client-id"},
			"client_secret": {"client-secret"},
			"code":          {"authorization-code"},
			"redirect_uri":  {authRedirectURI},
			"grant_type":    {"authorization_code"},
		}
		if r.Form.Encode() != want.Encode() {
			t.Errorf("exchange form = %q, want %q", r.Form.Encode(), want.Encode())
		}
		_, _ = io.WriteString(w, `{"refresh_token":"refresh-token"}`)
	}))
	defer tokenServer.Close()

	var listenCalls atomic.Int32
	flow := LoginFlow{
		HTTP:     tokenServer.Client(),
		Store:    store,
		Random:   bytes.NewReader(stateBytes),
		AuthURL:  "https://accounts.example/authorize",
		TokenURL: tokenServer.URL,
		Timeout:  time.Second,
		Listen: func(network, address string) (net.Listener, error) {
			listenCalls.Add(1)
			if network != "tcp" || address != "127.0.0.1:0" {
				t.Errorf("Listen(%q, %q)", network, address)
			}
			return net.Listen(network, address)
		},
		Browser: func(ctx context.Context, target string) error {
			parsed, err := url.Parse(target)
			if err != nil {
				return err
			}
			if parsed.Scheme+"://"+parsed.Host+parsed.Path != "https://accounts.example/authorize" {
				t.Errorf("authorization endpoint = %q", parsed.Scheme+"://"+parsed.Host+parsed.Path)
			}
			query := parsed.Query()
			if query.Get("client_id") != "client-id" || query.Get("response_type") != "code" {
				t.Errorf("authorization identity query = %v", query)
			}
			if query.Get("scope") != AdsScope || query.Get("access_type") != "offline" || query.Get("prompt") != "consent" {
				t.Errorf("authorization access query = %v", query)
			}
			if query.Get("state") != wantState {
				t.Errorf("state = %q, want %q", query.Get("state"), wantState)
			}
			authRedirectURI = query.Get("redirect_uri")
			if !strings.HasPrefix(authRedirectURI, "http://127.0.0.1:") || !strings.HasSuffix(authRedirectURI, "/oauth/callback") {
				t.Errorf("redirect URI = %q", authRedirectURI)
			}
			callback, err := url.Parse(authRedirectURI)
			if err != nil {
				return err
			}
			callback.RawQuery = url.Values{"state": {query.Get("state")}, "code": {"authorization-code"}}.Encode()
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, callback.String(), nil)
			if err != nil {
				return err
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				return err
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				return err
			}
			if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "gads is connected") {
				t.Errorf("callback response = %s %q", response.Status, body)
			}
			return nil
		},
	}
	options := LoginOptions{
		Profile: "work", ClientID: "client-id", ClientSecret: "client-secret", DeveloperToken: "developer-token",
	}
	if err := flow.Login(context.Background(), options); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if listenCalls.Load() != 1 || exchangeRequests.Load() != 1 {
		t.Fatalf("listener calls = %d, exchange requests = %d", listenCalls.Load(), exchangeRequests.Load())
	}
	got, err := store.LoadCredentials("work")
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	want := config.Credentials{
		ClientID: "client-id", ClientSecret: "client-secret", DeveloperToken: "developer-token", RefreshToken: "refresh-token",
	}
	if got != want {
		t.Fatalf("saved credentials = %#v, want %#v", got, want)
	}
}

func TestLoginFlowValidatesRequiredCredentialsBeforeSideEffects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options LoginOptions
	}{
		{name: "client ID", options: LoginOptions{ClientSecret: "secret", DeveloperToken: "developer"}},
		{name: "client secret", options: LoginOptions{ClientID: "client", DeveloperToken: "developer"}},
		{name: "developer token", options: LoginOptions{ClientID: "client", ClientSecret: "secret"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var listened, browsed bool
			flow := LoginFlow{
				Listen:  func(string, string) (net.Listener, error) { listened = true; return nil, errors.New("unexpected") },
				Browser: func(context.Context, string) error { browsed = true; return nil },
			}
			err := flow.Login(context.Background(), tt.options)
			if err == nil || !strings.Contains(err.Error(), "client ID, client secret, and developer token are required") {
				t.Fatalf("Login error = %v", err)
			}
			if listened || browsed {
				t.Fatalf("invalid input caused side effects: listened=%v browsed=%v", listened, browsed)
			}
		})
	}
}

func TestLoginFlowCallbackFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		callback   func(url.Values) url.Values
		wantStatus int
		want       string
	}{
		{
			name: "state mismatch",
			callback: func(query url.Values) url.Values {
				return url.Values{"state": {query.Get("state") + "wrong"}, "code": {"code"}}
			},
			wantStatus: http.StatusBadRequest,
			want:       "OAuth state mismatch",
		},
		{
			name: "provider denial",
			callback: func(query url.Values) url.Values {
				return url.Values{"state": {query.Get("state")}, "error": {"access_denied"}}
			},
			wantStatus: http.StatusBadRequest,
			want:       "OAuth authorization failed: access_denied",
		},
		{
			name: "missing code",
			callback: func(query url.Values) url.Values {
				return url.Values{"state": {query.Get("state")}}
			},
			wantStatus: http.StatusBadRequest,
			want:       "OAuth callback did not include a code",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var exchanges atomic.Int32
			tokenServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				exchanges.Add(1)
			}))
			defer tokenServer.Close()
			flow := LoginFlow{
				HTTP: tokenServer.Client(), Store: &memoryStore{}, TokenURL: tokenServer.URL,
				Random: bytes.NewReader(bytes.Repeat([]byte{1}, 32)), Timeout: time.Second,
				Browser: callbackBrowser(t, tt.callback, tt.wantStatus),
			}
			err := flow.Login(context.Background(), validLoginOptions())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Login error = %v, want containing %q", err, tt.want)
			}
			if exchanges.Load() != 0 {
				t.Fatalf("token exchange requests = %d, want 0", exchanges.Load())
			}
		})
	}
}

func TestLoginFlowTimeoutAndCancellation(t *testing.T) {
	t.Parallel()

	t.Run("timeout", func(t *testing.T) {
		flow := LoginFlow{
			Store: &memoryStore{}, Random: bytes.NewReader(bytes.Repeat([]byte{1}, 32)), Timeout: 15 * time.Millisecond,
			Browser: func(context.Context, string) error { return nil },
		}
		started := time.Now()
		err := flow.Login(context.Background(), validLoginOptions())
		if err == nil || !strings.Contains(err.Error(), "OAuth login timed out") {
			t.Fatalf("Login error = %v", err)
		}
		if time.Since(started) > time.Second {
			t.Fatalf("Login timeout took too long: %v", time.Since(started))
		}
	})

	t.Run("context cancellation while waiting", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		flow := LoginFlow{
			Store: &memoryStore{}, Random: bytes.NewReader(bytes.Repeat([]byte{1}, 32)), Timeout: time.Second,
			Browser: func(context.Context, string) error { cancel(); return nil },
		}
		err := flow.Login(ctx, validLoginOptions())
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Login error = %v, want context.Canceled", err)
		}
	})

	t.Run("context cancellation during exchange", func(t *testing.T) {
		started := make(chan struct{})
		client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			close(started)
			<-request.Context().Done()
			return nil, request.Context().Err()
		})}
		ctx, cancel := context.WithCancel(context.Background())
		flow := LoginFlow{
			HTTP: client, Store: &memoryStore{}, TokenURL: "https://oauth.invalid/token",
			Random: bytes.NewReader(bytes.Repeat([]byte{1}, 32)), Timeout: time.Second,
			Browser: callbackBrowser(t, func(query url.Values) url.Values {
				return url.Values{"state": {query.Get("state")}, "code": {"code"}}
			}, http.StatusOK),
		}
		done := make(chan error, 1)
		go func() { done <- flow.Login(ctx, validLoginOptions()) }()
		<-started
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "exchange OAuth code") {
				t.Fatalf("Login error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Login exchange did not stop after cancellation")
		}
	})
}

func TestLoginFlowInfrastructureFailures(t *testing.T) {
	t.Parallel()

	t.Run("listener", func(t *testing.T) {
		flow := LoginFlow{Listen: func(string, string) (net.Listener, error) { return nil, errors.New("no sockets") }}
		err := flow.Login(context.Background(), validLoginOptions())
		if err == nil || !strings.Contains(err.Error(), "start OAuth callback: no sockets") {
			t.Fatalf("Login error = %v", err)
		}
	})

	t.Run("random source", func(t *testing.T) {
		var browsed bool
		flow := LoginFlow{Random: errorReader{err: errors.New("entropy unavailable")}, Browser: func(context.Context, string) error {
			browsed = true
			return nil
		}}
		err := flow.Login(context.Background(), validLoginOptions())
		if err == nil || !strings.Contains(err.Error(), "create OAuth state: entropy unavailable") {
			t.Fatalf("Login error = %v", err)
		}
		if browsed {
			t.Fatal("browser opened after random state generation failed")
		}
	})

	t.Run("browser", func(t *testing.T) {
		flow := LoginFlow{
			Random:  bytes.NewReader(bytes.Repeat([]byte{1}, 32)),
			Browser: func(context.Context, string) error { return errors.New("browser unavailable") },
		}
		err := flow.Login(context.Background(), validLoginOptions())
		if err == nil || !strings.Contains(err.Error(), "open browser: browser unavailable") {
			t.Fatalf("Login error = %v", err)
		}
	})

	t.Run("credential save", func(t *testing.T) {
		tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"refresh_token":"refresh"}`)
		}))
		defer tokenServer.Close()
		flow := LoginFlow{
			HTTP: tokenServer.Client(), Store: &memoryStore{saveErr: errors.New("disk full")}, TokenURL: tokenServer.URL,
			Random: bytes.NewReader(bytes.Repeat([]byte{1}, 32)),
			Browser: callbackBrowser(t, func(query url.Values) url.Values {
				return url.Values{"state": {query.Get("state")}, "code": {"code"}}
			}, http.StatusOK),
		}
		err := flow.Login(context.Background(), validLoginOptions())
		if err == nil || !strings.Contains(err.Error(), "disk full") {
			t.Fatalf("Login error = %v", err)
		}
	})
}

func TestLoginFlowExchangeFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{name: "provider error", status: http.StatusBadRequest, body: `{"error":"invalid_grant","error_description":"code expired"}`, want: "OAuth exchange failed: invalid_grant: code expired"},
		{name: "provider error without description", status: http.StatusUnauthorized, body: `{"error":"invalid_client"}`, want: "OAuth exchange failed: invalid_client"},
		{name: "missing refresh token", status: http.StatusOK, body: `{}`, want: "OAuth exchange failed: 200 OK"},
		{name: "malformed response", status: http.StatusBadGateway, body: `not-json`, want: "decode OAuth exchange"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tokenServer(t, nil, tt.body, tt.status)
			defer server.Close()
			flow := LoginFlow{HTTP: server.Client(), TokenURL: server.URL}
			_, err := flow.exchange(context.Background(), validLoginOptions(), "code", "http://localhost/callback")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("exchange error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestRandomState(t *testing.T) {
	t.Parallel()

	source := bytes.Repeat([]byte{0xff}, 32)
	got, err := randomState(bytes.NewReader(source))
	if err != nil {
		t.Fatalf("randomState: %v", err)
	}
	if strings.ContainsAny(got, "+/=") {
		t.Fatalf("randomState returned non-URL-safe encoding %q", got)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if !bytes.Equal(decoded, source) {
		t.Fatalf("decoded state = %x, want %x", decoded, source)
	}
	if _, err := randomState(bytes.NewReader(make([]byte, 31))); err == nil || !strings.Contains(err.Error(), "create OAuth state") {
		t.Fatalf("short random source error = %v", err)
	}
}

func callbackBrowser(t *testing.T, callbackQuery func(url.Values) url.Values, wantStatus int) Browser {
	t.Helper()
	return func(ctx context.Context, target string) error {
		authorizeURL, err := url.Parse(target)
		if err != nil {
			return err
		}
		callbackURL, err := url.Parse(authorizeURL.Query().Get("redirect_uri"))
		if err != nil {
			return err
		}
		callbackURL.RawQuery = callbackQuery(authorizeURL.Query()).Encode()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, callbackURL.String(), nil)
		if err != nil {
			return err
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, response.Body)
		if response.StatusCode != wantStatus {
			t.Errorf("callback status = %d, want %d", response.StatusCode, wantStatus)
		}
		return nil
	}
}

func validLoginOptions() LoginOptions {
	return LoginOptions{Profile: "work", ClientID: "client", ClientSecret: "secret", DeveloperToken: "developer"}
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}
