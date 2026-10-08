package tools

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// FormatJSONResult encodes a successful tool result as JSON text.
func FormatJSONResult(data interface{}) (*mcp.CallToolResult, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return ErrorResult("encode tool response", err)
	}
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// ErrorResult reports operational failures inside the MCP tool result.
func ErrorResult(action string, err error) (*mcp.CallToolResult, error) {
	result := mcp.NewToolResultText(fmt.Sprintf("%s: %v", action, err))
	result.IsError = true
	return result, nil
}

func requiredString(arguments map[string]interface{}, name string) (string, error) {
	value, ok := arguments[name]
	if !ok {
		return "", fmt.Errorf("missing required argument %q", name)
	}
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("argument %q must be a non-empty string", name)
	}
	return text, nil
}

func optionalString(arguments map[string]interface{}, name string) string {
	text, _ := arguments[name].(string)
	return text
}

func requiredIdentifier(arguments map[string]interface{}, name string) (string, error) {
	value, ok := arguments[name]
	if !ok {
		return "", fmt.Errorf("missing required argument %q", name)
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return "", fmt.Errorf("argument %q must not be empty", name)
		}
		return typed, nil
	case float64:
		if typed < 0 || typed != float64(int64(typed)) {
			return "", fmt.Errorf("argument %q must be a non-negative integer", name)
		}
		return strconv.FormatInt(int64(typed), 10), nil
	case json.Number:
		return typed.String(), nil
	default:
		return "", fmt.Errorf("argument %q must be a string or integer", name)
	}
}

func requiredIntegerIdentifier(arguments map[string]interface{}, name string) (int, error) {
	identifier, err := requiredIdentifier(arguments, name)
	if err != nil {
		return 0, err
	}
	number, err := strconv.Atoi(identifier)
	if err != nil || number < 0 {
		return 0, fmt.Errorf("argument %q must be a non-negative integer", name)
	}
	return number, nil
}

func paginationValues(arguments map[string]interface{}) (url.Values, error) {
	values := url.Values{}
	for _, field := range []string{"state", "labels", "sort", "direction", "search", "head", "base", "since"} {
		if value := optionalString(arguments, field); value != "" {
			values.Set(field, value)
		}
	}
	for _, field := range []string{"page", "per_page"} {
		value, ok := arguments[field]
		if !ok {
			continue
		}
		number, ok := value.(float64)
		if !ok || number < 1 || number != float64(int(number)) {
			return nil, fmt.Errorf("argument %q must be a positive integer", field)
		}
		if field == "per_page" && number > 100 {
			return nil, fmt.Errorf("argument %q must not exceed 100", field)
		}
		values.Set(field, strconv.Itoa(int(number)))
	}
	return values, nil
}

func addPaginationOptions(options []mcp.ToolOption) []mcp.ToolOption {
	return append(options,
		mcp.WithNumber("page", mcp.Description("Page number, starting at 1"), mcp.Min(1)),
		mcp.WithNumber("per_page", mcp.Description("Items per page, maximum 100"), mcp.Min(1), mcp.Max(100)),
	)
}
