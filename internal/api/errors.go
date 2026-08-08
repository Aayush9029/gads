package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ErrorDetail struct {
	Code     string
	Message  string
	Trigger  string
	Location string
}

type Error struct {
	HTTPStatus int
	Status     string
	Message    string
	RequestID  string
	Details    []ErrorDetail
}

func (e *Error) Error() string {
	parts := []string{"Google Ads API error"}
	if e.Status != "" {
		parts[0] += " " + e.Status
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	for _, detail := range e.Details {
		line := strings.TrimSpace(detail.Code + ": " + detail.Message)
		if detail.Location != "" {
			line += " at " + detail.Location
		}
		if detail.Trigger != "" {
			line += " (trigger: " + detail.Trigger + ")"
		}
		parts = append(parts, line)
	}
	if e.RequestID != "" {
		parts = append(parts, "request ID: "+e.RequestID)
	}
	return strings.Join(parts, "; ")
}

func ParseError(status int, requestID string, value any) error {
	root, _ := value.(map[string]any)
	outer, _ := root["error"].(map[string]any)
	parsed := &Error{HTTPStatus: status, RequestID: requestID}
	parsed.Status, _ = outer["status"].(string)
	parsed.Message, _ = outer["message"].(string)
	for _, detailValue := range asSlice(outer["details"]) {
		detailMap, _ := detailValue.(map[string]any)
		failure, _ := detailMap["errors"].([]any)
		if request, ok := detailMap["requestId"].(string); ok && parsed.RequestID == "" {
			parsed.RequestID = request
		}
		for _, item := range failure {
			encoded, _ := json.Marshal(item)
			var raw struct {
				ErrorCode map[string]string `json:"errorCode"`
				Message   string            `json:"message"`
				Trigger   struct {
					StringValue string `json:"stringValue"`
				} `json:"trigger"`
				Location struct {
					FieldPathElements []struct {
						FieldName string      `json:"fieldName"`
						Index     json.Number `json:"index"`
					} `json:"fieldPathElements"`
				} `json:"location"`
			}
			decoder := json.NewDecoder(strings.NewReader(string(encoded)))
			decoder.UseNumber()
			_ = decoder.Decode(&raw)
			code := ""
			for key, value := range raw.ErrorCode {
				code = key + "." + value
				break
			}
			location := ""
			for _, element := range raw.Location.FieldPathElements {
				if location != "" {
					location += "."
				}
				location += element.FieldName
				if element.Index != "" {
					location += fmt.Sprintf("[%s]", element.Index)
				}
			}
			parsed.Details = append(parsed.Details, ErrorDetail{Code: code, Message: raw.Message, Trigger: raw.Trigger.StringValue, Location: location})
		}
	}
	if parsed.Message == "" {
		parsed.Message = fmt.Sprintf("HTTP %d", status)
	}
	return parsed
}

func asSlice(value any) []any {
	items, _ := value.([]any)
	return items
}
