package output

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
)

type Format string

const (
	Table Format = "table"
	JSON  Format = "json"
	JSONL Format = "jsonl"
	CSV   Format = "csv"
)

func ParseFormat(value string) (Format, error) {
	if value == "" {
		return Table, nil
	}
	format := Format(strings.ToLower(value))
	switch format {
	case Table, JSON, JSONL, CSV:
		return format, nil
	default:
		return "", fmt.Errorf("unknown output format %q; use table, json, jsonl, or csv", value)
	}
}

func Write(w io.Writer, value any, format Format) error {
	switch format {
	case JSON:
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	case JSONL:
		return writeJSONL(w, rows(value))
	case CSV:
		return writeCSV(w, rows(value))
	case Table:
		return writeTable(w, rows(value))
	default:
		return errors.New("invalid output format")
	}
}

func rows(value any) []any {
	if root, ok := value.(map[string]any); ok {
		for _, key := range []string{"results", "resourceNames", "googleAdsFields", "mutateOperationResponses"} {
			if list, ok := root[key].([]any); ok {
				return list
			}
		}
	}
	if list, ok := value.([]any); ok {
		return list
	}
	return []any{value}
}

func writeJSONL(w io.Writer, values []any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	for _, value := range values {
		if err := encoder.Encode(value); err != nil {
			return err
		}
	}
	return nil
}

func writeCSV(w io.Writer, values []any) error {
	records, columns := records(values)
	writer := csv.NewWriter(w)
	if err := writer.Write(columns); err != nil {
		return err
	}
	for _, record := range records {
		row := make([]string, len(columns))
		for index, column := range columns {
			row[index] = record[column]
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeTable(w io.Writer, values []any) error {
	records, columns := records(values)
	if len(columns) == 0 {
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(columns, "\t")); err != nil {
		return err
	}
	for _, record := range records {
		row := make([]string, len(columns))
		for index, column := range columns {
			row[index] = record[column]
		}
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func records(values []any) ([]map[string]string, []string) {
	result := make([]map[string]string, 0, len(values))
	set := map[string]struct{}{}
	for _, value := range values {
		record := map[string]string{}
		flatten("", value, record)
		for key := range record {
			set[key] = struct{}{}
		}
		result = append(result, record)
	}
	columns := make([]string, 0, len(set))
	for column := range set {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	return result, columns
}

func flatten(prefix string, value any, record map[string]string) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			flatten(next, typed[key], record)
		}
	case []any:
		encoded, _ := json.Marshal(typed)
		record[prefix] = string(encoded)
	case nil:
		record[prefix] = ""
	case json.Number:
		record[prefix] = typed.String()
	case string:
		record[prefix] = typed
	default:
		record[prefix] = fmt.Sprint(typed)
	}
}
