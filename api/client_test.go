package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDoInjectsCredentialAndNormalizesAPIVersionPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/user" {
			t.Errorf("path = %q, want /api/v5/user", r.URL.Path)
		}
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("page = %q, want 2", r.URL.Query().Get("page"))
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-secret" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"login":"tester"}`))
	}))
	defer server.Close()

	client := testClient(server, 1024)
	response, err := client.Do(context.Background(), http.MethodGet, "/api/v5/user", url.Values{"page": {"2"}}, nil)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestDoRejectsHostEscapeAndCallerCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("server must not receive rejected requests")
	}))
	defer server.Close()
	client := testClient(server, 1024)

	if _, err := client.Do(context.Background(), http.MethodGet, "https://example.com/user", nil, nil); err == nil {
		t.Fatal("absolute URL was accepted")
	}
	if _, err := client.Do(context.Background(), http.MethodGet, "/user", url.Values{"access_token": {"secret"}}, nil); err == nil {
		t.Fatal("access_token query was accepted")
	}
	if _, err := client.Do(context.Background(), http.MethodPost, "/hooks", nil, map[string]interface{}{"private_token": "secret"}); err == nil {
		t.Fatal("private_token body was accepted")
	}
}

func TestDoEnforcesResponseLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 17)))
	}))
	defer server.Close()
	client := testClient(server, 16)

	_, err := client.Do(context.Background(), http.MethodGet, "/large", nil, nil)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("Do() error = %v, want ErrResponseTooLarge", err)
	}
}

func TestIssueAcceptsStringAndNumericIdentifiers(t *testing.T) {
	for _, payload := range []string{
		`{"id":"abc","number":"19","title":"string id"}`,
		`{"id":123,"number":20,"title":"numeric id"}`,
	} {
		var issue Issue
		if err := json.Unmarshal([]byte(payload), &issue); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v", payload, err)
		}
		if issue.ID == "" || issue.Number == "" {
			t.Fatalf("normalized identifiers are empty for %s", payload)
		}
	}
}

func testClient(server *httptest.Server, maxResponseBytes int64) *GitCodeAPI {
	client := &GitCodeAPI{
		Token:            "test-secret",
		BaseURL:          server.URL + "/api/v5",
		MaxResponseBytes: maxResponseBytes,
		HTTPClient:       server.Client(),
	}
	client.initModules()
	return client
}
