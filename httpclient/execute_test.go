// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendExpandsEnvironmentAndPreservesRequestFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method=%s want POST", request.Method)
		}
		if got := request.URL.Query()["tag"]; len(got) != 2 || got[0] != "one" || got[1] != "two" {
			t.Errorf("query tag=%v", got)
		}
		if got := request.Header.Values("X-Test"); len(got) != 2 || got[1] != "Ada" {
			t.Errorf("X-Test=%v", got)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer Ada" {
			t.Errorf("Authorization=%q want Bearer Ada", got)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
		}
		if string(body) != `{"name":"Ada"}` {
			t.Errorf("body=%q", body)
		}
		writer.Header().Add("X-Reply", "yes")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	response := Send(context.Background(), Request{
		Method: "POST",
		URL:    "{{base_url}}/users",
		Query: []KeyValue{
			{Name: "tag", Value: "one"},
			{Name: "tag", Value: "two"},
			{Name: "skip", Value: "no", Disabled: true},
		},
		Headers: []KeyValue{
			{Name: "X-Test", Value: "static"},
			{Name: "X-Test", Value: "{{name}}"},
		},
		Auth: Auth{Type: AuthBearer, Token: "{{name}}"},
		Body: `{"name":"{{name}}"}`,
	}, Environment{Variables: map[string]string{
		"base_url": server.URL,
		"name":     "Ada",
	}})
	if response.Err != nil {
		t.Fatalf("Send: %v", response.Err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d want 201", response.StatusCode)
	}
	if got := PrettyBody(response.Body); !strings.Contains(got, "\n  \"ok\": true\n") {
		t.Fatalf("PrettyBody=%q", got)
	}
}

func TestSendSupportsStructuredAndInheritedAuthentication(t *testing.T) {
	tests := []struct {
		name        string
		requestAuth Auth
		envAuth     Auth
		wantHeader  string
		wantQuery   string
	}{
		{
			name:        "basic",
			requestAuth: Auth{Type: AuthBasic, Username: "{{user}}", Password: "{{password}}"},
			wantHeader:  "Basic YWRhOmFuYWx5dGljYWw=",
		},
		{
			name:        "bearer",
			requestAuth: Auth{Type: AuthBearer, Token: "{{token}}"},
			wantHeader:  "Bearer secret-token",
		},
		{
			name:        "API key header",
			requestAuth: Auth{Type: AuthAPIKey, Key: "X-API-Key", Value: "{{api_key}}", In: AuthInHeader},
			wantHeader:  "api-secret",
		},
		{
			name:        "API key query",
			requestAuth: Auth{Type: AuthAPIKey, Key: "access_key", Value: "{{api_key}}", In: AuthInQuery},
			wantQuery:   "api-secret",
		},
		{
			name:        "inherit bearer from environment",
			requestAuth: Auth{Type: AuthInherit},
			envAuth:     Auth{Type: AuthBearer, Token: "{{token}}"},
			wantHeader:  "Bearer secret-token",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if test.name == "API key header" {
					if got := request.Header.Get("X-API-Key"); got != test.wantHeader {
						t.Errorf("X-API-Key=%q want %q", got, test.wantHeader)
					}
				} else if test.wantHeader != "" {
					if got := request.Header.Get("Authorization"); got != test.wantHeader {
						t.Errorf("Authorization=%q want %q", got, test.wantHeader)
					}
				}
				if test.wantQuery != "" && request.URL.Query().Get("access_key") != test.wantQuery {
					t.Errorf("access_key=%q want %q", request.URL.Query().Get("access_key"), test.wantQuery)
				}
				writer.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			response := Send(context.Background(), Request{Method: "GET", URL: server.URL, Auth: test.requestAuth}, Environment{
				Variables: map[string]string{"user": "ada", "password": "analytical", "token": "secret-token", "api_key": "api-secret"},
				Auth:      test.envAuth,
			})
			if response.Err != nil {
				t.Fatalf("Send: %v", response.Err)
			}
		})
	}
}

func TestSendRejectsNonHTTPURL(t *testing.T) {
	response := Send(context.Background(), Request{Method: "GET", URL: "file:///tmp/private"}, Environment{})
	if response.Err == nil {
		t.Fatal("Send accepted a non-HTTP URL")
	}
}

func TestExpandVariablesLeavesUnknownNamesVisible(t *testing.T) {
	got := ExpandVariables("{{known}}/{{unknown}}", map[string]string{"known": "yes"})
	if got != "yes/{{unknown}}" {
		t.Fatalf("ExpandVariables=%q", got)
	}
}
