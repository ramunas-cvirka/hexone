// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package fm

import (
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
	FilenameIconArchive  = "archive"
	FilenameIconLocked   = "locked"
)

type FilenameColorsConfig struct {
	Text            string                   `yaml:"text,omitempty"`
	Icon            string                   `yaml:"icon,omitempty"`
	AgeRules        []FilenameAgeRule        `yaml:"age_rules,omitempty"`
	PermissionRules []FilenamePermissionRule `yaml:"permission_rules,omitempty"`
}

type FilenameAgeRule struct {
	MaxAge string `yaml:"max_age,omitempty"`
	Text   string `yaml:"text,omitempty"`
	Icon   string `yaml:"icon,omitempty"`
}

type FilenamePermissionRule struct {
	Permissions string `yaml:"permissions,omitempty"`
	Text        string `yaml:"text,omitempty"`
	Icon        string `yaml:"icon,omitempty"`
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
	case "archive", "zip", "compressed":
		return FilenameIconArchive
	case "locked", "lock", "private":
		return FilenameIconLocked
	default:
		return ""
	}
}

func ParseFilenameAge(raw string) (time.Duration, bool) {
	txt := strings.ToLower(strings.TrimSpace(raw))
	if txt == "" {
		return 0, false
	}
	if len(txt) < 2 {
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
	byPerm := make(map[string]FilenamePermissionRule, len(raw))
	order := make([]string, 0, len(raw))
	for _, item := range raw {
		perm := NormalizeFilenamePermissions(item.Permissions)
		if perm == "" {
			continue
		}
		text := NormalizeOptionalHexColor(item.Text)
		icon := NormalizeFilenameIcon(item.Icon)
		if text == "" && icon == "" {
			continue
		}
		if _, exists := byPerm[perm]; !exists {
			order = append(order, perm)
		}
		byPerm[perm] = FilenamePermissionRule{
			Permissions: perm,
			Text:        text,
			Icon:        icon,
		}
	}
	if len(order) == 0 {
		return nil
	}
	out := make([]FilenamePermissionRule, 0, len(order))
	for _, perm := range order {
		out = append(out, byPerm[perm])
	}
	return out
}
