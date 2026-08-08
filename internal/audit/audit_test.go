package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRedactSecretsAndCredentialPatterns(t *testing.T) {
	t.Parallel()

	const developerToken = "dev-token-123"
	const clientSecret = "client-secret-456"
	const refreshToken = "refresh-token-789"
	input := strings.Join([]string{
		"explicit=" + developerToken,
		"secret=" + clientSecret,
		"refresh=" + refreshToken,
		"Authorization: Bearer access-token-value",
		"developer-token: header-token",
		"client_secret=query-secret&next=1",
		"refresh_token=query-refresh; trailing",
	}, " ")
	got := Redact(input, developerToken, clientSecret, refreshToken, "", "  ")
	for _, secret := range []string{developerToken, clientSecret, refreshToken, "access-token-value", "header-token", "query-secret", "query-refresh"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted value still contains %q: %s", secret, got)
		}
	}
	for _, prefix := range []string{"Bearer [REDACTED]", "developer-token: [REDACTED]", "client_secret=[REDACTED]", "refresh_token=[REDACTED]"} {
		if !strings.Contains(got, prefix) {
			t.Fatalf("redacted value %q does not contain %q", got, prefix)
		}
	}
}

func TestRedactIsCaseInsensitiveAndLeavesOrdinaryText(t *testing.T) {
	t.Parallel()

	input := "request failed: BEARER abc123 DEVELOPER-TOKEN: xyz CLIENT_SECRET=one REFRESH_TOKEN=two safe=value"
	got := Redact(input)
	for _, secret := range []string{"abc123", "xyz", "one", "two"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted value still contains %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "safe=value") {
		t.Fatalf("ordinary diagnostic was changed: %q", got)
	}
}

func TestLoggerAppendWritesSanitizedJSONLWithPrivatePermissions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "gads", "logs", "audit.jsonl")
	fixedNow := time.Date(2026, time.August, 7, 18, 4, 5, 0, time.UTC)
	logger := Logger{
		Path:    path,
		Now:     func() time.Time { return fixedNow },
		Secrets: []string{"super-secret-developer-token"},
	}
	entries := []Entry{
		{
			Time:       "caller-supplied-time-must-be-replaced",
			Profile:    "production",
			Action:     "mutate validate",
			CustomerID: "1234567890",
			PlanHash:   "sha256:abc",
			Mode:       "validate",
			RequestID:  "request-1",
			Result:     "error",
			Error:      "developer-token: super-secret-developer-token Bearer access-token",
		},
		{
			Profile: "production",
			Action:  "mutate execute",
			Result:  "ok",
		},
	}
	for _, entry := range entries {
		if err := logger.Append(entry); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat audit directory: %v", err)
	}
	if got, want := dirInfo.Mode().Perm(), os.FileMode(0o700); got != want {
		t.Fatalf("directory mode = %o, want %o", got, want)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat audit file: %v", err)
	}
	if got, want := fileInfo.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Fatalf("file mode = %o, want %o", got, want)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	for _, secret := range []string{"super-secret-developer-token", "access-token", "caller-supplied-time-must-be-replaced"} {
		if strings.Contains(string(contents), secret) {
			t.Fatalf("audit log contains sensitive or caller-controlled value %q: %s", secret, contents)
		}
	}
	if lines := strings.Count(string(contents), "\n"); lines != len(entries) {
		t.Fatalf("audit log has %d JSONL records, want %d", lines, len(entries))
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var decoded []Entry
	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("decode JSONL record: %v", err)
		}
		decoded = append(decoded, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan audit log: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("decoded %d entries, want 2", len(decoded))
	}
	if got, want := decoded[0].Time, "2026-08-07T18:04:05Z"; got != want {
		t.Fatalf("time = %q, want %q", got, want)
	}
	if decoded[0].Error != "developer-token: [REDACTED] Bearer [REDACTED]" {
		t.Fatalf("error = %q, want sanitized diagnostic", decoded[0].Error)
	}
	if decoded[1].Error != "" || decoded[1].CustomerID != "" {
		t.Fatalf("omitempty fields should decode empty: %#v", decoded[1])
	}
}

func TestLoggerAppendRepairsExistingPermissions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "audit")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "audit.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("seed audit log: %v", err)
	}
	if err := (Logger{Path: path, Now: func() time.Time { return time.Unix(0, 0) }}).Append(Entry{Action: "test", Result: "ok"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode = %o, want 700", dirInfo.Mode().Perm())
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o, want 600", fileInfo.Mode().Perm())
	}
}

func TestLoggerWithEmptyPathIsNoOp(t *testing.T) {
	t.Parallel()

	called := false
	logger := Logger{Now: func() time.Time { called = true; return time.Now() }}
	if err := logger.Append(Entry{Action: "test", Result: "ok"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if called {
		t.Fatal("empty-path logger invoked the clock")
	}
}
