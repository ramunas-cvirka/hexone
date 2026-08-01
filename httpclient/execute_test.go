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
		Auth: "Bearer {{name}}",
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
