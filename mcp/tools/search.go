package tools

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gitcode-org-com/gitcode-mcp/api"
)

// AddSearchTools registers paginated GitCode search tools.
func AddSearchTools(s *server.MCPServer, apiClient *api.GitCodeAPI) {
	registerSearchTool(s, apiClient, "search_code", "/search/code", "Search GitCode source code.")
	registerSearchTool(s, apiClient, "search_repositories", "/search/repositories", "Search GitCode repositories.")
	registerSearchTool(s, apiClient, "search_issues", "/search/issues", "Search GitCode issues.")
	registerSearchTool(s, apiClient, "search_users", "/search/users", "Search GitCode users.")
	registerSearchTool(s, apiClient, "search_commits", "/search/commits", "Search GitCode commits.")
}

func registerSearchTool(s *server.MCPServer, apiClient *api.GitCodeAPI, name, path, description string) {
	tool := mcp.NewTool(name,
		mcp.WithDescription(description),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithNumber("page", mcp.Description("Page number, starting at 1"), mcp.Min(1)),
		mcp.WithNumber("per_page", mcp.Description("Items per page, maximum 100"), mcp.Min(1), mcp.Max(100)),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := requiredString(request.Params.Arguments, "query")
		if err != nil {
			return ErrorResult("validate "+name, err)
		}
		params, err := paginationValues(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate "+name, err)
		}
		params.Set("q", query)
		response, err := apiClient.Do(ctx, http.MethodGet, path, params, nil)
		if err != nil {
			return ErrorResult("call "+name, err)
		}
		return FormatJSONResult(json.RawMessage(response.Body))
	})
}
