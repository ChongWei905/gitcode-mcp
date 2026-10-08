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

// AddPullRequestTools registers the common typed Pull Request workflows.
func AddPullRequestTools(s *server.MCPServer, apiClient *api.GitCodeAPI) {
	listTool := mcp.NewTool("list_pull_requests",
		mcp.WithDescription("List repository Pull Requests with filters and pagination."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("state", mcp.Description("Pull Request state")),
		mcp.WithString("head", mcp.Description("Source branch filter")),
		mcp.WithString("base", mcp.Description("Target branch filter")),
		mcp.WithString("sort", mcp.Description("Sort field")),
		mcp.WithString("direction", mcp.Description("Sort direction"), mcp.Enum("asc", "desc")),
		mcp.WithNumber("page", mcp.Description("Page number, starting at 1"), mcp.Min(1)),
		mcp.WithNumber("per_page", mcp.Description("Items per page, maximum 100"), mcp.Min(1), mcp.Max(100)),
	)
	s.AddTool(listTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_pull_requests", err)
		}
		params, err := paginationValues(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_pull_requests", err)
		}
		pulls, err := apiClient.Pulls.ListPullRequestsWithParams(owner, repo, params)
		if err != nil {
			return ErrorResult("list GitCode Pull Requests", err)
		}
		return FormatJSONResult(pulls)
	})

	getTool := pullRequestNumberTool("get_pull_request", "Get one Pull Request. GitCode sequence numbers are handled as strings.")
	s.AddTool(getTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate get_pull_request", err)
		}
		number, err := requiredIdentifier(request.Params.Arguments, "pull_number")
		if err != nil {
			return ErrorResult("validate get_pull_request", err)
		}
		pull, err := apiClient.Pulls.GetPullRequestByNumber(owner, repo, number)
		if err != nil {
			return ErrorResult("get GitCode Pull Request", err)
		}
		return FormatJSONResult(pull)
	})

	reviewsTool := pullRequestNumberTool("list_pull_request_reviews", "List Pull Request reviews. Some GitCode repositories expose review discussion only through comments; use list_pull_request_comments when this endpoint returns 404.")
	s.AddTool(reviewsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_pull_request_reviews", err)
		}
		number, err := requiredIntegerIdentifier(request.Params.Arguments, "pull_number")
		if err != nil {
			return ErrorResult("validate list_pull_request_reviews", err)
		}
		reviews, err := apiClient.Pulls.ListPRReviews(owner, repo, number)
		if err != nil {
			return ErrorResult("list GitCode Pull Request reviews", err)
		}
		return FormatJSONResult(reviews)
	})

	commentsTool := mcp.NewTool("list_pull_request_comments",
		mcp.WithDescription("List Pull Request comments and inline discussions with pagination."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("pull_number", mcp.Required(), mcp.Description("Repository Pull Request sequence number")),
		mcp.WithNumber("page", mcp.Description("Page number, starting at 1"), mcp.Min(1)),
		mcp.WithNumber("per_page", mcp.Description("Items per page, maximum 100"), mcp.Min(1), mcp.Max(100)),
	)
	s.AddTool(commentsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_pull_request_comments", err)
		}
		number, err := requiredIdentifier(request.Params.Arguments, "pull_number")
		if err != nil {
			return ErrorResult("validate list_pull_request_comments", err)
		}
		params, err := paginationValues(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_pull_request_comments", err)
		}
		comments, err := apiClient.Pulls.ListPRCommentsWithParams(owner, repo, number, params)
		if err != nil {
			return ErrorResult("list GitCode Pull Request comments", err)
		}
		return FormatJSONResult(comments)
	})

	replyTool := mcp.NewTool("reply_pull_request_comment",
		mcp.WithDescription("Reply to a Pull Request discussion. Obtain discussion_id from list_pull_request_comments, then read comments back."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("pull_number", mcp.Required(), mcp.Description("Repository Pull Request sequence number")),
		mcp.WithString("discussion_id", mcp.Required(), mcp.Description("Discussion identifier from the comment response")),
		mcp.WithString("body", mcp.Required(), mcp.Description("Reply body")),
	)
	s.AddTool(replyTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate reply_pull_request_comment", err)
		}
		number, err := requiredIntegerIdentifier(request.Params.Arguments, "pull_number")
		if err != nil {
			return ErrorResult("validate reply_pull_request_comment", err)
		}
		discussionID, err := requiredString(request.Params.Arguments, "discussion_id")
		if err != nil {
			return ErrorResult("validate reply_pull_request_comment", err)
		}
		body, err := requiredString(request.Params.Arguments, "body")
		if err != nil {
			return ErrorResult("validate reply_pull_request_comment", err)
		}
		comment, err := apiClient.Pulls.ReplyToPRDiscussion(owner, repo, number, discussionID, body)
		if err != nil {
			return ErrorResult("reply to GitCode Pull Request comment", err)
		}
		return FormatJSONResult(comment)
	})

	registerPullRequestCollectionTool(s, apiClient, "list_pull_request_files", "files", "List files changed by a Pull Request with pagination.")
	registerPullRequestCollectionTool(s, apiClient, "list_pull_request_commits", "commits", "List commits in a Pull Request with pagination.")

	associatedIssuesTool := mcp.NewTool("list_pull_request_issues",
		mcp.WithDescription("List issues associated with a Pull Request. This is the canonical readback for PR-to-Issue links."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("pull_number", mcp.Required(), mcp.Description("Repository Pull Request sequence number")),
		mcp.WithNumber("page", mcp.Description("Page number, starting at 1"), mcp.Min(1)),
		mcp.WithNumber("per_page", mcp.Description("Items per page, maximum 100"), mcp.Min(1), mcp.Max(100)),
	)
	s.AddTool(associatedIssuesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_pull_request_issues", err)
		}
		number, err := requiredIntegerIdentifier(request.Params.Arguments, "pull_number")
		if err != nil {
			return ErrorResult("validate list_pull_request_issues", err)
		}
		params, err := paginationValues(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_pull_request_issues", err)
		}
		issues, err := apiClient.Pulls.ListAssociatedIssues(owner, repo, number, params)
		if err != nil {
			return ErrorResult("list GitCode Pull Request issues", err)
		}
		return FormatJSONResult(issues)
	})

	createTool := mcp.NewTool("create_pull_request",
		mcp.WithDescription("Create a Pull Request, then read it back before reporting success."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Pull Request title")),
		mcp.WithString("head", mcp.Required(), mcp.Description("Source branch")),
		mcp.WithString("base", mcp.Required(), mcp.Description("Target branch")),
		mcp.WithString("body", mcp.Description("Pull Request body")),
	)
	s.AddTool(createTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate create_pull_request", err)
		}
		title, err := requiredString(request.Params.Arguments, "title")
		if err != nil {
			return ErrorResult("validate create_pull_request", err)
		}
		head, err := requiredString(request.Params.Arguments, "head")
		if err != nil {
			return ErrorResult("validate create_pull_request", err)
		}
		base, err := requiredString(request.Params.Arguments, "base")
		if err != nil {
			return ErrorResult("validate create_pull_request", err)
		}
		pull, err := apiClient.Pulls.CreatePullRequest(owner, repo, title, head, base, optionalString(request.Params.Arguments, "body"))
		if err != nil {
			return ErrorResult("create GitCode Pull Request", err)
		}
		return FormatJSONResult(pull)
	})

	updateTool := mcp.NewTool("update_pull_request",
		mcp.WithDescription("Update Pull Request metadata or state, then read the Pull Request and linked issues back."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("pull_number", mcp.Required(), mcp.Description("Repository Pull Request sequence number")),
		mcp.WithString("title", mcp.Description("Replacement title")),
		mcp.WithString("body", mcp.Description("Replacement body; an empty string clears it")),
		mcp.WithString("state", mcp.Description("Pull Request state transition")),
		mcp.WithString("base", mcp.Description("Replacement target branch")),
	)
	s.AddTool(updateTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate update_pull_request", err)
		}
		number, err := requiredIdentifier(request.Params.Arguments, "pull_number")
		if err != nil {
			return ErrorResult("validate update_pull_request", err)
		}
		body := map[string]interface{}{}
		for _, field := range []string{"title", "body", "state", "base"} {
			if value, ok := request.Params.Arguments[field]; ok {
				body[field] = value
			}
		}
		if len(body) == 0 {
			return ErrorResult("validate update_pull_request", fmt.Errorf("at least one field to update is required"))
		}
		path := fmt.Sprintf("/repos/%s/%s/pulls/%s", url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(number))
		response, err := apiClient.Do(ctx, http.MethodPatch, path, nil, body)
		if err != nil {
			return ErrorResult("update GitCode Pull Request", err)
		}
		return FormatJSONResult(json.RawMessage(response.Body))
	})
}

func pullRequestNumberTool(name, description string) mcp.Tool {
	return mcp.NewTool(name,
		mcp.WithDescription(description),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("pull_number", mcp.Required(), mcp.Description("Repository Pull Request sequence number")),
	)
}

func repositoryArguments(arguments map[string]interface{}) (string, string, error) {
	owner, err := requiredString(arguments, "owner")
	if err != nil {
		return "", "", err
	}
	repo, err := requiredString(arguments, "repo")
	if err != nil {
		return "", "", err
	}
	if strings.ContainsAny(owner, "/\\") || strings.ContainsAny(repo, "/\\") {
		return "", "", fmt.Errorf("owner and repo must be single GitCode path segments")
	}
	return owner, repo, nil
}

func registerPullRequestCollectionTool(s *server.MCPServer, apiClient *api.GitCodeAPI, name, suffix, description string) {
	tool := mcp.NewTool(name,
		mcp.WithDescription(description),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("pull_number", mcp.Required(), mcp.Description("Repository Pull Request sequence number")),
		mcp.WithNumber("page", mcp.Description("Page number, starting at 1"), mcp.Min(1)),
		mcp.WithNumber("per_page", mcp.Description("Items per page, maximum 100"), mcp.Min(1), mcp.Max(100)),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate "+name, err)
		}
		number, err := requiredIdentifier(request.Params.Arguments, "pull_number")
		if err != nil {
			return ErrorResult("validate "+name, err)
		}
		params, err := paginationValues(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate "+name, err)
		}
		path := fmt.Sprintf("/repos/%s/%s/pulls/%s/%s", url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(number), suffix)
		response, err := apiClient.Do(ctx, http.MethodGet, path, params, nil)
		if err != nil {
			return ErrorResult("call "+name, err)
		}
		return FormatJSONResult(json.RawMessage(response.Body))
	})
}
