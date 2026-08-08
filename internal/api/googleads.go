package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type SearchOptions struct {
	CustomerID         string
	Query              string
	PageToken          string
	ValidateOnly       bool
	ReturnTotalResults bool
}

func (c *Client) Search(ctx context.Context, options SearchOptions) (Response, error) {
	if options.CustomerID == "" {
		return Response{}, errors.New("customer ID is required")
	}
	body := map[string]any{"query": options.Query}
	if options.PageToken != "" {
		body["pageToken"] = options.PageToken
	}
	if options.ValidateOnly {
		body["validateOnly"] = true
	}
	if options.ReturnTotalResults {
		body["returnTotalResults"] = true
	}
	return c.Do(ctx, Request{Method: http.MethodPost, Path: fmt.Sprintf("customers/%s/googleAds:search", options.CustomerID), Body: body, ReplaySafe: true})
}

func (c *Client) SearchStream(ctx context.Context, customerID, query string) (Response, error) {
	if customerID == "" {
		return Response{}, errors.New("customer ID is required")
	}
	return c.Do(ctx, Request{Method: http.MethodPost, Path: fmt.Sprintf("customers/%s/googleAds:searchStream", customerID), Body: map[string]any{"query": query}, ReplaySafe: true})
}

func (c *Client) SearchStreamBounded(ctx context.Context, customerID, query string, maxRows int) (Response, error) {
	if customerID == "" {
		return Response{}, errors.New("customer ID is required")
	}
	if maxRows < 1 {
		return Response{}, errors.New("max rows must be positive")
	}
	if c.Tokens == nil {
		return Response{}, errors.New("OAuth token provider is not configured")
	}
	token, err := c.Tokens.AccessToken(ctx)
	if err != nil {
		return Response{}, err
	}
	body, err := json.Marshal(map[string]any{"query": query})
	if err != nil {
		return Response{}, err
	}
	endpoint := strings.TrimRight(c.Endpoint, "/")
	if endpoint == "" {
		endpoint = "https://googleads.googleapis.com"
	}
	version := strings.Trim(c.Version, "/")
	if version == "" {
		version = "v25"
	}
	target := fmt.Sprintf("%s/%s/customers/%s/googleAds:searchStream", endpoint, version, customerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("developer-token", c.DeveloperToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if c.LoginCustomerID != "" {
		req.Header.Set("login-customer-id", c.LoginCustomerID)
	}
	if c.LinkedCustomerID != "" {
		req.Header.Set("linked-customer-id", c.LinkedCustomerID)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
		return Response{}, fmt.Errorf("Google Ads stream failed: %w", err)
	}
	defer resp.Body.Close()
	requestID := responseRequestID(resp.Header)
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 128<<20))
	decoder.UseNumber()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var value any
		if err := decoder.Decode(&value); err != nil {
			return Response{StatusCode: resp.StatusCode, RequestID: requestID}, err
		}
		return Response{StatusCode: resp.StatusCode, RequestID: requestID, Value: value}, ParseError(resp.StatusCode, requestID, value)
	}
	tokenValue, err := decoder.Token()
	if err != nil {
		return Response{}, fmt.Errorf("decode Google Ads stream: %w", err)
	}
	if delimiter, ok := tokenValue.(json.Delim); !ok || delimiter != '[' {
		return Response{}, errors.New("Google Ads stream response is not a JSON array")
	}
	result := map[string]any{"results": []any{}}
	for decoder.More() {
		var batch map[string]any
		if err := decoder.Decode(&batch); err != nil {
			return Response{}, fmt.Errorf("decode Google Ads stream batch: %w", err)
		}
		if id, ok := batch["requestId"].(string); ok {
			requestID = id
		}
		rows, _ := batch["results"].([]any)
		current := result["results"].([]any)
		remaining := maxRows - len(current)
		if len(rows) > remaining {
			rows = rows[:remaining]
		}
		result["results"] = append(current, rows...)
		for _, key := range []string{"fieldMask", "summaryRow", "queryResourceConsumption", "metricAttributes"} {
			if value, ok := batch[key]; ok {
				result[key] = value
			}
		}
		if len(result["results"].([]any)) >= maxRows {
			return Response{StatusCode: resp.StatusCode, RequestID: requestID, Value: result}, nil
		}
	}
	return Response{StatusCode: resp.StatusCode, RequestID: requestID, Value: result}, nil
}

func (c *Client) SearchFields(ctx context.Context, query, pageToken string, pageSize int) (Response, error) {
	body := map[string]any{"query": query}
	if pageToken != "" {
		body["pageToken"] = pageToken
	}
	if pageSize > 0 {
		body["pageSize"] = pageSize
	}
	return c.Do(ctx, Request{Method: http.MethodPost, Path: "googleAdsFields:search", Body: body, ReplaySafe: true})
}

func (c *Client) ListAccessibleCustomers(ctx context.Context) (Response, error) {
	return c.Do(ctx, Request{Method: http.MethodGet, Path: "customers:listAccessibleCustomers", ReplaySafe: true})
}

func (c *Client) Mutate(ctx context.Context, customerID string, operations []any, partialFailure, validateOnly bool) (Response, error) {
	if customerID == "" {
		return Response{}, errors.New("customer ID is required")
	}
	body := map[string]any{
		"mutateOperations":    operations,
		"partialFailure":      partialFailure,
		"validateOnly":        validateOnly,
		"responseContentType": "MUTABLE_RESOURCE",
	}
	return c.Do(ctx, Request{Method: http.MethodPost, Path: fmt.Sprintf("customers/%s/googleAds:mutate", customerID), Body: body, ReplaySafe: validateOnly})
}
