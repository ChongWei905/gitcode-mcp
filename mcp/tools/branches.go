package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gitcode-org-com/gitcode-mcp/api"
)

// AddBranchTools registers the common typed branch workflows.
func AddBranchTools(s *server.MCPServer, apiClient *api.GitCodeAPI) {
	listTool := mcp.NewTool("list_branches",
		mcp.WithDescription("List repository branches with pagination."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithNumber("page", mcp.Description("Page number, starting at 1"), mcp.Min(1)),
		mcp.WithNumber("per_page", mcp.Description("Items per page, maximum 100"), mcp.Min(1), mcp.Max(100)),
	)
	s.AddTool(listTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_branches", err)
		}
		params, err := paginationValues(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate list_branches", err)
		}
		branches, err := apiClient.Branches.ListBranchesWithParams(owner, repo, params)
		if err != nil {
			return ErrorResult("list GitCode branches", err)
		}
		return FormatJSONResult(branches)
	})

	getTool := mcp.NewTool("get_branch",
		mcp.WithDescription("Get one repository branch."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("branch", mcp.Required(), mcp.Description("Branch name")),
	)
	s.AddTool(getTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate get_branch", err)
		}
		branch, err := requiredString(request.Params.Arguments, "branch")
		if err != nil {
			return ErrorResult("validate get_branch", err)
		}
		branchInfo, err := apiClient.Branches.GetBranch(owner, repo, branch)
		if err != nil {
			return ErrorResult("get GitCode branch", err)
		}
		return FormatJSONResult(branchInfo)
	})

	createTool := mcp.NewTool("create_branch",
		mcp.WithDescription("Create a branch from another branch, tag, or commit, then read it back."),
		mcp.WithString("owner", mcp.Required(), mcp.Description("Repository owner or organization path")),
		mcp.WithString("repo", mcp.Required(), mcp.Description("Repository path")),
		mcp.WithString("branch", mcp.Required(), mcp.Description("New branch name")),
		mcp.WithString("ref", mcp.Required(), mcp.Description("Source branch, tag, or commit SHA")),
	)
	s.AddTool(createTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		owner, repo, err := repositoryArguments(request.Params.Arguments)
		if err != nil {
			return ErrorResult("validate create_branch", err)
		}
		branch, err := requiredString(request.Params.Arguments, "branch")
		if err != nil {
			return ErrorResult("validate create_branch", err)
		}
		ref, err := requiredString(request.Params.Arguments, "ref")
		if err != nil {
			return ErrorResult("validate create_branch", err)
		}
		branchInfo, err := apiClient.Branches.CreateBranch(owner, repo, branch, ref)
		if err != nil {
			return ErrorResult("create GitCode branch", err)
		}
		return FormatJSONResult(branchInfo)
	})
}
