package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Aayush9029/gads/internal/config"
)

const (
	AuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	TokenURL     = "https://oauth2.googleapis.com/token"
	AdsScope     = "https://www.googleapis.com/auth/adwords"
)

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type TokenManager struct {
	HTTP        *http.Client
	Store       config.Store
	Profile     string
	Credentials *config.Credentials
	Clock       Clock
	TokenURL    string
	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

func (m *TokenManager) AccessToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	clock := m.Clock
	if clock == nil {
		clock = realClock{}
	}
	if m.accessToken != "" && clock.Now().Add(5*time.Minute).Before(m.expiresAt) {
		return m.accessToken, nil
	}
	var credentials config.Credentials
	if m.Credentials != nil {
		credentials = *m.Credentials
	} else {
		if m.Store == nil {
			return "", errors.New("credential store is not configured")
		}
		var err error
		credentials, err = m.Store.LoadCredentials(m.Profile)
		if err != nil {
			return "", fmt.Errorf("load OAuth credentials: %w", err)
		}
	}
	if credentials.RefreshToken == "" {
		return "", errors.New("no refresh token; run gads auth login")
	}
	endpoint := m.TokenURL
	if endpoint == "" {
		endpoint = TokenURL
	}
	form := url.Values{
		"client_id":     {credentials.ClientID},
		"client_secret": {credentials.ClientSecret},
		"refresh_token": {credentials.RefreshToken},
		"grant_type":    {"refresh_token"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := m.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("refresh OAuth token: %w", err)
	}
	defer resp.Body.Close()
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode OAuth response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || body.AccessToken == "" {
		message := body.Error
		if body.Description != "" {
			message += ": " + body.Description
		}
		if message == "" {
			message = resp.Status
		}
		return "", fmt.Errorf("OAuth refresh failed: %s", message)
	}
	m.accessToken = body.AccessToken
	m.expiresAt = clock.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
	return m.accessToken, nil
}
