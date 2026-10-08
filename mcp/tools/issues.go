package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gitcode-org-com/gitcode-mcp/api"
)

// AddIssueTools registers the common typed Issue workflows.
func AddIssueTools(s *server.MCPServer, apiClient *api.GitCodeAPI) {
	listIssuesTool := mcp.NewTool("list_issues",
		mcp.WithDescription("List repository issues with filters and pagination."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("state", mcp.Description("Issue state: open, closed, or all"), mcp.Enum("open", "closed", "all")),
		mcp.WithString("labels", mcp.Description("Comma-separated label names")),
		mcp.WithString("sort", mcp.Description("Sort field, such as created or updated")),
		mcp.WithString("direction", mcp.Description("Sort direction"), mcp.Enum("asc", "desc")),
		mcp.WithString("search", mcp.Description("Keyword in title or body")),
		mcp.WithNumber("page", mcp.Description("Page number, starting at 1"), mcp.Min(1)),
		mcp.WithNumber("per_page", mcp.Description("Items per page, maximum 100"), mcp.Min(1), mcp.Max(100)),
	)
	s.AddTool(listIssuesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_issues", err)
		}
		params, err := paginationValues(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_issues", err)
		}
		issues, err := apiClient.Issues.ListIssuesWithParams(owner, repo, params)
		if err != nil {
			return ErrorResult("list GitCode issues", err)
		}
		return FormatJSONResult(issues)
	})

	getIssueTool := mcp.NewTool("get_issue",
		mcp.WithDescription("Get one repository issue. GitCode issue numbers are handled as strings."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("issue_number", mcp.Required(), mcp.Description("Repository issue sequence number, without #")),
	)
	s.AddTool(getIssueTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate get_issue", err)
		}
		number, err := requiredIdentifier(request.Params.Arguments, "issue_number")
		if err != nil {
			return ErrorResult("validate get_issue", err)
		}
		issue, err := apiClient.Issues.GetIssueByNumber(owner, repo, number)
		if err != nil {
			return ErrorResult("get GitCode issue", err)
		}
		return FormatJSONResult(issue)
	})

	createIssueTool := mcp.NewTool("create_issue",
		mcp.WithDescription("Create a repository issue. This is a write operation; read the issue back after creation."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Issue title")),
		mcp.WithString("body", mcp.Description("Issue body")),
	)
	s.AddTool(createIssueTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate create_issue", err)
		}
		title, err := requiredString(request.Params.Arguments, "title")
		if err != nil {
			return ErrorResult("validate create_issue", err)
		}
		issue, err := apiClient.Issues.CreateIssue(owner, repo, title, optionalString(request.Params.Arguments, "body"))
		if err != nil {
			return ErrorResult("create GitCode issue", err)
		}
		return FormatJSONResult(issue)
	})

	updateIssueTool := mcp.NewTool("update_issue",
		mcp.WithDescription("Update issue fields. Use state=close or state=reopen for lifecycle changes, then read the issue back."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("issue_number", mcp.Required(), mcp.Description("Repository issue sequence number, without #")),
		mcp.WithString("title", mcp.Description("Replacement title")),
		mcp.WithString("body", mcp.Description("Replacement body; an empty string clears it")),
		mcp.WithString("state", mcp.Description("Lifecycle action: close or reopen"), mcp.Enum("close", "reopen")),
		mcp.WithString("assignee", mcp.Description("Assignee username")),
		mcp.WithString("labels", mcp.Description("Comma-separated label names")),
	)
	s.AddTool(updateIssueTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate update_issue", err)
		}
		number, err := requiredIdentifier(request.Params.Arguments, "issue_number")
		if err != nil {
			return ErrorResult("validate update_issue", err)
		}
		body := map[string]interface{}{}
		for _, field := range []string{"title", "body", "state", "assignee", "labels"} {
			if value, ok := request.Params.Arguments[field]; ok {
				body[field] = value
			}
		}
		if len(body) == 0 {
			return ErrorResult("validate update_issue", fmt.Errorf("at least one field to update is required"))
		}
		path := fmt.Sprintf("/repos/%s/%s/issues/%s", url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(number))
		response, err := apiClient.Do(ctx, http.MethodPatch, path, nil, body)
		if err != nil {
			return ErrorResult("update GitCode issue", err)
		}
		return FormatJSONResult(json.RawMessage(response.Body))
	})

	listCommentsTool := mcp.NewTool("list_issue_comments",
		mcp.WithDescription("List comments on one issue with pagination."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("issue_number", mcp.Required(), mcp.Description("Repository issue sequence number, without #")),
		mcp.WithNumber("page", mcp.Description("Page number, starting at 1"), mcp.Min(1)),
		mcp.WithNumber("per_page", mcp.Description("Items per page, maximum 100"), mcp.Min(1), mcp.Max(100)),
		mcp.WithString("since", mcp.Description("Only comments updated since this ISO-8601 timestamp")),
	)
	s.AddTool(listCommentsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_issue_comments", err)
		}
		number, err := requiredIdentifier(request.Params.Arguments, "issue_number")
		if err != nil {
			return ErrorResult("validate list_issue_comments", err)
		}
		params, err := paginationValues(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_issue_comments", err)
		}
		comments, err := apiClient.Issues.ListCommentsWithParams(owner, repo, number, params)
		if err != nil {
			return ErrorResult("list GitCode issue comments", err)
		}
		return FormatJSONResult(comments)
	})

	addCommentTool := mcp.NewTool("add_issue_comment",
		mcp.WithDescription("Add a comment to an issue, then read the comment or issue back."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("issue_number", mcp.Required(), mcp.Description("Repository issue sequence number, without #")),
		mcp.WithString("body", mcp.Required(), mcp.Description("Comment body")),
	)
	s.AddTool(addCommentTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate add_issue_comment", err)
		}
		number, err := requiredIdentifier(request.Params.Arguments, "issue_number")
		if err != nil {
			return ErrorResult("validate add_issue_comment", err)
		}
		commentBody, err := requiredString(request.Params.Arguments, "body")
		if err != nil {
			return ErrorResult("validate add_issue_comment", err)
		}
		path := fmt.Sprintf("/repos/%s/%s/issues/%s/comments", url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(number))
		response, err := apiClient.Do(ctx, http.MethodPost, path, nil, map[string]string{"body": commentBody})
		if err != nil {
			return ErrorResult("add GitCode issue comment", err)
		}
		return FormatJSONResult(json.RawMessage(response.Body))
	})
}
