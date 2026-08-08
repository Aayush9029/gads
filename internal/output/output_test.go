package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParseFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  Format
	}{
		{name: "default", input: "", want: Table},
		{name: "table", input: "TABLE", want: Table},
		{name: "json", input: "Json", want: JSON},
		{name: "jsonl", input: "JSONL", want: JSONL},
		{name: "csv", input: "Csv", want: CSV},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseFormat(tt.input)
			if err != nil {
				t.Fatalf("ParseFormat(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ParseFormat(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseFormatRejectsUnknownFormat(t *testing.T) {
	t.Parallel()

	got, err := ParseFormat("yaml")
	if err == nil {
		t.Fatal("ParseFormat(yaml) unexpectedly succeeded")
	}
	if got != "" {
		t.Fatalf("format = %q, want empty", got)
	}
	if want := `unknown output format "yaml"; use table, json, jsonl, or csv`; err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestWriteGoldenFormatsPreserveNumericPrecision(t *testing.T) {
	t.Parallel()

	value := []any{
		map[string]any{
			"campaign": map[string]any{"id": "123", "name": "Alpha"},
			"clicks":   json.Number("9007199254740993"),
		},
	}
	tests := []struct {
		name   string
		format Format
		want   string
	}{
		{
			name:   "json",
			format: JSON,
			want: "[\n" +
				"  {\n" +
				"    \"campaign\": {\n" +
				"      \"id\": \"123\",\n" +
				"      \"name\": \"Alpha\"\n" +
				"    },\n" +
				"    \"clicks\": 9007199254740993\n" +
				"  }\n" +
				"]\n",
		},
		{
			name:   "jsonl",
			format: JSONL,
			want:   "{\"campaign\":{\"id\":\"123\",\"name\":\"Alpha\"},\"clicks\":9007199254740993}\n",
		},
		{
			name:   "csv",
			format: CSV,
			want:   "campaign.id,campaign.name,clicks\n123,Alpha,9007199254740993\n",
		},
		{
			name:   "table",
			format: Table,
			want:   "campaign.id  campaign.name  clicks\n123          Alpha          9007199254740993\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			if err := Write(&output, value, tt.format); err != nil {
				t.Fatalf("Write(%s): %v", tt.format, err)
			}
			if got := output.String(); got != tt.want {
				t.Fatalf("golden mismatch\n--- got ---\n%s--- want ---\n%s", got, tt.want)
			}
			if !strings.Contains(output.String(), "9007199254740993") {
				t.Fatal("output lost the exact integer representation")
			}
		})
	}
}

func TestWriteExtractsKnownGoogleResponseCollections(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"results", "resourceNames", "googleAdsFields", "mutateOperationResponses"} {
		key := key
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			value := map[string]any{
				key:        []any{map[string]any{"id": json.Number("123")}},
				"metadata": "must not become a row",
			}
			var output bytes.Buffer
			if err := Write(&output, value, JSONL); err != nil {
				t.Fatalf("Write: %v", err)
			}
			if got, want := output.String(), "{\"id\":123}\n"; got != want {
				t.Fatalf("output = %q, want %q", got, want)
			}
		})
	}
}

func TestWriteCSVAndTableFlattenNestedValuesDeterministically(t *testing.T) {
	t.Parallel()

	value := []any{
		map[string]any{
			"z": nil,
			"nested": map[string]any{
				"b": true,
				"a": []any{"x", json.Number("9007199254740993")},
			},
		},
		map[string]any{"nested": map[string]any{"b": false}},
	}

	var csvOutput bytes.Buffer
	if err := Write(&csvOutput, value, CSV); err != nil {
		t.Fatalf("Write CSV: %v", err)
	}
	if got, want := csvOutput.String(), "nested.a,nested.b,z\n\"[\"\"x\"\",9007199254740993]\",true,\n,false,\n"; got != want {
		t.Fatalf("CSV output = %q, want %q", got, want)
	}

	var tableOutput bytes.Buffer
	if err := Write(&tableOutput, value, Table); err != nil {
		t.Fatalf("Write table: %v", err)
	}
	if got, want := tableOutput.String(), "nested.a                nested.b  z\n[\"x\",9007199254740993]  true      \n                        false     \n"; got != want {
		t.Fatalf("table output = %q, want %q", got, want)
	}
}

func TestWriteRejectsInvalidFormat(t *testing.T) {
	t.Parallel()

	err := Write(&bytes.Buffer{}, map[string]any{"ok": true}, Format("yaml"))
	if err == nil || err.Error() != "invalid output format" {
		t.Fatalf("error = %v, want invalid output format", err)
	}
}

func TestWritePropagatesWriterErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("write failed")
	for _, format := range []Format{JSON, JSONL, CSV, Table} {
		format := format
		t.Run(string(format), func(t *testing.T) {
			t.Parallel()
			err := Write(errorWriter{err: wantErr}, []any{map[string]any{"id": "1"}}, format)
			if !errors.Is(err, wantErr) {
				t.Fatalf("error = %v, want %v", err, wantErr)
			}
		})
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) {
	return 0, w.err
}
