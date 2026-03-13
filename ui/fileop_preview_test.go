// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"reflect"
	"testing"
)

func TestFileOpPreviewLines(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "three or fewer",
			in:   []string{"alpha", "beta", "gamma"},
			want: []string{"alpha", "beta", "gamma"},
		},
		{
			name: "four items",
			in:   []string{"alpha", "beta", "gamma", "omega"},
			want: []string{"alpha", "beta", "...", "omega"},
		},
		{
			name: "filters blanks",
			in:   []string{"alpha", "", "beta", "gamma", "omega"},
			want: []string{"alpha", "beta", "...", "omega"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileOpPreviewLines(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("fileOpPreviewLines(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}
