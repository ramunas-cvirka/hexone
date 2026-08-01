// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package httpclient

import "strings"

const CurrentVersion = 1

type File struct {
	Version      int           `yaml:"version"`
	Environments []Environment `yaml:"environments,omitempty"`
	Collections  []Collection  `yaml:"collections,omitempty"`
}

type Environment struct {
	Name      string            `yaml:"name"`
	Variables map[string]string `yaml:"variables,omitempty"`
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
	Auth    string     `yaml:"auth,omitempty"`
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
	if f.Version <= 0 {
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
	}
}
