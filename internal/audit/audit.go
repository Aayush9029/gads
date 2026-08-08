package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Entry struct {
	Time       string `json:"time"`
	Profile    string `json:"profile"`
	Action     string `json:"action"`
	CustomerID string `json:"customer_id,omitempty"`
	PlanHash   string `json:"plan_hash,omitempty"`
	Mode       string `json:"mode,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
	Result     string `json:"result"`
	Error      string `json:"error,omitempty"`
}

type Logger struct {
	Path    string
	Now     func() time.Time
	Secrets []string
}

func (l Logger) Append(entry Entry) error {
	if l.Path == "" {
		return nil
	}
	if l.Now == nil {
		l.Now = time.Now
	}
	entry.Time = l.Now().UTC().Format(time.RFC3339)
	entry.Error = Redact(entry.Error, l.Secrets...)
	dir := filepath.Dir(l.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(l.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(file, "%s\n", encoded); err != nil {
		return err
	}
	return file.Sync()
}

func Redact(value string, secrets ...string) string {
	redacted := value
	for _, secret := range secrets {
		if strings.TrimSpace(secret) != "" {
			redacted = strings.ReplaceAll(redacted, secret, "[REDACTED]")
		}
	}
	pattern := regexp.MustCompile(`(?i)(Bearer\s+|developer-token:\s*|client_secret=|refresh_token=)[^\s&;,]+`)
	redacted = pattern.ReplaceAllString(redacted, "${1}[REDACTED]")
	return redacted
}
