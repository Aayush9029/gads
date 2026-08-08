package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/Aayush9029/gads/internal/config"
)

type Browser func(context.Context, string) error
type ListenFunc func(network, address string) (net.Listener, error)

type LoginFlow struct {
	HTTP     *http.Client
	Store    config.Store
	Browser  Browser
	Listen   ListenFunc
	Random   io.Reader
	AuthURL  string
	TokenURL string
	Timeout  time.Duration
}

type LoginOptions struct {
	Profile        string
	ClientID       string
	ClientSecret   string
	DeveloperToken string
}

func OpenBrowser(ctx context.Context, target string) error {
	return exec.CommandContext(ctx, "open", target).Start()
}

func (f LoginFlow) Login(ctx context.Context, options LoginOptions) error {
	if options.ClientID == "" || options.ClientSecret == "" || options.DeveloperToken == "" {
		return errors.New("client ID, client secret, and developer token are required")
	}
	listen := f.Listen
	if listen == nil {
		listen = net.Listen
	}
	listener, err := listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start OAuth callback: %w", err)
	}
	defer listener.Close()
	redirectURI := "http://" + listener.Addr().String() + "/oauth/callback"
	state, err := randomState(f.Random)
	if err != nil {
		return err
	}
	authURL := f.AuthURL
	if authURL == "" {
		authURL = AuthorizeURL
	}
	query := url.Values{
		"client_id":     {options.ClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {AdsScope},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
		"state":         {state},
	}
	result := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "Invalid OAuth state. Return to the terminal and try again.", http.StatusBadRequest)
			result <- callbackResult{err: errors.New("OAuth state mismatch")}
			return
		}
		if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
			http.Error(w, "Authorization was not completed. You can close this window.", http.StatusBadRequest)
			result <- callbackResult{err: fmt.Errorf("OAuth authorization failed: %s", oauthErr)}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "The callback did not include an authorization code.", http.StatusBadRequest)
			result <- callbackResult{err: errors.New("OAuth callback did not include a code")}
			return
		}
		_, _ = io.WriteString(w, "gads is connected. You can close this window.\n")
		result <- callbackResult{code: code}
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer server.Shutdown(context.Background())
	browser := f.Browser
	if browser == nil {
		browser = OpenBrowser
	}
	if err := browser(ctx, authURL+"?"+query.Encode()); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	timeout := f.Timeout
	if timeout == 0 {
		timeout = 3 * time.Minute
	}
	var callback callbackResult
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeout):
		return errors.New("OAuth login timed out")
	case callback = <-result:
		if callback.err != nil {
			return callback.err
		}
	}
	refreshToken, err := f.exchange(ctx, options, callback.code, redirectURI)
	if err != nil {
		return err
	}
	return f.Store.SaveCredentials(options.Profile, config.Credentials{
		ClientID: options.ClientID, ClientSecret: options.ClientSecret,
		DeveloperToken: options.DeveloperToken, RefreshToken: refreshToken,
	})
}

type callbackResult struct {
	code string
	err  error
}

func (f LoginFlow) exchange(ctx context.Context, options LoginOptions, code, redirectURI string) (string, error) {
	endpoint := f.TokenURL
	if endpoint == "" {
		endpoint = TokenURL
	}
	form := url.Values{
		"client_id": {options.ClientID}, "client_secret": {options.ClientSecret},
		"code": {code}, "redirect_uri": {redirectURI}, "grant_type": {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := f.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("exchange OAuth code: %w", err)
	}
	defer resp.Body.Close()
	var body struct {
		RefreshToken string `json:"refresh_token"`
		Error        string `json:"error"`
		Description  string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode OAuth exchange: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || body.RefreshToken == "" {
		message := strings.Trim(strings.TrimSpace(body.Error)+": "+strings.TrimSpace(body.Description), ": ")
		if message == "" {
			message = resp.Status
		}
		return "", fmt.Errorf("OAuth exchange failed: %s", message)
	}
	return body.RefreshToken, nil
}

func randomState(source io.Reader) (string, error) {
	if source == nil {
		source = rand.Reader
	}
	b := make([]byte, 32)
	if _, err := io.ReadFull(source, b); err != nil {
		return "", fmt.Errorf("create OAuth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
