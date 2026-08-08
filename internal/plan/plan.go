package plan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Plan struct {
	Schema              string `json:"schema"`
	APIVersion          string `json:"api_version"`
	CustomerID          string `json:"customer_id"`
	PartialFailure      bool   `json:"partial_failure"`
	Operations          []any  `json:"operations"`
	Method              string `json:"method,omitempty"`
	Path                string `json:"path,omitempty"`
	Body                any    `json:"body,omitempty"`
	LocalValidationOnly bool   `json:"local_validation_only,omitempty"`
	CreatedAt           string `json:"created_at,omitempty"`
	Hash                string `json:"hash,omitempty"`
}

func NewLocalRequest(apiVersion, customerID, method, path string, body any, now time.Time) (Plan, error) {
	p, err := NewRequest(apiVersion, customerID, method, path, body, now)
	if err != nil {
		return Plan{}, err
	}
	p.LocalValidationOnly = true
	p.Hash, err = p.DeterministicHash()
	return p, err
}

func NewRequest(apiVersion, customerID, method, path string, body any, now time.Time) (Plan, error) {
	if customerID == "" {
		return Plan{}, errors.New("customer ID is required")
	}
	if method == "" || path == "" {
		return Plan{}, errors.New("method and path are required")
	}
	p := Plan{Schema: "gads.mutation-plan/v1", APIVersion: apiVersion, CustomerID: customerID, Method: method, Path: path, Body: body, CreatedAt: now.UTC().Format(time.RFC3339)}
	hash, err := p.DeterministicHash()
	if err != nil {
		return Plan{}, err
	}
	p.Hash = hash
	return p, nil
}

func New(apiVersion, customerID string, partialFailure bool, operations []any, now time.Time) (Plan, error) {
	if customerID == "" {
		return Plan{}, errors.New("customer ID is required")
	}
	if len(operations) == 0 {
		return Plan{}, errors.New("at least one mutation operation is required")
	}
	p := Plan{Schema: "gads.mutation-plan/v1", APIVersion: apiVersion, CustomerID: customerID, PartialFailure: partialFailure, Operations: operations, CreatedAt: now.UTC().Format(time.RFC3339)}
	hash, err := p.DeterministicHash()
	if err != nil {
		return Plan{}, err
	}
	p.Hash = hash
	return p, nil
}

func (p Plan) DeterministicHash() (string, error) {
	canonical := map[string]any{
		"schema": p.Schema, "api_version": p.APIVersion, "customer_id": p.CustomerID,
	}
	if p.Path != "" {
		canonical["method"] = p.Method
		canonical["path"] = p.Path
		canonical["body"] = p.Body
		if p.LocalValidationOnly {
			canonical["local_validation_only"] = true
		}
	} else {
		canonical["partial_failure"] = p.PartialFailure
		canonical["operations"] = p.Operations
	}
	b, err := canonicalJSON(canonical)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func (p Plan) Verify() error {
	if p.Schema != "gads.mutation-plan/v1" {
		return fmt.Errorf("unsupported plan schema %q", p.Schema)
	}
	if (p.Path == "") == (len(p.Operations) == 0) {
		return errors.New("plan must contain either mutate operations or one action request")
	}
	hash, err := p.DeterministicHash()
	if err != nil {
		return err
	}
	if p.Hash != hash {
		return fmt.Errorf("plan hash mismatch: computed %s", hash)
	}
	return nil
}

func Read(path string) (Plan, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Plan{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	var p Plan
	if err := decoder.Decode(&p); err != nil {
		return Plan{}, fmt.Errorf("decode plan: %w", err)
	}
	return p, p.Verify()
}

func Write(path string, p Plan) error {
	if err := p.Verify(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(dir, ".gads-plan-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func canonicalJSON(value any) ([]byte, error) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result := []byte{'{'}
		for i, key := range keys {
			if i > 0 {
				result = append(result, ',')
			}
			encodedKey, _ := json.Marshal(key)
			result = append(result, encodedKey...)
			result = append(result, ':')
			encoded, err := canonicalJSON(typed[key])
			if err != nil {
				return nil, err
			}
			result = append(result, encoded...)
		}
		return append(result, '}'), nil
	case []any:
		result := []byte{'['}
		for i, item := range typed {
			if i > 0 {
				result = append(result, ',')
			}
			encoded, err := canonicalJSON(item)
			if err != nil {
				return nil, err
			}
			result = append(result, encoded...)
		}
		return append(result, ']'), nil
	default:
		return json.Marshal(typed)
	}
}
