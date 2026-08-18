// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

// Command unusedcheck reports unexported declarations that no build of hexone
// can reach.
//
// staticcheck only ever sees one build configuration at a time, and this
// repository has per-OS files and two build tags. A symbol that looks dead on
// one configuration is regularly the only caller on another: ui/platform is
// full of _windows/_darwin/_linux siblings, the pdfium tag swaps out the whole
// PDF backend, and uiverify adds headless test drivers. Trusting a single run
// means deleting live code and breaking a platform nobody built locally.
//
// So run every configuration and report only what every one of them calls
// dead. Two things stop that from being naive:
//
//   - No machine can analyze every configuration. Gio's Linux backend needs
//     cgo, so GOOS=linux cannot be type-checked from macOS, and GOOS=darwin
//     cannot be type-checked from Linux. A configuration that fails to compile
//     is skipped, never treated as "found nothing" — an empty result would
//     wrongly widen the intersection and flag live code.
//   - Because configurations get skipped, symbols that live only on an
//     unanalyzable platform still surface. Those go in allowlist.txt with the
//     reason they are exempt, which doubles as documentation of every symbol
//     that exists for one platform alone.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const staticcheckPkg = "honnef.co/go/tools/cmd/staticcheck@v0.7.0"

type config struct {
	name string
	goos string
	tags string
	// pattern is the package pattern to analyze, and prefix is the repo-relative
	// path it covers. A configuration only constrains findings it can see.
	pattern string
	prefix  string
}

// configs covers each shipped OS plus each build tag combination. The tag
// entries run on the host OS; between a developer's machine and CI every
// platform gets analyzed natively. The pdfium entries are scoped to ui/ because
// cmd/hexone-pdfium-worker needs a native pdfium library that is rarely present.
var configs = []config{
	{name: "darwin", goos: "darwin", pattern: "./...", prefix: ""},
	{name: "linux", goos: "linux", pattern: "./...", prefix: ""},
	{name: "windows", goos: "windows", pattern: "./...", prefix: ""},
	{name: "uiverify", tags: "uiverify", pattern: "./...", prefix: ""},
	{name: "pdfium", tags: "pdfium", pattern: "./ui/...", prefix: "ui/"},
	{name: "pdfium+uiverify", tags: "pdfium uiverify", pattern: "./ui/...", prefix: "ui/"},
}

// finding is one reported declaration, keyed without its line:column so the
// same symbol matches across configurations even if positions shift.
type finding struct {
	key  string
	file string
	text string
}

var positionRe = regexp.MustCompile(`:\d+:\d+:`)

func main() {
	verbose := flag.Bool("v", false, "list the findings of every configuration")
	allowPath := flag.String("allowlist", "", "path to the allowlist (default: alongside this tool)")
	flag.Parse()

	bin, err := exec.LookPath("staticcheck")
	if err != nil {
		fmt.Fprintf(os.Stderr, "staticcheck not found on PATH; install it with:\n\tgo install %s\n", staticcheckPkg)
		os.Exit(2)
	}

	results := make(map[string]map[string]finding)
	var analyzed, skipped []string
	for _, cfg := range configs {
		found, ok := run(bin, cfg)
		if !ok {
			skipped = append(skipped, cfg.name)
			fmt.Printf("skip  %-16s sources could not be type-checked here\n", cfg.name)
			continue
		}
		analyzed = append(analyzed, cfg.name)
		results[cfg.name] = found
		fmt.Printf("ran   %-16s %d unused in %s\n", cfg.name, len(found), cfg.pattern)
		if *verbose {
			for _, f := range sorted(found) {
				fmt.Printf("        %s\n", f.text)
			}
		}
	}
	if len(analyzed) == 0 {
		fmt.Fprintln(os.Stderr, "no configuration could be analyzed")
		os.Exit(2)
	}

	dead := intersect(results)

	allow, err := loadAllowlist(*allowPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "allowlist: %v\n", err)
		os.Exit(2)
	}
	var reported []finding
	used := make(map[string]bool)
	for _, f := range dead {
		if reason, ok := allow[f.key]; ok {
			used[f.key] = true
			if *verbose {
				fmt.Printf("allow %s (%s)\n", f.key, reason)
			}
			continue
		}
		reported = append(reported, f)
	}

	fmt.Printf("\nanalyzed %d configuration(s): %s\n", len(analyzed), strings.Join(analyzed, ", "))
	if len(skipped) > 0 {
		fmt.Printf("skipped %d: %s — run this there to cover them\n", len(skipped), strings.Join(skipped, ", "))
	}
	if n := len(allow) - len(used); n > 0 && len(skipped) == 0 {
		fmt.Printf("note: %d allowlist entr(ies) matched nothing; they may be stale\n", n)
	}

	if len(reported) == 0 {
		fmt.Println("\nno declaration is unused in every configuration that can see it")
		return
	}
	fmt.Printf("\n%d declaration(s) unreachable in every configuration that can see them:\n\n", len(reported))
	for _, f := range reported {
		fmt.Printf("  %s\n", f.text)
	}
	fmt.Print(`
Either remove them, or — if one is the sole caller on a platform this run could
not analyze — add it to tools/unusedcheck/allowlist.txt with the reason.
`)
	os.Exit(1)
}

// intersect keeps findings that every configuration able to see them reported.
// Scoping matters: a ui/-only configuration must not vouch for protocols/.
func intersect(results map[string]map[string]finding) []finding {
	prefixes := make(map[string]string, len(configs))
	for _, cfg := range configs {
		prefixes[cfg.name] = cfg.prefix
	}
	reportedBy := make(map[string]int)
	texts := make(map[string]finding)
	for _, found := range results {
		for key, f := range found {
			reportedBy[key]++
			texts[key] = f
		}
	}
	var out []finding
	for key, f := range texts {
		visibleTo := 0
		for name := range results {
			if strings.HasPrefix(f.file, prefixes[name]) {
				visibleTo++
			}
		}
		if reportedBy[key] == visibleTo {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}

// run analyzes one configuration. It reports ok=false when staticcheck could
// not compile the sources, which makes its findings meaningless to intersect.
func run(bin string, cfg config) (map[string]finding, bool) {
	args := []string{"-checks", "U1000"}
	if cfg.tags != "" {
		args = append(args, "-tags", cfg.tags)
	}
	args = append(args, cfg.pattern)

	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	if cfg.goos != "" {
		cmd.Env = append(cmd.Env, "GOOS="+cfg.goos)
	}
	out, _ := cmd.Output() // a non-zero exit just means findings exist

	found := make(map[string]finding)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasSuffix(line, "(compile)") {
			return nil, false
		}
		if !strings.HasSuffix(line, "(U1000)") {
			continue
		}
		key := positionRe.ReplaceAllString(line, ":")
		found[key] = finding{key: key, file: line[:strings.Index(line, ":")], text: line}
	}
	return found, true
}

func loadAllowlist(path string) (map[string]string, error) {
	if path == "" {
		path = filepath.Join("tools", "unusedcheck", "allowlist.txt")
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	allow := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entry, reason, _ := strings.Cut(line, "#")
		allow[strings.TrimSpace(entry)] = strings.TrimSpace(reason)
	}
	return allow, nil
}

func sorted(m map[string]finding) []finding {
	out := make([]finding, 0, len(m))
	for _, f := range m {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key < out[j].key })
	return out
}
