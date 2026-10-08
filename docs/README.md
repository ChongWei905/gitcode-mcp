# GitCode MCP 客户端配置

本目录保留 Claude、Cline、Cursor 和 Windsurf 的 STDIO 配置示例。客户端配置只负责启动服务，不保存 GitCode AK。

```json
{
  "mcpServers": {
    "gitcode": {
      "command": "gitcode-mcp",
      "args": [],
      "env": {
        "GITCODE_API_URL": "https://api.gitcode.com/api/v5"
      }
    }
  }
}
```

AK 只放在本地固定文件中：

```text
~/.gitcode_mcp/.env
```

并确保权限为 0600：

```bash
chmod 700 ~/.gitcode_mcp
chmod 600 ~/.gitcode_mcp/.env
```

如果二进制不在 `PATH`，把示例中的 `command` 改成 `gitcode-mcp` 或 `run-gitcode-mcp` 的绝对路径。修改配置后重启对应 MCP 客户端。
