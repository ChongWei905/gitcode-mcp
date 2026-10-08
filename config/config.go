package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultAPIURL           = "https://api.gitcode.com/api/v5"
	defaultAPITimeout       = 30
	defaultMaxResponseBytes = int64(4 * 1024 * 1024)
)

// Config contains the runtime settings used by the GitCode MCP server.
type Config struct {
	GitCodeToken    string
	GitCodeAPIURL   string
	APITimeout      int
	MaxResponseSize int64

	MCPTransport string
	MCPSSEPort   int

	CredentialFile   string
	CredentialSource string
}

var defaultConfig = Config{
	GitCodeAPIURL:   defaultAPIURL,
	MCPTransport:    "stdio",
	MCPSSEPort:      8000,
	APITimeout:      defaultAPITimeout,
	MaxResponseSize: defaultMaxResponseBytes,
}

// GlobalConfig is initialized once during process startup.
var GlobalConfig = defaultConfig

// Init loads configuration with process environment variables taking priority
// over the stable credentials file at ~/.gitcode_mcp/.env.
func Init() error {
	fileValues, credentialFile, err := readCredentialFile()
	if err != nil {
		return err
	}

	cfg := defaultConfig
	cfg.CredentialFile = credentialFile
	cfg.GitCodeToken, cfg.CredentialSource = resolveSecret(fileValues,
		"GITCODE_TOKEN",
		"GITCODE_ACCESS_TOKEN",
	)
	cfg.GitCodeAPIURL = resolveValue(fileValues, cfg.GitCodeAPIURL, "GITCODE_API_URL")
	cfg.MCPTransport = strings.ToLower(resolveValue(fileValues, cfg.MCPTransport, "MCP_TRANSPORT"))

	if value := resolveValue(fileValues, "", "MCP_SSE_PORT"); value != "" {
		port, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			return fmt.Errorf("invalid MCP_SSE_PORT: %w", parseErr)
		}
		cfg.MCPSSEPort = port
	}

	if value := resolveValue(fileValues, "", "GITCODE_API_TIMEOUT", "API_TIMEOUT"); value != "" {
		timeout, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			return fmt.Errorf("invalid GITCODE_API_TIMEOUT: %w", parseErr)
		}
		cfg.APITimeout = timeout
	}

	if value := resolveValue(fileValues, "", "GITCODE_MAX_RESPONSE_BYTES"); value != "" {
		limit, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid GITCODE_MAX_RESPONSE_BYTES: %w", parseErr)
		}
		cfg.MaxResponseSize = limit
	}

	if err := validateConfig(cfg); err != nil {
		return err
	}

	GlobalConfig = cfg
	return nil
}

func readCredentialFile() (map[string]string, string, error) {
	path := strings.TrimSpace(os.Getenv("GITCODE_MCP_ENV_FILE"))
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, "", fmt.Errorf("resolve home directory: %w", err)
		}
		path = filepath.Join(home, ".gitcode_mcp", ".env")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, "", fmt.Errorf("resolve GitCode credential file: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, absPath, nil
		}
		return nil, absPath, fmt.Errorf("inspect GitCode credential file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, absPath, fmt.Errorf("GitCode credential path is not a regular file: %s", absPath)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, absPath, fmt.Errorf("GitCode credential file must not be readable by group or others: %s", absPath)
	}

	values, err := godotenv.Read(absPath)
	if err != nil {
		return nil, absPath, fmt.Errorf("read GitCode credential file: %w", err)
	}
	return values, absPath, nil
}

func resolveSecret(fileValues map[string]string, keys ...string) (string, string) {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value, "environment"
		}
	}
	for _, key := range keys {
		if value := strings.TrimSpace(fileValues[key]); value != "" {
			return value, "credential_file"
		}
	}
	return "", "missing"
}

func resolveValue(fileValues map[string]string, fallback string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	for _, key := range keys {
		if value := strings.TrimSpace(fileValues[key]); value != "" {
			return value
		}
	}
	return fallback
}

func validateConfig(cfg Config) error {
	parsedURL, err := url.Parse(cfg.GitCodeAPIURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("invalid GITCODE_API_URL: %q", cfg.GitCodeAPIURL)
	}
	if parsedURL.Scheme != "https" && !isLoopbackHost(parsedURL.Hostname()) {
		return fmt.Errorf("GITCODE_API_URL must use HTTPS outside loopback hosts")
	}
	if cfg.APITimeout <= 0 {
		return fmt.Errorf("GITCODE_API_TIMEOUT must be greater than zero")
	}
	if cfg.MaxResponseSize <= 0 {
		return fmt.Errorf("GITCODE_MAX_RESPONSE_BYTES must be greater than zero")
	}
	if cfg.MCPTransport != "stdio" && cfg.MCPTransport != "sse" {
		return fmt.Errorf("MCP_TRANSPORT must be stdio or sse")
	}
	if cfg.MCPTransport == "sse" && (cfg.MCPSSEPort <= 0 || cfg.MCPSSEPort > 65535) {
		return fmt.Errorf("MCP_SSE_PORT must be between 1 and 65535")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
