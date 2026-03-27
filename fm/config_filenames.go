// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package fm

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	FilenameIconRecent   = "recent"
	FilenameIconDocument = "document"
	FilenameIconCode     = "code"
	FilenameIconImage    = "image"
	FilenameIconVideo    = "video"
	FilenameIconAudio    = "audio"
	FilenameIconBook     = "book"
	FilenameIconTable    = "table"
	FilenameIconArchive  = "archive"
	FilenameIconApp      = "app"
	FilenameIconLink     = "link"
	FilenameIconLocked   = "locked"
)

const (
	FilenamePermissionMatchExact = ""
	FilenamePermissionMatchAll   = "all"
	FilenamePermissionMatchAny   = "any"
	FilenamePermissionMatchNone  = "none"
)

const (
	FilenameSizeMatchAtLeast = ""
	FilenameSizeMatchAtMost  = "at_most"
)

type FilenameColorsConfig struct {
	Text            string                   `yaml:"text,omitempty"`
	Icon            string                   `yaml:"icon,omitempty"`
	AgeRules        []FilenameAgeRule        `yaml:"age_rules,omitempty"`
	PermissionRules []FilenamePermissionRule `yaml:"permission_rules,omitempty"`
	ExtensionRules  []FilenameExtensionRule  `yaml:"extension_rules,omitempty"`
	SizeRules       []FilenameSizeRule       `yaml:"size_rules,omitempty"`
}

type FilenameAgeRule struct {
	MaxAge string `yaml:"max_age,omitempty"`
	Text   string `yaml:"text,omitempty"`
	Icon   string `yaml:"icon,omitempty"`
}

type FilenamePermissionRule struct {
	Permissions string `yaml:"permissions,omitempty"`
	Match       string `yaml:"match,omitempty"`
	Text        string `yaml:"text,omitempty"`
	Icon        string `yaml:"icon,omitempty"`
}

type FilenameExtensionRule struct {
	Extension string `yaml:"extension,omitempty"`
	Text      string `yaml:"text,omitempty"`
	Icon      string `yaml:"icon,omitempty"`
}

type FilenameSizeRule struct {
	Size  string `yaml:"size,omitempty"`
	Match string `yaml:"match,omitempty"`
	Text  string `yaml:"text,omitempty"`
	Icon  string `yaml:"icon,omitempty"`
}

func NormalizeFilenameIcon(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "default", "inherit", "file":
		return ""
	case "recent", "schedule", "time", "clock":
		return FilenameIconRecent
	case "document", "doc", "description", "text":
		return FilenameIconDocument
	case "code", "edit", "script":
		return FilenameIconCode
	case "image", "photo", "picture":
		return FilenameIconImage
	case "video", "movie", "media", "film":
		return FilenameIconVideo
	case "audio", "music", "album", "sound":
		return FilenameIconAudio
	case "book", "manual", "read":
		return FilenameIconBook
	case "table", "grid", "sheet", "spreadsheet":
		return FilenameIconTable
	case "archive", "zip", "compressed":
		return FilenameIconArchive
	case "app", "application", "binary", "executable", "program":
		return FilenameIconApp
	case "link", "shortcut", "symlink", "alias":
		return FilenameIconLink
	case "locked", "lock", "private":
		return FilenameIconLocked
	default:
		return ""
	}
}

func ParseFilenameAge(raw string) (time.Duration, bool) {
	txt := strings.ToLower(strings.TrimSpace(raw))
	if txt == "" || len(txt) < 2 {
		return 0, false
	}
	unit := txt[len(txt)-1]
	countText := strings.TrimSpace(txt[:len(txt)-1])
	count, err := strconv.Atoi(countText)
	if err != nil || count <= 0 {
		return 0, false
	}
	switch unit {
	case 'm':
		return time.Duration(count) * time.Minute, true
	case 'h':
		return time.Duration(count) * time.Hour, true
	case 'd':
		return time.Duration(count) * 24 * time.Hour, true
	case 'w':
		return time.Duration(count) * 7 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

func NormalizeFilenameAge(raw string) (string, bool) {
	dur, ok := ParseFilenameAge(raw)
	if !ok || dur <= 0 {
		return "", false
	}
	switch {
	case dur%(7*24*time.Hour) == 0:
		return strconv.FormatInt(int64(dur/(7*24*time.Hour)), 10) + "w", true
	case dur%(24*time.Hour) == 0:
		return strconv.FormatInt(int64(dur/(24*time.Hour)), 10) + "d", true
	case dur%time.Hour == 0:
		return strconv.FormatInt(int64(dur/time.Hour), 10) + "h", true
	default:
		return strconv.FormatInt(int64(dur/time.Minute), 10) + "m", true
	}
}

func NormalizeFilenamePermissions(raw string) string {
	txt := strings.TrimSpace(raw)
	if txt == "" {
		return ""
	}
	if strings.HasPrefix(txt, "0o") || strings.HasPrefix(txt, "0O") {
		txt = txt[2:]
	}
	for _, r := range txt {
		if r < '0' || r > '7' {
			return ""
		}
	}
	switch len(txt) {
	case 3:
		return "0" + txt
	case 4:
		return txt
	default:
		return ""
	}
}

func ParseFilenamePermissions(raw string) (uint16, bool) {
	perm := NormalizeFilenamePermissions(raw)
	if perm == "" {
		return 0, false
	}
	v, err := strconv.ParseUint(perm, 8, 16)
	if err != nil {
		return 0, false
	}
	return uint16(v), true
}

func NormalizeFilenamePermissionMatch(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "exact", "equals", "equal", "=":
		return FilenamePermissionMatchExact
	case "all", "has_all", "contains_all":
		return FilenamePermissionMatchAll
	case "any", "has_any", "contains_any", "partial":
		return FilenamePermissionMatchAny
	case "none", "without", "has_none", "contains_none", "not":
		return FilenamePermissionMatchNone
	default:
		return FilenamePermissionMatchExact
	}
}

func FilenamePermissionMatches(actualRaw string, rule FilenamePermissionRule) bool {
	actual, ok := ParseFilenamePermissions(actualRaw)
	if !ok {
		return false
	}
	want, ok := ParseFilenamePermissions(rule.Permissions)
	if !ok {
		return false
	}
	match := NormalizeFilenamePermissionMatch(rule.Match)
	switch match {
	case FilenamePermissionMatchAll:
		return actual&want == want
	case FilenamePermissionMatchAny:
		return actual&want != 0
	case FilenamePermissionMatchNone:
		return actual&want == 0
	default:
		return actual == want
	}
}

func NormalizeFilenameExtension(raw string) string {
	txt := strings.ToLower(strings.TrimSpace(raw))
	if txt == "" {
		return ""
	}
	if strings.HasPrefix(txt, "*.") {
		txt = txt[1:]
	} else if strings.HasPrefix(txt, "*") {
		txt = strings.TrimPrefix(txt, "*")
	}
	txt = strings.TrimSpace(txt)
	if txt == "" || txt == "." {
		return ""
	}
	if strings.ContainsAny(txt, `/\:`) {
		return ""
	}
	if !strings.HasPrefix(txt, ".") {
		txt = "." + txt
	}
	return txt
}

func ParseFilenameSize(raw string) (int64, bool) {
	txt := strings.ToLower(strings.TrimSpace(raw))
	if txt == "" {
		return 0, false
	}
	if strings.HasSuffix(txt, "ib") && len(txt) > 2 {
		txt = txt[:len(txt)-2]
	}
	suffix := ""
	for len(txt) > 0 {
		last := txt[len(txt)-1]
		if last < 'a' || last > 'z' {
			break
		}
		suffix = string(last) + suffix
		txt = txt[:len(txt)-1]
	}
	txt = strings.TrimSpace(txt)
	if txt == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(txt, 10, 64)
	if err != nil || value < 0 {
		return 0, false
	}

	multiplier := int64(1)
	switch suffix {
	case "", "b", "byte", "bytes":
		multiplier = 1
	case "k", "kb":
		multiplier = 1 << 10
	case "m", "mb":
		multiplier = 1 << 20
	case "g", "gb":
		multiplier = 1 << 30
	case "t", "tb":
		multiplier = 1 << 40
	default:
		return 0, false
	}
	if value > math.MaxInt64/multiplier {
		return 0, false
	}
	return value * multiplier, true
}

func NormalizeFilenameSize(raw string) (string, bool) {
	size, ok := ParseFilenameSize(raw)
	if !ok {
		return "", false
	}
	switch {
	case size%(1<<40) == 0 && size >= 1<<40:
		return strconv.FormatInt(size/(1<<40), 10) + "t", true
	case size%(1<<30) == 0 && size >= 1<<30:
		return strconv.FormatInt(size/(1<<30), 10) + "g", true
	case size%(1<<20) == 0 && size >= 1<<20:
		return strconv.FormatInt(size/(1<<20), 10) + "m", true
	case size%(1<<10) == 0 && size >= 1<<10:
		return strconv.FormatInt(size/(1<<10), 10) + "k", true
	default:
		return strconv.FormatInt(size, 10) + "b", true
	}
}

func NormalizeFilenameSizeMatch(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "at_most", "at-most", "at most", "<=", "lte", "max", "smaller", "smaller_or_equal":
		return FilenameSizeMatchAtMost
	default:
		return FilenameSizeMatchAtLeast
	}
}

func FilenameSizeMatches(sizeBytes int64, rule FilenameSizeRule) bool {
	limit, ok := ParseFilenameSize(rule.Size)
	if !ok {
		return false
	}
	switch NormalizeFilenameSizeMatch(rule.Match) {
	case FilenameSizeMatchAtMost:
		return sizeBytes <= limit
	default:
		return sizeBytes >= limit
	}
}

func NormalizeFilenameAgeRules(raw []FilenameAgeRule) []FilenameAgeRule {
	if len(raw) == 0 {
		return nil
	}
	byAge := make(map[string]FilenameAgeRule, len(raw))
	order := make([]string, 0, len(raw))
	for _, item := range raw {
		maxAge, ok := NormalizeFilenameAge(item.MaxAge)
		if !ok {
			continue
		}
		text := NormalizeOptionalHexColor(item.Text)
		icon := NormalizeFilenameIcon(item.Icon)
		if text == "" && icon == "" {
			continue
		}
		if _, exists := byAge[maxAge]; !exists {
			order = append(order, maxAge)
		}
		byAge[maxAge] = FilenameAgeRule{
			MaxAge: maxAge,
			Text:   text,
			Icon:   icon,
		}
	}
	if len(order) == 0 {
		return nil
	}
	sort.SliceStable(order, func(i, j int) bool {
		left, _ := ParseFilenameAge(order[i])
		right, _ := ParseFilenameAge(order[j])
		return left < right
	})
	out := make([]FilenameAgeRule, 0, len(order))
	for _, maxAge := range order {
		out = append(out, byAge[maxAge])
	}
	return out
}

func NormalizeFilenamePermissionRules(raw []FilenamePermissionRule) []FilenamePermissionRule {
	if len(raw) == 0 {
		return nil
	}
	byKey := make(map[string]FilenamePermissionRule, len(raw))
	order := make([]string, 0, len(raw))
	for _, item := range raw {
		perm := NormalizeFilenamePermissions(item.Permissions)
		if perm == "" {
			continue
		}
		match := NormalizeFilenamePermissionMatch(item.Match)
		if match != FilenamePermissionMatchExact && perm == "0000" {
			continue
		}
		text := NormalizeOptionalHexColor(item.Text)
		icon := NormalizeFilenameIcon(item.Icon)
		if text == "" && icon == "" {
			continue
		}
		key := match + ":" + perm
		if _, exists := byKey[key]; !exists {
			order = append(order, key)
		}
		byKey[key] = FilenamePermissionRule{
			Permissions: perm,
			Match:       match,
			Text:        text,
			Icon:        icon,
		}
	}
	if len(order) == 0 {
		return nil
	}
	out := make([]FilenamePermissionRule, 0, len(order))
	for _, key := range order {
		out = append(out, byKey[key])
	}
	return out
}

func NormalizeFilenameExtensionRules(raw []FilenameExtensionRule) []FilenameExtensionRule {
	if len(raw) == 0 {
		return nil
	}
	byExt := make(map[string]FilenameExtensionRule, len(raw))
	order := make([]string, 0, len(raw))
	for _, item := range raw {
		ext := NormalizeFilenameExtension(item.Extension)
		if ext == "" {
			continue
		}
		text := NormalizeOptionalHexColor(item.Text)
		icon := NormalizeFilenameIcon(item.Icon)
		if text == "" && icon == "" {
			continue
		}
		if _, exists := byExt[ext]; !exists {
			order = append(order, ext)
		}
		byExt[ext] = FilenameExtensionRule{
			Extension: ext,
			Text:      text,
			Icon:      icon,
		}
	}
	if len(order) == 0 {
		return nil
	}
	out := make([]FilenameExtensionRule, 0, len(order))
	for _, ext := range order {
		out = append(out, byExt[ext])
	}
	return out
}

func NormalizeFilenameSizeRules(raw []FilenameSizeRule) []FilenameSizeRule {
	if len(raw) == 0 {
		return nil
	}
	byKey := make(map[string]FilenameSizeRule, len(raw))
	order := make([]string, 0, len(raw))
	for _, item := range raw {
		size, ok := NormalizeFilenameSize(item.Size)
		if !ok {
			continue
		}
		match := NormalizeFilenameSizeMatch(item.Match)
		text := NormalizeOptionalHexColor(item.Text)
		icon := NormalizeFilenameIcon(item.Icon)
		if text == "" && icon == "" {
			continue
		}
		key := match + ":" + size
		if _, exists := byKey[key]; !exists {
			order = append(order, key)
		}
		byKey[key] = FilenameSizeRule{
			Size:  size,
			Match: match,
			Text:  text,
			Icon:  icon,
		}
	}
	if len(order) == 0 {
		return nil
	}
	out := make([]FilenameSizeRule, 0, len(order))
	for _, key := range order {
		out = append(out, byKey[key])
	}
	return out
}
