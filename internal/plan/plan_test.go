package plan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewCreatesDeterministicPlan(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 7, 14, 5, 6, 999, time.FixedZone("EDT", -4*60*60))
	operations := []any{
		map[string]any{
			"campaignOperation": map[string]any{
				"create": map[string]any{"name": "Brand", "status": "PAUSED"},
			},
		},
	}
	p, err := New("v22", "1234567890", true, operations, now)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, want := p.Schema, "gads.mutation-plan/v1"; got != want {
		t.Fatalf("schema = %q, want %q", got, want)
	}
	if got, want := p.CreatedAt, "2026-08-07T18:05:06Z"; got != want {
		t.Fatalf("created_at = %q, want %q", got, want)
	}
	if !strings.HasPrefix(p.Hash, "sha256:") || len(p.Hash) != len("sha256:")+64 {
		t.Fatalf("hash = %q, want sha256 and 64 lowercase hex characters", p.Hash)
	}
	if err := p.Verify(); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	samePayloadDifferentMapOrder := []any{
		map[string]any{
			"campaignOperation": map[string]any{
				"create": map[string]any{"status": "PAUSED", "name": "Brand"},
			},
		},
	}
	p2, err := New("v22", "1234567890", true, samePayloadDifferentMapOrder, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("New reordered: %v", err)
	}
	if p.Hash != p2.Hash {
		t.Fatalf("hash changed with map insertion order or timestamp: %q != %q", p.Hash, p2.Hash)
	}
}

func TestNewRequestCreatesDeterministicActionPlan(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 7, 18, 0, 0, 0, time.UTC)
	p, err := NewRequest(
		"v22",
		"1234567890",
		"POST",
		"customers/1234567890:uploadClickConversions",
		map[string]any{"partialFailure": true, "conversions": []any{map[string]any{"gclid": "abc"}}},
		now,
	)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if p.Method != "POST" || p.Path != "customers/1234567890:uploadClickConversions" {
		t.Fatalf("request plan method/path = %q %q", p.Method, p.Path)
	}
	if len(p.Operations) != 0 {
		t.Fatalf("action plan unexpectedly has %d mutate operations", len(p.Operations))
	}
	if err := p.Verify(); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestNewValidatesRequiredInputs(t *testing.T) {
	t.Parallel()

	now := time.Unix(0, 0)
	if _, err := New("v22", "", false, []any{map[string]any{}}, now); err == nil || err.Error() != "customer ID is required" {
		t.Fatalf("missing customer error = %v", err)
	}
	if _, err := New("v22", "123", false, nil, now); err == nil || err.Error() != "at least one mutation operation is required" {
		t.Fatalf("missing operations error = %v", err)
	}
	if _, err := NewRequest("v22", "", "POST", "path", nil, now); err == nil || err.Error() != "customer ID is required" {
		t.Fatalf("missing action customer error = %v", err)
	}
	for _, tt := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "method", path: "path"},
		{name: "path", method: "POST"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewRequest("v22", "123", tt.method, tt.path, nil, now); err == nil || err.Error() != "method and path are required" {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestDeterministicHashGoldenAndSensitivity(t *testing.T) {
	t.Parallel()

	base := Plan{
		Schema:         "gads.mutation-plan/v1",
		APIVersion:     "v22",
		CustomerID:     "1234567890",
		PartialFailure: true,
		Operations: []any{
			map[string]any{"campaignBudgetOperation": map[string]any{"create": map[string]any{"amountMicros": json.Number("9007199254740993"), "name": "Budget"}}},
		},
	}
	got, err := base.DeterministicHash()
	if err != nil {
		t.Fatalf("DeterministicHash: %v", err)
	}
	const want = "sha256:ca2ad7484150f47a625cd72a4b482c5c22b55e73dce7c01e3ef994867a31c8fc"
	if got != want {
		t.Fatalf("hash golden = %q, want %q", got, want)
	}

	changed := base
	changed.PartialFailure = false
	changedHash, err := changed.DeterministicHash()
	if err != nil {
		t.Fatalf("changed DeterministicHash: %v", err)
	}
	if changedHash == got {
		t.Fatal("hash did not change after partial_failure changed")
	}

	reordered := base
	reordered.Operations = append(append([]any(nil), base.Operations...), map[string]any{"customerOperation": map[string]any{"update": map[string]any{"resourceName": "customers/1234567890"}}})
	reorderedHash, err := reordered.DeterministicHash()
	if err != nil {
		t.Fatalf("second operation DeterministicHash: %v", err)
	}
	reordered.Operations[0], reordered.Operations[1] = reordered.Operations[1], reordered.Operations[0]
	operationOrderHash, err := reordered.DeterministicHash()
	if err != nil {
		t.Fatalf("reordered DeterministicHash: %v", err)
	}
	if operationOrderHash == reorderedHash {
		t.Fatal("hash did not bind mutation operation order")
	}
}

func TestVerifyDetectsTamperingAndInvalidShape(t *testing.T) {
	t.Parallel()

	p, err := New("v22", "123", false, []any{map[string]any{"customerOperation": map[string]any{"update": map[string]any{"resourceName": "customers/123"}}}}, time.Now())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tests := []struct {
		name      string
		mutate    func(*Plan)
		wantError string
	}{
		{name: "schema", mutate: func(p *Plan) { p.Schema = "gads.mutation-plan/v2" }, wantError: "unsupported plan schema"},
		{name: "customer", mutate: func(p *Plan) { p.CustomerID = "999" }, wantError: "plan hash mismatch"},
		{name: "api version", mutate: func(p *Plan) { p.APIVersion = "v99" }, wantError: "plan hash mismatch"},
		{name: "partial failure", mutate: func(p *Plan) { p.PartialFailure = true }, wantError: "plan hash mismatch"},
		{name: "operations", mutate: func(p *Plan) { p.Operations[0].(map[string]any)["tampered"] = true }, wantError: "plan hash mismatch"},
		{name: "hash", mutate: func(p *Plan) { p.Hash = "sha256:00" }, wantError: "plan hash mismatch"},
		{name: "neither plan shape", mutate: func(p *Plan) { p.Operations = nil }, wantError: "plan must contain either"},
		{name: "both plan shapes", mutate: func(p *Plan) { p.Path = "customers/123:mutate" }, wantError: "plan must contain either"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copy := deepCopyPlan(t, p)
			tt.mutate(&copy)
			err := copy.Verify()
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Verify error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestWriteReadRoundTripUsesPrivatePermissionsAndPreservesNumbers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "private", "nested", "plan.json")
	p, err := New(
		"v22",
		"1234567890",
		false,
		[]any{map[string]any{"campaignBudgetOperation": map[string]any{"create": map[string]any{"amountMicros": json.Number("9223372036854775807")}}}},
		time.Date(2026, time.August, 7, 18, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := Write(path, p); err != nil {
		t.Fatalf("Write: %v", err)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat plan: %v", err)
	}
	if got, want := fileInfo.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Fatalf("plan mode = %o, want %o", got, want)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat plan directory: %v", err)
	}
	if got, want := dirInfo.Mode().Perm(), os.FileMode(0o700); got != want {
		t.Fatalf("plan directory mode = %o, want %o", got, want)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Hash != p.Hash {
		t.Fatalf("round-trip hash = %q, want %q", got.Hash, p.Hash)
	}
	operation := got.Operations[0].(map[string]any)
	budget := operation["campaignBudgetOperation"].(map[string]any)
	create := budget["create"].(map[string]any)
	amount, ok := create["amountMicros"].(json.Number)
	if !ok || amount.String() != "9223372036854775807" {
		t.Fatalf("amountMicros = %#v (%T), want exact json.Number", create["amountMicros"], create["amountMicros"])
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read raw plan: %v", err)
	}
	if len(contents) == 0 || contents[len(contents)-1] != '\n' {
		t.Fatal("serialized plan must end in a newline")
	}
}

func TestReadRejectsTamperedPlanFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "plan.json")
	p, err := New("v22", "123", false, []any{map[string]any{"customerOperation": map[string]any{"update": map[string]any{"resourceName": "customers/123"}}}}, time.Now())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := Write(path, p); err != nil {
		t.Fatalf("Write: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	tampered := strings.Replace(string(contents), "customers/123", "customers/999", 1)
	if err := os.WriteFile(path, []byte(tampered), 0o600); err != nil {
		t.Fatalf("tamper plan: %v", err)
	}
	if _, err := Read(path); err == nil || !strings.Contains(err.Error(), "plan hash mismatch") {
		t.Fatalf("Read error = %v, want plan hash mismatch", err)
	}
}

func TestReadRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(path, []byte("{not json\n"), 0o600); err != nil {
		t.Fatalf("write malformed plan: %v", err)
	}
	if _, err := Read(path); err == nil || !strings.HasPrefix(err.Error(), "decode plan:") {
		t.Fatalf("Read error = %v, want decode plan error", err)
	}
}

func TestWriteRejectsUnverifiedPlan(t *testing.T) {
	t.Parallel()

	err := Write(filepath.Join(t.TempDir(), "plan.json"), Plan{Schema: "gads.mutation-plan/v1"})
	if err == nil || !strings.Contains(err.Error(), "plan must contain either") {
		t.Fatalf("Write error = %v, want invalid shape", err)
	}
}

func deepCopyPlan(t *testing.T, value Plan) Plan {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal plan copy: %v", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.UseNumber()
	var result Plan
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("decode plan copy: %v", err)
	}
	return result
}
