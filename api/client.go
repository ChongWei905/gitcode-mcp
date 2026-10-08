package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gitcode-org-com/gitcode-mcp/config"
)

var (
	ErrAuthFailed       = errors.New("authentication failed")
	ErrPermissionDenied = errors.New("permission denied")
	ErrNotFound         = errors.New("resource not found")
	ErrValidation       = errors.New("request validation failed")
	ErrConflict         = errors.New("resource conflict")
	ErrRateLimit        = errors.New("API rate limit exceeded")
	ErrServer           = errors.New("GitCode server error")
	ErrUnknown          = errors.New("unknown GitCode API error")
	ErrMissingToken     = errors.New("GitCode token is missing; configure GITCODE_TOKEN in ~/.gitcode_mcp/.env")
	ErrResponseTooLarge = errors.New("GitCode API response exceeds the configured size limit")
)

// APIError describes a non-success response returned by GitCode.
type APIError struct {
	Code    int
	Message string
	Err     error
}

// Error implements the error interface without including credentials or request bodies.
func (e *APIError) Error() string {
	return fmt.Sprintf("GitCode API error [%d]: %s: %v", e.Code, e.Message, e.Err)
}

// Unwrap returns the categorized error for errors.Is checks.
func (e *APIError) Unwrap() error {
	return e.Err
}

// FlexibleID accepts identifiers serialized by GitCode as either strings or numbers.
type FlexibleID string

// UnmarshalJSON normalizes string and number identifiers to a string.
func (id *FlexibleID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*id = ""
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*id = FlexibleID(text)
		return nil
	}

	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		*id = FlexibleID(number.String())
		return nil
	}

	return fmt.Errorf("invalid GitCode identifier: %s", string(data))
}

// MarshalJSON emits identifiers as strings, matching the current GitCode schema.
func (id FlexibleID) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(id))
}

// Response is the raw result of a GitCode API call.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// GitCodeAPI is an authenticated client restricted to one GitCode API base URL.
type GitCodeAPI struct {
	Token            string
	BaseURL          string
	Timeout          time.Duration
	MaxResponseBytes int64
	CredentialFile   string
	CredentialSource string
	HTTPClient       *http.Client

	Repos    *RepositoryAPI
	Branches *BranchAPI
	Issues   *IssueAPI
	Pulls    *PullRequestAPI
	Search   *SearchAPI
}

// NewGitCodeAPI creates a client from the initialized process configuration.
func NewGitCodeAPI(token string) (*GitCodeAPI, error) {
	if token == "" {
		token = config.GlobalConfig.GitCodeToken
	}

	timeout := time.Duration(config.GlobalConfig.APITimeout) * time.Second
	client := &GitCodeAPI{
		Token:            token,
		BaseURL:          strings.TrimRight(config.GlobalConfig.GitCodeAPIURL, "/"),
		Timeout:          timeout,
		MaxResponseBytes: config.GlobalConfig.MaxResponseSize,
		CredentialFile:   config.GlobalConfig.CredentialFile,
		CredentialSource: config.GlobalConfig.CredentialSource,
	}
	client.HTTPClient = newDirectHTTPClient(client.BaseURL, timeout)

	if err := client.validate(); err != nil {
		return nil, err
	}
	client.initModules()
	return client, nil
}

func (c *GitCodeAPI) initModules() {
	c.Repos = NewRepositoryAPI(c)
	c.Branches = NewBranchAPI(c)
	c.Issues = NewIssueAPI(c)
	c.Pulls = NewPullRequestAPI(c)
	c.Search = NewSearchAPI(c)
}

func (c *GitCodeAPI) validate() error {
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return fmt.Errorf("invalid GitCode API base URL: %q", c.BaseURL)
	}
	if c.HTTPClient == nil {
		return errors.New("GitCode HTTP client is nil")
	}
	if c.MaxResponseBytes <= 0 {
		return errors.New("GitCode response limit must be greater than zero")
	}
	return nil
}

func newDirectHTTPClient(baseURL string, timeout time.Duration) *http.Client {
	base, _ := url.Parse(baseURL)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many GitCode API redirects")
			}
			if base != nil && !strings.EqualFold(req.URL.Hostname(), base.Hostname()) {
				return errors.New("refusing to forward GitCode credentials to another host")
			}
			return nil
		},
	}
}

// Do sends an authenticated, context-aware request to a relative GitCode API path.
func (c *GitCodeAPI) Do(ctx context.Context, method, path string, params url.Values, body interface{}) (*Response, error) {
	if c.Token == "" {
		return nil, ErrMissingToken
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if !isSupportedMethod(method) {
		return nil, fmt.Errorf("unsupported GitCode API method: %s", method)
	}
	if err := ValidateNoCredentialFields(params); err != nil {
		return nil, err
	}
	if err := ValidateNoCredentialFields(body); err != nil {
		return nil, err
	}

	requestURL, safePath, err := c.buildURL(path, params)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		jsonData, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, fmt.Errorf("encode GitCode request body: %w", marshalErr)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create GitCode API request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GitCode-MCP/2.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	log.Printf("GitCode API request: %s %s", method, safePath)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send GitCode API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := readLimited(resp.Body, c.MaxResponseBytes)
	if err != nil {
		return nil, err
	}
	result := &Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       respBody,
	}
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return result, nil
	}
	return nil, newAPIError(resp.StatusCode, respBody)
}

func (c *GitCodeAPI) buildURL(path string, params url.Values) (string, string, error) {
	normalizedPath, err := normalizeAPIPath(path)
	if err != nil {
		return "", "", err
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", "", fmt.Errorf("parse GitCode API base URL: %w", err)
	}
	if strings.HasSuffix(strings.TrimRight(base.Path, "/"), "/api/v5") {
		normalizedPath = strings.TrimPrefix(normalizedPath, "/api/v5")
		if normalizedPath == "" {
			normalizedPath = "/"
		}
	}

	requestURL, err := url.Parse(strings.TrimRight(c.BaseURL, "/") + normalizedPath)
	if err != nil {
		return "", "", fmt.Errorf("build GitCode API URL: %w", err)
	}
	query := requestURL.Query()
	for key, values := range params {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	requestURL.RawQuery = query.Encode()
	return requestURL.String(), requestURL.EscapedPath(), nil
}

func normalizeAPIPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasPrefix(path, "/") {
		return "", errors.New("GitCode API path must start with /")
	}
	if strings.HasPrefix(path, "//") || strings.Contains(path, "://") || strings.ContainsAny(path, "\\\r\n") {
		return "", errors.New("GitCode API path must be relative to the configured API host")
	}
	parsed, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse GitCode API path: %w", err)
	}
	if parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("put GitCode query parameters in the query object, not in path")
	}
	return parsed.EscapedPath(), nil
}

func isSupportedMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read GitCode API response: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, ErrResponseTooLarge
	}
	return data, nil
}

func newAPIError(status int, body []byte) *APIError {
	message := extractErrorMessage(body)
	var category error
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		category = ErrValidation
	case http.StatusUnauthorized:
		category = ErrAuthFailed
	case http.StatusForbidden:
		category = ErrPermissionDenied
	case http.StatusNotFound:
		category = ErrNotFound
	case http.StatusConflict, http.StatusPreconditionFailed:
		category = ErrConflict
	case http.StatusTooManyRequests:
		category = ErrRateLimit
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		category = ErrServer
	default:
		category = ErrUnknown
	}
	return &APIError{Code: status, Message: message, Err: category}
}

func extractErrorMessage(body []byte) string {
	var payload map[string]interface{}
	if json.Unmarshal(body, &payload) == nil {
		for _, key := range []string{"message", "error_description", "error"} {
			if value, ok := payload[key].(string); ok && value != "" {
				return truncate(value, 2048)
			}
		}
	}
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = "request failed without an error body"
	}
	return truncate(message, 2048)
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

// ValidateNoCredentialFields rejects caller-supplied authentication fields.
func ValidateNoCredentialFields(value interface{}) error {
	switch typed := value.(type) {
	case nil:
		return nil
	case url.Values:
		for key := range typed {
			if isCredentialKey(key) {
				return fmt.Errorf("credential field %q is managed internally by the MCP server", key)
			}
		}
	case map[string]interface{}:
		for key, child := range typed {
			if isCredentialKey(key) {
				return fmt.Errorf("credential field %q is managed internally by the MCP server", key)
			}
			if err := ValidateNoCredentialFields(child); err != nil {
				return err
			}
		}
	case map[string]string:
		for key := range typed {
			if isCredentialKey(key) {
				return fmt.Errorf("credential field %q is managed internally by the MCP server", key)
			}
		}
	case []interface{}:
		for _, child := range typed {
			if err := ValidateNoCredentialFields(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func isCredentialKey(key string) bool {
	normalized := strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.ToLower(key))
	switch normalized {
	case "token", "accesstoken", "authorization", "privatetoken", "gitcodetoken", "gitcodeaccesstoken":
		return true
	default:
		return false
	}
}

// Request preserves the original byte-oriented client API for typed modules.
func (c *GitCodeAPI) Request(method, path string, params url.Values, body interface{}) ([]byte, error) {
	response, err := c.Do(context.Background(), method, path, params, body)
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}

// GET sends an authenticated GET request.
func (c *GitCodeAPI) GET(path string, params url.Values) ([]byte, error) {
	return c.Request(http.MethodGet, path, params, nil)
}

// POST sends an authenticated POST request.
func (c *GitCodeAPI) POST(path string, params url.Values, body interface{}) ([]byte, error) {
	return c.Request(http.MethodPost, path, params, body)
}

// PUT sends an authenticated PUT request.
func (c *GitCodeAPI) PUT(path string, params url.Values, body interface{}) ([]byte, error) {
	return c.Request(http.MethodPut, path, params, body)
}

// DELETE sends an authenticated DELETE request.
func (c *GitCodeAPI) DELETE(path string, params url.Values) ([]byte, error) {
	return c.Request(http.MethodDelete, path, params, nil)
}

// PATCH sends an authenticated PATCH request.
func (c *GitCodeAPI) PATCH(path string, params url.Values, body interface{}) ([]byte, error) {
	return c.Request(http.MethodPatch, path, params, body)
}

// BaseAPI is embedded by typed GitCode API modules.
type BaseAPI struct {
	Client *GitCodeAPI
}

type RepositoryAPI struct{ BaseAPI }
type BranchAPI struct{ BaseAPI }
type IssueAPI struct{ BaseAPI }
type PullRequestAPI struct{ BaseAPI }
type SearchAPI struct{ BaseAPI }

func NewRepositoryAPI(client *GitCodeAPI) *RepositoryAPI {
	return &RepositoryAPI{BaseAPI{Client: client}}
}

func NewBranchAPI(client *GitCodeAPI) *BranchAPI {
	return &BranchAPI{BaseAPI{Client: client}}
}

func NewIssueAPI(client *GitCodeAPI) *IssueAPI {
	return &IssueAPI{BaseAPI{Client: client}}
}

func NewPullRequestAPI(client *GitCodeAPI) *PullRequestAPI {
	return &PullRequestAPI{BaseAPI{Client: client}}
}

func NewSearchAPI(client *GitCodeAPI) *SearchAPI {
	return &SearchAPI{BaseAPI{Client: client}}
}
