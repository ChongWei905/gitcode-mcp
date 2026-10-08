# GitCode MCP 2.0

这是 GitCode MCP 服务的扩展版本。它把 GitCode `/api/v5` 的认证、网络约束、常用工作流和通用 REST 访问统一封装起来，调用者不需要、也不允许在工具参数中传 AK。

## 解决的问题

- AK 固定从 `~/.gitcode_mcp/.env` 加载，MCP 客户端配置和每次 API 调用都不再重复填写。
- 保留仓库、分支、Issue、Pull Request 和搜索等常用强类型工具。
- `gitcode_api_request` 可以访问尚未封装成强类型工具的 GitCode `/api/v5` 接口，包括 Commit、Tag、Milestone、Organizations、Webhooks、Release、Actions、AI Hub 等类别。
- GitCode 返回的 ID/Issue 编号可能是 JSON 字符串或数字，客户端统一按字符串兼容解析。
- 请求只能发送到配置的 GitCode API 主机；跨主机跳转、完整 URL、调用方传入的 token 字段都会被拒绝。
- GitCode 请求不使用本机 HTTP(S) 代理，响应大小默认限制为 4 MiB。
- 写操作必须由用户授权；通用 `DELETE` 还必须传 `confirm_destructive=true`。

GitCode 当前官方 API 目录见：[GitCode OpenAPI](https://docs.gitcode.com/docs/apis/)。

## 凭据：唯一来源

默认凭据文件是：

```text
~/.gitcode_mcp/.env
```

内容：

```dotenv
GITCODE_TOKEN=<your-personal-access-token>
GITCODE_API_URL=https://api.gitcode.com/api/v5
MCP_TRANSPORT=stdio
```

权限必须收紧，否则服务会拒绝启动：

```bash
chmod 700 ~/.gitcode_mcp
chmod 600 ~/.gitcode_mcp/.env
```

配置优先级如下：

1. 进程环境变量 `GITCODE_TOKEN` 或 `GITCODE_ACCESS_TOKEN`
2. `GITCODE_MCP_ENV_FILE` 指定的文件
3. 默认文件 `~/.gitcode_mcp/.env`

不要把 AK 写进 `~/.codex/config.toml`、MCP 工具参数、命令历史或日志。

## Codex 配置示例

构建后，可通过仓库中的无密钥启动脚本运行本服务。将以下路径替换成克隆目录的绝对路径：

```toml
[mcp_servers.gitcode]
command = "<absolute-path-to-clone>/run-gitcode-mcp"
enabled = true

[mcp_servers.gitcode.env]
GITCODE_API_URL = "https://api.gitcode.com/api/v5"
MCP_TRANSPORT = "stdio"
```

启动脚本只负责清理代理并启动二进制；凭据由二进制自己读取。修改服务或配置后，需要在 Codex 设置中重启这个 MCP，或新开一个 Codex 任务。

## 工具面

### 认证与完整 API 兜底

- `gitcode_auth_status`：只读验证 MCP 内部凭据，不返回 token。
- `gitcode_api_request`：调用任意已文档化的相对 `/api/v5` 路径；自动鉴权。

通用读取示例的参数形状：

```json
{
  "method": "GET",
  "path": "/api/v5/user/issues",
  "query": {"state": "open", "page": 1, "per_page": 100}
}
```

通用工具禁止 `access_token`、`Authorization`、`PRIVATE-TOKEN`、`GITCODE_TOKEN` 等认证字段，也禁止传完整 URL。

### 常用强类型工具

- 仓库：`list_repositories`、`get_repository`、`get_repository_content`、`create_repository`
- 分支：`list_branches`、`get_branch`、`create_branch`
- Issue：`list_issues`、`get_issue`、`create_issue`、`update_issue`、`list_issue_comments`、`add_issue_comment`
- Pull Request：`list_pull_requests`、`get_pull_request`、`list_pull_request_reviews`、`list_pull_request_comments`、`reply_pull_request_comment`、`list_pull_request_files`、`list_pull_request_commits`、`list_pull_request_issues`、`create_pull_request`、`update_pull_request`
- 搜索：`search_code`、`search_repositories`、`search_issues`、`search_users`、`search_commits`

列表类工具统一支持 `page`/`per_page`；GitCode 单页上限为 100。

## 开发与验证

要求 Go 1.23+（仓库 toolchain 为 Go 1.24.1）。

```bash
env -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY \
  -u http_proxy -u https_proxy -u all_proxy \
  go test ./...

./build.sh
```

`build.sh` 会执行 `gofmt`、全量 Go 测试并生成 `bin/gitcode-mcp`。
