package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitLoadsStableCredentialFile(t *testing.T) {
	home := t.TempDir()
	credentialDir := filepath.Join(home, ".gitcode_mcp")
	if err := os.MkdirAll(credentialDir, 0o700); err != nil {
		t.Fatal(err)
	}
	credentialFile := filepath.Join(credentialDir, ".env")
	contents := "GITCODE_TOKEN=file-secret\nGITCODE_API_URL=https://api.gitcode.com/api/v5\nMCP_TRANSPORT=stdio\n"
	if err := os.WriteFile(credentialFile, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	clearConfigEnvironment(t)
	t.Setenv("HOME", home)
	if err := Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if GlobalConfig.GitCodeToken != "file-secret" {
		t.Fatalf("token = %q, want file credential", GlobalConfig.GitCodeToken)
	}
	if GlobalConfig.CredentialSource != "credential_file" {
		t.Fatalf("credential source = %q", GlobalConfig.CredentialSource)
	}
	if GlobalConfig.CredentialFile != credentialFile {
		t.Fatalf("credential file = %q, want %q", GlobalConfig.CredentialFile, credentialFile)
	}
}

func TestInitEnvironmentOverridesCredentialFile(t *testing.T) {
	credentialFile := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(credentialFile, []byte("GITCODE_TOKEN=file-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	clearConfigEnvironment(t)
	t.Setenv("GITCODE_MCP_ENV_FILE", credentialFile)
	t.Setenv("GITCODE_TOKEN", "environment-secret")
	if err := Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if GlobalConfig.GitCodeToken != "environment-secret" {
		t.Fatalf("token = %q, want environment credential", GlobalConfig.GitCodeToken)
	}
	if GlobalConfig.CredentialSource != "environment" {
		t.Fatalf("credential source = %q", GlobalConfig.CredentialSource)
	}
}

func TestInitRejectsInsecureCredentialPermissions(t *testing.T) {
	credentialFile := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(credentialFile, []byte("GITCODE_TOKEN=secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	clearConfigEnvironment(t)
	t.Setenv("GITCODE_MCP_ENV_FILE", credentialFile)
	err := Init()
	if err == nil || !strings.Contains(err.Error(), "must not be readable") {
		t.Fatalf("Init() error = %v, want permission error", err)
	}
}

func clearConfigEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"GITCODE_MCP_ENV_FILE", "GITCODE_TOKEN", "GITCODE_ACCESS_TOKEN", "GITCODE_API_URL",
		"MCP_TRANSPORT", "MCP_SSE_PORT", "GITCODE_API_TIMEOUT", "API_TIMEOUT", "GITCODE_MAX_RESPONSE_BYTES",
	} {
		t.Setenv(key, "")
	}
}
