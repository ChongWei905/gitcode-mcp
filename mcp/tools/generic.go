package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gitcode-org-com/gitcode-mcp/api"
)

var genericRequestSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "method": {
      "type": "string",
      "enum": ["GET", "POST", "PUT", "PATCH", "DELETE"],
      "description": "HTTP method. Prefer GET unless the user explicitly authorized a write."
    },
    "path": {
      "type": "string",
      "pattern": "^/",
      "description": "Relative GitCode API path, with or without the /api/v5 prefix. Never include a host or credentials."
    },
    "query": {
      "type": "object",
      "description": "Query parameters. Authentication fields are rejected because the server adds them internally.",
      "additionalProperties": {
        "oneOf": [
          {"type": "string"},
          {"type": "number"},
          {"type": "boolean"},
          {"type": "array", "items": {"type": ["string", "number", "boolean"]}}
        ]
      }
    },
    "body": {
      "description": "JSON request body. May be an object, array, scalar, or null. Authentication fields are rejected."
    },
    "confirm_destructive": {
      "type": "boolean",
      "default": false,
      "description": "Must be true for DELETE after the user explicitly authorizes the exact destructive action."
    }
  },
  "required": ["method", "path"]
}`)

// AddGenericTools registers authenticated diagnostics and the safe REST escape hatch.
func AddGenericTools(s *server.MCPServer, apiClient *api.GitCodeAPI) {
	authStatusTool := mcp.NewTool("gitcode_auth_status",
		mcp.WithDescription("Verify the MCP-managed GitCode credential with a read-only /user request. Never returns the token."),
	)
	s.AddTool(authStatusTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		response, err := apiClient.Do(ctx, http.MethodGet, "/user", nil, nil)
		if err != nil {
			return ErrorResult("verify GitCode authentication", err)
		}

		var user map[string]interface{}
		if err := json.Unmarshal(response.Body, &user); err != nil {
			return ErrorResult("decode GitCode user response", err)
		}
		account := map[string]interface{}{}
		for _, key := range []string{"id", "login", "username", "name", "html_url"} {
			if value, ok := user[key]; ok {
				account[key] = value
			}
		}
		return FormatJSONResult(map[string]interface{}{
			"authenticated":     true,
			"api_url":           apiClient.BaseURL,
			"credential_source": apiClient.CredentialSource,
			"credential_file":   apiClient.CredentialFile,
			"account":           account,
		})
	})

	genericTool := mcp.NewToolWithRawSchema(
		"gitcode_api_request",
		"Call any documented GitCode /api/v5 endpoint through the MCP-managed credential. Prefer typed tools when available. Full URLs, caller-supplied tokens, and unconfirmed DELETE requests are rejected.",
		genericRequestSchema,
	)
	s.AddTool(genericTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		method, err := requiredString(request.Params.Arguments, "method")
		if err != nil {
			return ErrorResult("validate generic GitCode request", err)
		}
		method = strings.ToUpper(method)
		path, err := requiredString(request.Params.Arguments, "path")
		if err != nil {
			return ErrorResult("validate generic GitCode request", err)
		}
		if method == http.MethodDelete {
			confirmed, _ := request.Params.Arguments["confirm_destructive"].(bool)
			if !confirmed {
				return ErrorResult("validate generic GitCode request", errorsForDeleteConfirmation())
			}
		}

		query, err := genericQueryValues(request.Params.Arguments["query"])
		if err != nil {
			return ErrorResult("validate GitCode query", err)
		}
		body, hasBody := request.Params.Arguments["body"]
		if !hasBody {
			body = nil
		}
		response, err := apiClient.Do(ctx, method, path, query, body)
		if err != nil {
			return ErrorResult("GitCode API request failed", err)
		}
		return FormatJSONResult(formatGenericResponse(response))
	})
}

func errorsForDeleteConfirmation() error {
	return fmt.Errorf("DELETE requires confirm_destructive=true after explicit user authorization")
}

func genericQueryValues(raw interface{}) (url.Values, error) {
	values := url.Values{}
	if raw == nil {
		return values, nil
	}
	query, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("query must be an object")
	}
	if err := api.ValidateNoCredentialFields(query); err != nil {
		return nil, err
	}
	for key, rawValue := range query {
		items, err := genericQueryItems(rawValue)
		if err != nil {
			return nil, fmt.Errorf("query parameter %q: %w", key, err)
		}
		for _, item := range items {
			values.Add(key, item)
		}
	}
	return values, nil
}

func genericQueryItems(value interface{}) ([]string, error) {
	switch typed := value.(type) {
	case string:
		return []string{typed}, nil
	case bool:
		return []string{strconv.FormatBool(typed)}, nil
	case float64:
		return []string{strconv.FormatFloat(typed, 'f', -1, 64)}, nil
	case json.Number:
		return []string{typed.String()}, nil
	case []interface{}:
		items := make([]string, 0, len(typed))
		for _, child := range typed {
			childItems, err := genericQueryItems(child)
			if err != nil {
				return nil, err
			}
			items = append(items, childItems...)
		}
		return items, nil
	default:
		return nil, fmt.Errorf("value must be a string, number, boolean, or array of those types")
	}
}

func formatGenericResponse(response *api.Response) map[string]interface{} {
	result := map[string]interface{}{
		"status_code": response.StatusCode,
	}
	if len(response.Body) > 0 {
		var decoded interface{}
		decoder := json.NewDecoder(bytes.NewReader(response.Body))
		decoder.UseNumber()
		if err := decoder.Decode(&decoded); err == nil {
			result["body"] = decoded
		} else {
			result["body"] = string(response.Body)
		}
	}

	pagination := map[string]string{}
	for _, key := range []string{
		"Link", "Location", "X-Page", "X-Per-Page", "X-Next-Page", "X-Prev-Page",
		"X-Total", "X-Total-Count", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset",
	} {
		if value := response.Header.Get(key); value != "" {
			pagination[key] = value
		}
	}
	if len(pagination) > 0 {
		result["response_headers"] = pagination
	}
	return result
}
