// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const MaxResponseBytes = 4 << 20

type Response struct {
	StatusCode int
	Status     string
	Headers    []KeyValue
	Body       []byte
	Duration   time.Duration
	Size       int64
	Truncated  bool
	Err        error
}

type Sender func(context.Context, Request, Environment) Response

func Send(ctx context.Context, request Request, environment Environment) Response {
	started := time.Now()
	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if method == "" {
		method = http.MethodGet
	}
	resolvedURL := ExpandVariables(request.URL, environment.Variables)
	parsedURL, err := url.Parse(resolvedURL)
	if err != nil {
		return Response{Duration: time.Since(started), Err: fmt.Errorf("invalid URL: %w", err)}
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return Response{Duration: time.Since(started), Err: fmt.Errorf("URL must use http or https")}
	}
	query := parsedURL.Query()
	for _, item := range request.Query {
		if item.Disabled || strings.TrimSpace(item.Name) == "" {
			continue
		}
		query.Add(ExpandVariables(item.Name, environment.Variables), ExpandVariables(item.Value, environment.Variables))
	}
	parsedURL.RawQuery = query.Encode()

	body := ExpandVariables(request.Body, environment.Variables)
	httpRequest, err := http.NewRequestWithContext(ctx, method, parsedURL.String(), strings.NewReader(body))
	if err != nil {
		return Response{Duration: time.Since(started), Err: err}
	}
	for _, header := range request.Headers {
		if header.Disabled || strings.TrimSpace(header.Name) == "" {
			continue
		}
		httpRequest.Header.Add(
			ExpandVariables(header.Name, environment.Variables),
			ExpandVariables(header.Value, environment.Variables),
		)
	}
	if auth := strings.TrimSpace(ExpandVariables(request.Auth, environment.Variables)); auth != "" && httpRequest.Header.Get("Authorization") == "" {
		httpRequest.Header.Set("Authorization", auth)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return Response{Duration: time.Since(started), Err: err}
	}
	defer httpResponse.Body.Close()

	limited := io.LimitReader(httpResponse.Body, MaxResponseBytes+1)
	responseBody, readErr := io.ReadAll(limited)
	truncated := len(responseBody) > MaxResponseBytes
	if truncated {
		responseBody = responseBody[:MaxResponseBytes]
	}
	result := Response{
		StatusCode: httpResponse.StatusCode,
		Status:     httpResponse.Status,
		Headers:    responseHeaders(httpResponse.Header),
		Body:       responseBody,
		Duration:   time.Since(started),
		Size:       httpResponse.ContentLength,
		Truncated:  truncated,
		Err:        readErr,
	}
	if result.Size < 0 {
		result.Size = int64(len(responseBody))
	}
	return result
}

func ExpandVariables(value string, variables map[string]string) string {
	if value == "" || len(variables) == 0 {
		return value
	}
	return osExpand(value, func(name string) (string, bool) {
		resolved, ok := variables[name]
		return resolved, ok
	})
}

func osExpand(value string, lookup func(string) (string, bool)) string {
	var out strings.Builder
	for {
		start := strings.Index(value, "{{")
		if start < 0 {
			out.WriteString(value)
			return out.String()
		}
		endOffset := strings.Index(value[start+2:], "}}")
		if endOffset < 0 {
			out.WriteString(value)
			return out.String()
		}
		end := start + 2 + endOffset
		out.WriteString(value[:start])
		name := strings.TrimSpace(value[start+2 : end])
		if replacement, ok := lookup(name); ok {
			out.WriteString(replacement)
		} else {
			out.WriteString(value[start : end+2])
		}
		value = value[end+2:]
	}
}

func PrettyBody(body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}
	var pretty bytes.Buffer
	if json.Indent(&pretty, trimmed, "", "  ") == nil {
		return pretty.String()
	}
	return string(body)
}

func responseHeaders(headers http.Header) []KeyValue {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]KeyValue, 0, len(names))
	for _, name := range names {
		for _, value := range headers.Values(name) {
			out = append(out, KeyValue{Name: name, Value: value})
		}
	}
	return out
}
