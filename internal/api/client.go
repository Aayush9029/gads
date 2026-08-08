package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TokenProvider interface {
	AccessToken(context.Context) (string, error)
}

type Sleeper func(context.Context, time.Duration) error

type Client struct {
	HTTP             *http.Client
	Tokens           TokenProvider
	Endpoint         string
	Version          string
	DeveloperToken   string
	LoginCustomerID  string
	LinkedCustomerID string
	MaxAttempts      int
	Sleep            Sleeper
	Jitter           func(time.Duration) time.Duration
}

type Request struct {
	Method     string
	Path       string
	Query      url.Values
	Body       any
	ReplaySafe bool
}

type Response struct {
	StatusCode int
	RequestID  string
	Value      any
}

func (c *Client) Do(ctx context.Context, request Request) (Response, error) {
	body, err := marshalBody(request.Body)
	if err != nil {
		return Response{}, err
	}
	attempts := c.MaxAttempts
	if attempts < 1 {
		attempts = 3
	}
	if !request.ReplaySafe {
		attempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		response, retry, err := c.doOnce(ctx, request, body)
		if err == nil || !retry || attempt == attempts-1 {
			return response, err
		}
		lastErr = err
		delay := time.Duration(1<<attempt) * 250 * time.Millisecond
		if c.Jitter != nil {
			delay = c.Jitter(delay)
		} else {
			delay += time.Duration(rand.Int64N(int64(delay/2 + 1)))
		}
		if err := c.sleep(ctx, delay); err != nil {
			return Response{}, err
		}
	}
	return Response{}, lastErr
}

func (c *Client) doOnce(ctx context.Context, request Request, body []byte) (Response, bool, error) {
	if c.Tokens == nil {
		return Response{}, false, errors.New("OAuth token provider is not configured")
	}
	token, err := c.Tokens.AccessToken(ctx)
	if err != nil {
		return Response{}, false, err
	}
	endpoint := strings.TrimRight(c.Endpoint, "/")
	if endpoint == "" {
		endpoint = "https://googleads.googleapis.com"
	}
	version := strings.Trim(c.Version, "/")
	if version == "" {
		version = "v25"
	}
	target := endpoint + "/" + version + "/" + strings.TrimLeft(request.Path, "/")
	if encoded := request.Query.Encode(); encoded != "" {
		target += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, request.Method, target, bytes.NewReader(body))
	if err != nil {
		return Response{}, false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("developer-token", c.DeveloperToken)
	req.Header.Set("Accept", "application/json")
	if request.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.LoginCustomerID != "" {
		req.Header.Set("login-customer-id", c.LoginCustomerID)
	}
	if c.LinkedCustomerID != "" {
		req.Header.Set("linked-customer-id", c.LinkedCustomerID)
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Response{}, false, ctx.Err()
		}
		return Response{}, true, fmt.Errorf("Google Ads request failed: %w", err)
	}
	defer resp.Body.Close()
	requestID := responseRequestID(resp.Header)
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 128<<20))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil && !errors.Is(err, io.EOF) {
		return Response{StatusCode: resp.StatusCode, RequestID: requestID}, retryableStatus(resp.StatusCode), fmt.Errorf("decode Google Ads response: %w", err)
	}
	result := Response{StatusCode: resp.StatusCode, RequestID: requestID, Value: value}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := ParseError(resp.StatusCode, requestID, value)
		return result, retryableStatus(resp.StatusCode), apiErr
	}
	return result, false, nil
}

func (c *Client) sleep(ctx context.Context, duration time.Duration) error {
	if c.Sleep != nil {
		return c.Sleep(ctx, duration)
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func marshalBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	if raw, ok := body.([]byte); ok {
		if !json.Valid(raw) {
			return nil, errors.New("request body is not valid JSON")
		}
		return raw, nil
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	return encoded, nil
}

func retryableStatus(status int) bool {
	return status == http.StatusInternalServerError || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func responseRequestID(header http.Header) string {
	for _, key := range []string{"request-id", "google-ads-request-id"} {
		if value := header.Get(key); value != "" {
			return value
		}
	}
	return ""
}
