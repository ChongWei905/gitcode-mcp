package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gitcode-org-com/gitcode-mcp/api"
)

// AddRepositoryTools registers the common typed repository workflows.
func AddRepositoryTools(s *server.MCPServer, apiClient *api.GitCodeAPI) {
	listTool := mcp.NewTool("list_repositories",
		mcp.WithDescription("List repositories accessible to the authenticated GitCode user with pagination."),
		mcp.WithNumber("page", mcp.Description("Page number, starting at 1"), mcp.Min(1)),
		mcp.WithNumber("per_page", mcp.Description("Items per page, maximum 100"), mcp.Min(1), mcp.Max(100)),
	)
	s.AddTool(listTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, err := paginationValues(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_repositories", err)
		}
		repositories, err := apiClient.Repos.ListUserReposWithParams(params)
		if err != nil {
			return ErrorResult("list GitCode repositories", err)
		}
		return FormatJSONResult(repositories)
	})

	getTool := mcp.NewTool("get_repository",
		mcp.WithDescription("Get repository metadata."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
	)
	s.AddTool(getTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate get_repository", err)
		}
		repository, err := apiClient.Repos.GetRepo(owner, repo)
		if err != nil {
			return ErrorResult("get GitCode repository", err)
		}
		return FormatJSONResult(repository)
	})

	contentTool := mcp.NewTool("get_repository_content",
		mcp.WithDescription("Get a file or directory from a repository at a branch, tag, or commit. File content follows GitCode's base64 response schema."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("path", mcp.Required(), mcp.Description("Repository-relative file or directory path")),
		mcp.WithString("ref", mcp.Description("Branch, tag, or commit; defaults to the repository default branch")),
	)
	s.AddTool(contentTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate get_repository_content", err)
		}
		contentPath, err := requiredString(request.Params.Arguments, "path")
		if err != nil {
			return ErrorResult("validate get_repository_content", err)
		}
		if strings.HasPrefix(contentPath, "/") || hasUnsafeContentSegment(contentPath) {
			return ErrorResult("validate get_repository_content", fmt.Errorf("path must be repository-relative and must not contain . or .. segments"))
		}
		params := url.Values{}
		if ref := optionalString(request.Params.Arguments, "ref"); ref != "" {
			params.Set("ref", ref)
		}
		path := fmt.Sprintf("/repos/%s/%s/contents/%s", url.PathEscape(owner), url.PathEscape(repo), escapeContentPath(contentPath))
		response, err := apiClient.Do(ctx, http.MethodGet, path, params, nil)
		if err != nil {
			return ErrorResult("get GitCode repository content", err)
		}
		return FormatJSONResult(json.RawMessage(response.Body))
	})

	createTool := mcp.NewTool("create_repository",
		mcp.WithDescription("Create a repository for the authenticated user, then read it back."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Repository name")),
		mcp.WithString("description", mcp.Description("Repository description")),
		mcp.WithBoolean("private", mcp.Description("Whether the repository is private")),
	)
	s.AddTool(createTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := requiredString(request.Params.Arguments, "name")
		if err != nil {
			return ErrorResult("validate create_repository", err)
		}
		private, _ := request.Params.Arguments["private"].(bool)
		repository, err := apiClient.Repos.CreateRepo(name, optionalString(request.Params.Arguments, "description"), private)
		if err != nil {
			return ErrorResult("create GitCode repository", err)
		}
		return FormatJSONResult(repository)
	})
}

func escapeContentPath(path string) string {
	parts := strings.Split(path, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func hasUnsafeContentSegment(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return true
		}
	}
	return false
}
