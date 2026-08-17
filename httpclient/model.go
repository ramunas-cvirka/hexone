// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"encoding/base64"
	"strings"

	"go.yaml.in/yaml/v4"
)

const CurrentVersion = 3

const (
	AuthNone    = "none"
	AuthInherit = "inherit"
	AuthBasic   = "basic"
	AuthBearer  = "bearer"
	AuthAPIKey  = "api-key"
	AuthRaw     = "raw"

	AuthInHeader = "header"
	AuthInQuery  = "query"
)

type File struct {
	Version      int           `yaml:"version"`
	Environments []Environment `yaml:"environments,omitempty"`
	Collections  []Collection  `yaml:"collections,omitempty"`
}

type Environment struct {
	Name                  string            `yaml:"name"`
	Variables             map[string]string `yaml:"variables,omitempty"`
	VariablesCredentialID string            `yaml:"variables_credential_id,omitempty"`
	Auth                  Auth              `yaml:"auth,omitempty"`
}

// Auth describes request authentication. Values may contain {{environment}}
// templates. Request auth may use the inherit type; environment auth may not.
type Auth struct {
	Type         string `yaml:"type,omitempty"`
	Username     string `yaml:"username,omitempty"`
	Password     string `yaml:"password,omitempty"`
	Token        string `yaml:"token,omitempty"`
	Key          string `yaml:"key,omitempty"`
	Value        string `yaml:"value,omitempty"`
	In           string `yaml:"in,omitempty"`
	CredentialID string `yaml:"credential_id,omitempty"`
}

func (a Auth) IsZero() bool {
	return normalizeAuthType(a.Type) == AuthNone && a.Username == "" && a.Password == "" && a.Token == "" && a.Key == "" && a.Value == ""
}

// UnmarshalYAML keeps version-1 collection files compatible. Those files used
// a scalar Authorization value such as "Bearer {{token}}".
func (a *Auth) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*a = parseLegacyAuth(node.Value)
		return nil
	}
	type plainAuth Auth
	var decoded plainAuth
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	*a = Auth(decoded)
	a.Normalize(true)
	return nil
}

func parseLegacyAuth(value string) Auth {
	value = strings.TrimSpace(value)
	if value == "" {
		return Auth{}
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "bearer ") {
		return Auth{Type: AuthBearer, Token: strings.TrimSpace(value[len("bearer "):])}
	}
	if strings.HasPrefix(lower, "basic ") {
		encoded := strings.TrimSpace(value[len("basic "):])
		if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
			if username, password, ok := strings.Cut(string(decoded), ":"); ok {
				return Auth{Type: AuthBasic, Username: username, Password: password}
			}
		}
	}
	return Auth{Type: AuthRaw, Value: value}
}

func normalizeAuthType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", AuthNone:
		return AuthNone
	case AuthInherit, "environment", "env":
		return AuthInherit
	case AuthBasic:
		return AuthBasic
	case AuthBearer:
		return AuthBearer
	case AuthAPIKey, "api_key", "apikey":
		return AuthAPIKey
	case AuthRaw:
		return AuthRaw
	default:
		return AuthNone
	}
}

func (a *Auth) Normalize(allowInherit bool) {
	if a == nil {
		return
	}
	a.Type = normalizeAuthType(a.Type)
	if a.Type == AuthInherit && !allowInherit {
		a.Type = AuthNone
	}
	if a.Type == AuthAPIKey {
		switch strings.ToLower(strings.TrimSpace(a.In)) {
		case AuthInQuery:
			a.In = AuthInQuery
		default:
			a.In = AuthInHeader
		}
	} else {
		a.In = ""
	}
}

type Collection struct {
	ID       string    `yaml:"id,omitempty"`
	Name     string    `yaml:"name"`
	Folders  []Folder  `yaml:"folders,omitempty"`
	Requests []Request `yaml:"requests,omitempty"`
}

type Folder struct {
	Name     string    `yaml:"name"`
	Requests []Request `yaml:"requests,omitempty"`
}

type Request struct {
	ID      string     `yaml:"id,omitempty"`
	Name    string     `yaml:"name"`
	Method  string     `yaml:"method"`
	URL     string     `yaml:"url"`
	Query   []KeyValue `yaml:"query,omitempty"`
	Headers []KeyValue `yaml:"headers,omitempty"`
	Auth    Auth       `yaml:"auth,omitempty"`
	Body    string     `yaml:"body,omitempty"`
}

type KeyValue struct {
	Name     string `yaml:"name"`
	Value    string `yaml:"value,omitempty"`
	Disabled bool   `yaml:"disabled,omitempty"`
}

func DefaultFile() *File {
	return &File{
		Version: CurrentVersion,
		Environments: []Environment{
			{
				Name: "local-dev",
				Variables: map[string]string{
					"base_url": "http://localhost:8080",
				},
			},
		},
		Collections: []Collection{
			{
				ID:   "starter",
				Name: "Starter API",
				Folders: []Folder{
					{
						Name: "Examples",
						Requests: []Request{
							{
								ID:     "health",
								Name:   "Health check",
								Method: "GET",
								URL:    "{{base_url}}/health",
								Headers: []KeyValue{
									{Name: "Accept", Value: "application/json"},
								},
							},
							{
								ID:     "create-user",
								Name:   "Create user",
								Method: "POST",
								URL:    "{{base_url}}/v1/users",
								Headers: []KeyValue{
									{Name: "Content-Type", Value: "application/json"},
									{Name: "Accept", Value: "application/json"},
								},
								Body: "{\n  \"name\": \"Ada Lovelace\",\n  \"email\": \"ada@example.test\"\n}",
							},
						},
					},
				},
			},
		},
	}
}

func (f *File) Normalize() {
	if f == nil {
		return
	}
	if f.Version < CurrentVersion {
		f.Version = CurrentVersion
	}
	for collectionIndex := range f.Collections {
		collection := &f.Collections[collectionIndex]
		collection.Name = strings.TrimSpace(collection.Name)
		if collection.Name == "" {
			collection.Name = "Untitled collection"
		}
		normalizeRequests(collection.Requests)
		for folderIndex := range collection.Folders {
			folder := &collection.Folders[folderIndex]
			folder.Name = strings.TrimSpace(folder.Name)
			if folder.Name == "" {
				folder.Name = "Untitled folder"
			}
			normalizeRequests(folder.Requests)
		}
	}
	for environmentIndex := range f.Environments {
		environment := &f.Environments[environmentIndex]
		environment.Name = strings.TrimSpace(environment.Name)
		if environment.Name == "" {
			environment.Name = "environment"
		}
		if environment.Variables == nil {
			environment.Variables = map[string]string{}
		}
		environment.Auth.Normalize(false)
	}
}

func normalizeRequests(requests []Request) {
	for i := range requests {
		request := &requests[i]
		request.Name = strings.TrimSpace(request.Name)
		if request.Name == "" {
			request.Name = "Untitled request"
		}
		request.Method = strings.ToUpper(strings.TrimSpace(request.Method))
		if request.Method == "" {
			request.Method = "GET"
		}
		request.URL = strings.TrimSpace(request.URL)
		request.Auth.Normalize(true)
	}
}
