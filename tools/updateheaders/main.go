// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	headerRE = regexp.MustCompile(`\A(?:// Copyright \d{4} Ramunas Cvirka(?:\. All rights reserved\.)?\n(?:\/\/ Use of this source code is governed by an Apache 2\.0 license\s*\n\/\/ that can be found in the LICENSE file\.\n|// SPDX-License-Identifier: Apache-2.0\n)\n)`)
)

func main() {
	year := strings.TrimSpace(os.Getenv("HEXONE_COPYRIGHT_YEAR"))
	if year == "" {
		year = fmt.Sprintf("%d", time.Now().Year())
	}
	header := []byte(fmt.Sprintf(
		"// Copyright %s Ramunas Cvirka\n"+
			"// SPDX-License-Identifier: Apache-2.0\n\n",
		year,
	))

	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}

	var updated int
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".cache", ".git", "dist", "third_party", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		changed, err := applyHeader(path, header)
		if err != nil {
			return err
		}
		if changed {
			updated++
		}
		return nil
	})
	if err != nil {
		fail(err)
	}

	fmt.Printf("updated headers in %d Go files\n", updated)
}

func applyHeader(path string, header []byte) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if isGeneratedGoFile(data) {
		return false, nil
	}

	trimmed := headerRE.ReplaceAll(data, nil)
	if bytes.HasPrefix(trimmed, header) {
		return false, nil
	}
	out := append(append([]byte(nil), header...), trimmed...)
	if bytes.Equal(out, data) {
		return false, nil
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

func isGeneratedGoFile(data []byte) bool {
	lines := bytes.SplitN(data, []byte{'\n'}, 8)
	for _, line := range lines {
		if bytes.Contains(line, []byte("Code generated")) && bytes.Contains(line, []byte("DO NOT EDIT")) {
			return true
		}
	}
	return false
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
