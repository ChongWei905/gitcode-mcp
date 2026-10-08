#!/bin/bash
set -euo pipefail

project_dir="$(cd "$(dirname "$0")" && pwd)"
build_dir="${project_dir}/bin"

mkdir -p "${build_dir}"
cd "${project_dir}"

echo "Formatting GitCode MCP sources..."
gofmt -w ./api ./config ./mcp ./main.go

echo "Running GitCode MCP tests..."
go test ./...

echo "Building GitCode MCP 2.0..."
go build -trimpath -o "${build_dir}/gitcode-mcp" .

echo "Built ${build_dir}/gitcode-mcp"
