#!/bin/bash
set -euo pipefail

project_dir="$(cd "$(dirname "$0")" && pwd)"
install_dir="${GITCODE_MCP_INSTALL_DIR:-${HOME}/.local/bin}"
config_dir="${GITCODE_MCP_CONFIG_DIR:-${HOME}/.gitcode_mcp}"
dry_run=false

if [[ "${1:-}" == "--dry-run" ]]; then
  dry_run=true
fi

if [[ "${dry_run}" == true ]]; then
  echo "Would build ${project_dir}/bin/gitcode-mcp"
  echo "Would install the binary to ${install_dir}/gitcode-mcp"
  echo "Would preserve or create ${config_dir}/.env with mode 0600"
  exit 0
fi

"${project_dir}/build.sh"

mkdir -p "${install_dir}" "${config_dir}"
chmod 700 "${config_dir}"
install -m 0755 "${project_dir}/bin/gitcode-mcp" "${install_dir}/gitcode-mcp"

if [[ ! -e "${config_dir}/.env" ]]; then
  install -m 0600 "${project_dir}/.env.example" "${config_dir}/.env"
else
  chmod 600 "${config_dir}/.env"
fi

echo "Installed ${install_dir}/gitcode-mcp"
echo "Credential file: ${config_dir}/.env"
echo "Edit the credential file locally; do not put the token in MCP client configuration."
