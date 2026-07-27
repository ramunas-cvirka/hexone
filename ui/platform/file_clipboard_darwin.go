// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const readFileClipboardJXA = `
ObjC.import("AppKit");
function run() {
    const pasteboard = $.NSPasteboard.generalPasteboard;
    const options = $.NSDictionary.dictionaryWithObjectForKey(
        true,
        $.NSPasteboardURLReadingFileURLsOnlyKey
    );
    const classes = $.NSArray.arrayWithObject($.NSURL);
    const urls = pasteboard.readObjectsForClassesOptions(classes, options);
    const paths = [];
    if (urls) {
        for (let i = 0; i < urls.count; i++) {
            const value = ObjC.unwrap(urls.objectAtIndex(i).path);
            if (value) paths.push(value);
        }
    }
    return JSON.stringify(paths);
}
`

const writeFileClipboardJXA = `
ObjC.import("AppKit");
function run(argv) {
    const urls = $.NSMutableArray.array;
    argv.forEach(function(path) {
        urls.addObject($.NSURL.fileURLWithPath($(path).stringByStandardizingPath));
    });
    const pasteboard = $.NSPasteboard.generalPasteboard;
    pasteboard.clearContents;
    if (!pasteboard.writeObjects(urls)) {
        throw new Error("the system pasteboard rejected the file URLs");
    }
}
`

// ReadClipboardFiles returns local file paths published by Finder or another
// application as native file URLs.
func ReadClipboardFiles() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", "-e", readFileClipboardJXA).Output()
	if err != nil {
		return nil, fmt.Errorf("read file clipboard: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	var paths []string
	if err := json.Unmarshal([]byte(raw), &paths); err != nil {
		return nil, fmt.Errorf("decode file clipboard: %w", err)
	}
	return normalizeClipboardFilePaths(paths), nil
}

// WriteClipboardFiles publishes paths as native file URLs so Finder and other
// desktop applications can paste them as files.
func WriteClipboardFiles(paths []string) error {
	paths = normalizeClipboardFilePaths(paths)
	if len(paths) == 0 {
		return errors.New("no local files to copy")
	}
	args := []string{"-l", "JavaScript", "-e", writeFileClipboardJXA, "--"}
	args = append(args, paths...)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "osascript", args...).CombinedOutput(); err != nil {
		detail := strings.TrimSpace(string(out))
		if detail != "" {
			return fmt.Errorf("write file clipboard: %s: %w", detail, err)
		}
		return fmt.Errorf("write file clipboard: %w", err)
	}
	return nil
}
