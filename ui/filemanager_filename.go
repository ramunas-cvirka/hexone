// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"hexone/filesys"
	"hexone/fm"
	"image/color"
	"sort"
	"sync"
	"time"

	"gioui.org/widget"
	mdicons "golang.org/x/exp/shiny/materialdesign/icons"
)

type filenameIconOption struct {
	key   string
	label string
}

var filenameIconOptions = []filenameIconOption{
	{key: "", label: "Default"},
	{key: fm.FilenameIconRecent, label: "Recent"},
	{key: fm.FilenameIconDocument, label: "Document"},
	{key: fm.FilenameIconCode, label: "Code"},
	{key: fm.FilenameIconImage, label: "Image"},
	{key: fm.FilenameIconVideo, label: "Video"},
	{key: fm.FilenameIconArchive, label: "Archive"},
	{key: fm.FilenameIconLocked, label: "Locked"},
}

type filePaneFilenameVisual struct {
	color    color.NRGBA
	hasColor bool
	iconKey  string
}

type filePaneFilenameAgeRule struct {
	maxAge time.Duration
	visual filePaneFilenameVisual
}

type filePaneFilenameTheme struct {
	defaultVisual   filePaneFilenameVisual
	ageRules        []filePaneFilenameAgeRule
	permissionRules map[string]filePaneFilenameVisual
}

var filenameRuleIcons struct {
	once     sync.Once
	file     *widget.Icon
	recent   *widget.Icon
	document *widget.Icon
	code     *widget.Icon
	image    *widget.Icon
	video    *widget.Icon
	archive  *widget.Icon
	locked   *widget.Icon
}

func mustFilenameIcon(ic *widget.Icon, err error) *widget.Icon {
	if err != nil {
		panic(err)
	}
	return ic
}

func loadFilenameRuleIcons() {
	filenameRuleIcons.file = mustFilenameIcon(widget.NewIcon(mdicons.EditorInsertDriveFile))
	filenameRuleIcons.recent = mustFilenameIcon(widget.NewIcon(mdicons.ActionSchedule))
	filenameRuleIcons.document = mustFilenameIcon(widget.NewIcon(mdicons.ActionDescription))
	filenameRuleIcons.code = mustFilenameIcon(widget.NewIcon(mdicons.EditorModeEdit))
	filenameRuleIcons.image = mustFilenameIcon(widget.NewIcon(mdicons.EditorInsertPhoto))
	filenameRuleIcons.video = mustFilenameIcon(widget.NewIcon(mdicons.AVMovie))
	filenameRuleIcons.archive = mustFilenameIcon(widget.NewIcon(mdicons.ContentArchive))
	filenameRuleIcons.locked = mustFilenameIcon(widget.NewIcon(mdicons.ActionLock))
}

func filenameIconLabel(key string) string {
	for _, opt := range filenameIconOptions {
		if opt.key == fm.NormalizeFilenameIcon(key) {
			return opt.label
		}
	}
	return "Default"
}

func filenamePreviewIcon(key string) *widget.Icon {
	if icon := filenameRuleIcon(key); icon != nil {
		return icon
	}
	filenameRuleIcons.once.Do(loadFilenameRuleIcons)
	return filenameRuleIcons.file
}

func filenameRuleIcon(key string) *widget.Icon {
	filenameRuleIcons.once.Do(loadFilenameRuleIcons)
	switch fm.NormalizeFilenameIcon(key) {
	case fm.FilenameIconRecent:
		return filenameRuleIcons.recent
	case fm.FilenameIconDocument:
		return filenameRuleIcons.document
	case fm.FilenameIconCode:
		return filenameRuleIcons.code
	case fm.FilenameIconImage:
		return filenameRuleIcons.image
	case fm.FilenameIconVideo:
		return filenameRuleIcons.video
	case fm.FilenameIconArchive:
		return filenameRuleIcons.archive
	case fm.FilenameIconLocked:
		return filenameRuleIcons.locked
	default:
		return nil
	}
}

func newFilePaneFilenameTheme(cfg *fm.Config) filePaneFilenameTheme {
	if cfg == nil {
		cfg = fm.DefaultConfig()
	}
	theme := filePaneFilenameTheme{
		defaultVisual: parseFilePaneFilenameVisual(cfg.Colors.Filenames.Text, cfg.Colors.Filenames.Icon),
	}
	if len(cfg.Colors.Filenames.AgeRules) > 0 {
		theme.ageRules = make([]filePaneFilenameAgeRule, 0, len(cfg.Colors.Filenames.AgeRules))
		for _, rule := range cfg.Colors.Filenames.AgeRules {
			maxAge, ok := fm.ParseFilenameAge(rule.MaxAge)
			if !ok || maxAge <= 0 {
				continue
			}
			visual := parseFilePaneFilenameVisual(rule.Text, rule.Icon)
			if !visual.hasColor && visual.iconKey == "" {
				continue
			}
			theme.ageRules = append(theme.ageRules, filePaneFilenameAgeRule{
				maxAge: maxAge,
				visual: visual,
			})
		}
		sort.SliceStable(theme.ageRules, func(i, j int) bool {
			return theme.ageRules[i].maxAge < theme.ageRules[j].maxAge
		})
	}
	if len(cfg.Colors.Filenames.PermissionRules) > 0 {
		theme.permissionRules = make(map[string]filePaneFilenameVisual, len(cfg.Colors.Filenames.PermissionRules))
		for _, rule := range cfg.Colors.Filenames.PermissionRules {
			perm := fm.NormalizeFilenamePermissions(rule.Permissions)
			if perm == "" {
				continue
			}
			visual := parseFilePaneFilenameVisual(rule.Text, rule.Icon)
			if !visual.hasColor && visual.iconKey == "" {
				continue
			}
			theme.permissionRules[perm] = visual
		}
	}
	return theme
}

func parseFilePaneFilenameVisual(textHex, iconKey string) filePaneFilenameVisual {
	visual := filePaneFilenameVisual{
		iconKey: fm.NormalizeFilenameIcon(iconKey),
	}
	if c, ok := fm.ParseHexColor(textHex); ok {
		visual.color = c
		visual.hasColor = true
	}
	return visual
}

func (v filePaneFilenameVisual) merge(next filePaneFilenameVisual) filePaneFilenameVisual {
	if next.hasColor {
		v.color = next.color
		v.hasColor = true
	}
	if next.iconKey != "" {
		v.iconKey = next.iconKey
	}
	return v
}

func (t filePaneFilenameTheme) visualForEntry(entry filesys.Entry, now time.Time) filePaneFilenameVisual {
	if entry.Kind != filesys.EntryFile {
		return filePaneFilenameVisual{}
	}
	visual := t.defaultVisual
	if !entry.ModTime.IsZero() && len(t.ageRules) > 0 {
		age := now.Sub(entry.ModTime)
		if age < 0 {
			age = 0
		}
		for _, rule := range t.ageRules {
			if age <= rule.maxAge {
				visual = visual.merge(rule.visual)
				break
			}
		}
	}
	if len(t.permissionRules) > 0 {
		if permVisual, ok := t.permissionRules[fm.NormalizeFilenamePermissions(entry.PermOctal)]; ok {
			visual = visual.merge(permVisual)
		}
	}
	return visual
}

func (m *filePaneModel) rebuildFilenameVisuals(now time.Time) {
	if m == nil {
		return
	}
	if len(m.entries) == 0 {
		m.filenameVisuals = nil
		return
	}
	if cap(m.filenameVisuals) < len(m.entries) {
		m.filenameVisuals = make([]filePaneFilenameVisual, len(m.entries))
	} else {
		m.filenameVisuals = m.filenameVisuals[:len(m.entries)]
	}
	for i, entry := range m.entries {
		m.filenameVisuals[i] = m.filenameTheme.visualForEntry(entry, now)
	}
}

func (m *filePaneModel) filenameVisual(row int) filePaneFilenameVisual {
	if m == nil || row < 0 || row >= len(m.filenameVisuals) {
		return filePaneFilenameVisual{}
	}
	return m.filenameVisuals[row]
}
