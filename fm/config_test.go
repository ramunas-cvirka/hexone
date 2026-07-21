// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package fm

import (
	resources "hexone"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestNameCompactBackwardCompatibility(t *testing.T) {
	raw := `
name_compact:
  keep_start_chars: 7
  min_head: 9
  min_tail: 5
  approx_char_px: 11
  marker: __
columns:
  permission_format: symbolic
  perm_min_width_dp: 10
  size_min_width_dp: 11
  date_min_width_dp: 12
date_formats:
  - Jan 02
  - 01-02
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if cfg.NameCompact.KeepStartChars != 7 {
		t.Fatalf("KeepStartChars=%d, want 7", cfg.NameCompact.KeepStartChars)
	}
	if cfg.NameCompact.Marker != "__" {
		t.Fatalf("Marker=%q, want __", cfg.NameCompact.Marker)
	}

	if got, want := SizeMinWidthDp(cfg), defaultApproxCharPx*5+8+8; got != want {
		t.Fatalf("SizeMinWidthDp=%d, want %d", got, want)
	}
	if got, want := PermMinWidthDp(cfg), columnWidthDp(defaultOctalWidthChars, false); got != want {
		t.Fatalf("PermMinWidthDp=%d, want %d", got, want)
	}
	if got, want := DateMinWidthDp(cfg), defaultApproxCharPx*5+16+8; got != want {
		t.Fatalf("DateMinWidthDp=%d, want %d", got, want)
	}
}

func TestMarshalConfigOmitsInternalFields(t *testing.T) {
	cfg := DefaultConfig()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	out := string(data)

	for _, disallowed := range []string{
		"approx_char_px:",
		"min_head:",
		"min_tail:",
		"perm_min_width_dp:",
		"size_min_width_dp:",
		"date_min_width_dp:",
		"name_width_dp:",
		"name_min_width_dp:",
		"perm_width_dp:",
		"full_pad_dp:",
		"size_width_dp:",
		"date_width_dp:",
		"brief_width_dp:",
		"brief_gap_dp:",
	} {
		if strings.Contains(out, disallowed) {
			t.Fatalf("serialized config still contains %q:\n%s", disallowed, out)
		}
	}
	if !strings.Contains(out, "keep_start_chars:") {
		t.Fatalf("serialized config missing keep_start_chars:\n%s", out)
	}
	if !strings.Contains(out, "full_chars:") || !strings.Contains(out, "brief_chars:") {
		t.Fatalf("serialized config missing character-based column widths:\n%s", out)
	}
	if strings.Contains(out, "name_chars:") {
		t.Fatalf("serialized config should use full_chars instead of name_chars:\n%s", out)
	}
	if strings.Contains(out, "key_bindings:") {
		t.Fatalf("serialized config should not contain legacy key_bindings block:\n%s", out)
	}
	if !strings.Contains(out, "interface:\n") || !strings.Contains(out, "font_size_sp: 14") {
		t.Fatalf("serialized config missing interface font settings:\n%s", out)
	}
}

func TestMouseWheelSelectionMovementDefaultsOffAndRoundTrips(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.General.WheelMovesSelection {
		t.Fatal("mouse wheel should scroll the pane without moving the active item by default")
	}

	cfg.General.WheelMovesSelection = true
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if !strings.Contains(string(data), "wheel_moves_selection: true") {
		t.Fatalf("serialized config missing wheel behavior:\n%s", data)
	}

	loaded := DefaultConfig()
	if err := yaml.Unmarshal(data, loaded); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if !loaded.General.WheelMovesSelection {
		t.Fatal("mouse wheel selection behavior did not survive config round trip")
	}
}

func TestDeleteSafetyOptionsDefaultOffAndRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.General.UseTrash || cfg.General.DeleteWithoutConfirm {
		t.Fatalf("delete options should default off: useTrash=%v withoutConfirmation=%v", cfg.General.UseTrash, cfg.General.DeleteWithoutConfirm)
	}

	cfg.General.UseTrash = true
	cfg.General.DeleteWithoutConfirm = true
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "use_trash: true") || !strings.Contains(out, "delete_without_confirmation: true") {
		t.Fatalf("serialized config missing delete options:\n%s", out)
	}

	loaded := DefaultConfig()
	if err := yaml.Unmarshal(data, loaded); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if !loaded.General.UseTrash || !loaded.General.DeleteWithoutConfirm {
		t.Fatalf("delete options did not round trip: %#v", loaded.General)
	}
}

func TestNormalizeTerminalHeightRowsAllowsTallDynamicLayouts(t *testing.T) {
	if got, want := NormalizeTerminalHeightRows(180), 180; got != want {
		t.Fatalf("NormalizeTerminalHeightRows(180)=%d want %d", got, want)
	}
	if got, want := NormalizeTerminalHeightRows(10_000), maxTerminalHeightRows; got != want {
		t.Fatalf("NormalizeTerminalHeightRows safety clamp=%d want %d", got, want)
	}
}

func TestDateWidthFitsPreferredFormat(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DateFormats = []string{"2006-01-02 15:04:05", "01-02"}
	got := DateWidthDp(cfg)
	want := columnWidthDp(float32(len(cfg.DateFormats[0]))+0.5, false)
	if got != want {
		t.Fatalf("DateWidthDp=%d want %d for preferred format", got, want)
	}
}

func TestMetadataWidthsFitPreferredTextWithoutExcessSlack(t *testing.T) {
	cfg := DefaultConfig()
	if got, want := PermWidthDp(cfg), columnWidthDp(defaultPermWidthChars, false); got != want {
		t.Fatalf("symbolic permission width=%d want %d", got, want)
	}
	cfg.Columns.PermissionFormat = "octal"
	if got, want := PermWidthDp(cfg), columnWidthDp(defaultOctalWidthChars, false); got != want {
		t.Fatalf("octal permission width=%d want %d", got, want)
	}
	if got, want := SizeWidthDp(cfg), columnWidthDp(defaultSizeWidthChars, false); got != want {
		t.Fatalf("size width=%d want %d", got, want)
	}
}

func TestInterfaceFontDefaultsIndependentlyFromPaneFont(t *testing.T) {
	raw := `
general:
  typeface: Iosevka Nerd Font Mono
  font_size_sp: 22
`
	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if got, want := cfg.General.FontSizeSp, float32(22); got != want {
		t.Fatalf("pane font size=%v want %v", got, want)
	}
	if got, want := cfg.Interface.FontSizeSp, float32(14); got != want {
		t.Fatalf("interface font size=%v want independent default %v", got, want)
	}
	if got, want := cfg.Interface.Typeface, resources.BundledFontFamilyFiraCodeNerdFontMono; got != want {
		t.Fatalf("interface typeface=%q want %q", got, want)
	}
}

func TestDefaultConfigUsesShippedVisualStyle(t *testing.T) {
	cfg := DefaultConfig()

	if got, want := cfg.General.Typeface, resources.BundledFontFamilyFiraCodeNerdFontMono; got != want {
		t.Fatalf("pane typeface=%q want %q", got, want)
	}
	if got, want := cfg.General.FontSizeSp, float32(15); got != want {
		t.Fatalf("pane font size=%v want %v", got, want)
	}
	if got, want := cfg.Interface.FontSizeSp, float32(14); got != want {
		t.Fatalf("interface font size=%v want %v", got, want)
	}
	if got, want := cfg.Tabs.Typeface, resources.BundledFontFamilyIosevkaNerdFontMono; got != want {
		t.Fatalf("tabs typeface=%q want %q", got, want)
	}
	if got, want := cfg.Tabs.FontSizeSp, float32(12); got != want {
		t.Fatalf("tabs font size=%v want %v", got, want)
	}
	if got, want := cfg.Terminal.FontSizeSp, float32(14); got != want {
		t.Fatalf("terminal font size=%v want %v", got, want)
	}
	if got, want := cfg.Viewer.FontSizeSp, float32(14); got != want {
		t.Fatalf("viewer font size=%v want %v", got, want)
	}

	colors := map[string]struct {
		got  string
		want string
	}{
		"pane background":       {cfg.Colors.FilePaneBackground, "#202020"},
		"pane text":             {cfg.Colors.FilePaneText, "#BABABA"},
		"hover":                 {cfg.Colors.Hover, "#2A2A2A"},
		"selection":             {cfg.Colors.Selection, "#3A3A3A"},
		"selected files":        {cfg.Colors.SelectedFiles, "#002CF0"},
		"selected files text":   {cfg.Colors.SelectedFilesText, "#FBC4DF"},
		"focused selected":      {cfg.Colors.FocusedSelected, "#0000F0"},
		"focused selected text": {cfg.Colors.FocusedSelectedText, "#F66EB2"},
		"viewer background":     {cfg.Viewer.Background, "#202020"},
		"viewer text":           {cfg.Viewer.Text, "#D2D2D2"},
		"viewer selection":      {cfg.Viewer.Selection, "#3C3C50"},
	}
	for name, test := range colors {
		if test.got != test.want {
			t.Errorf("%s=%q want %q", name, test.got, test.want)
		}
	}

	if got, want := cfg.Colors.Filenames.AgeRules, []FilenameAgeRule{{MaxAge: "1d", Text: "#FFFFFF"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("filename age rules=%#v want %#v", got, want)
	}
}

func TestGeneralPaneWeightDefaultsAndNormalization(t *testing.T) {
	cfg := DefaultConfig()
	if got, want := cfg.General.FileWeight, FontWeightRegular; got != want {
		t.Fatalf("default file weight=%q want %q", got, want)
	}
	if got, want := cfg.General.DirWeight, FontWeightBold; got != want {
		t.Fatalf("default dir weight=%q want %q", got, want)
	}
	if got, want := cfg.General.PermissionsWeight, FontWeightRegular; got != want {
		t.Fatalf("default permissions weight=%q want %q", got, want)
	}

	raw := `
general:
  file_weight: light
  dir_weight: normal
  permissions_weight: invalid
  size_weight: bold
  date_weight: regular
`
	cfg = DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if got, want := cfg.General.FileWeight, FontWeightRegular; got != want {
		t.Fatalf("file weight=%q want normalized %q", got, want)
	}
	if got, want := cfg.General.DirWeight, FontWeightRegular; got != want {
		t.Fatalf("dir weight=%q want %q", got, want)
	}
	if got, want := cfg.General.PermissionsWeight, FontWeightRegular; got != want {
		t.Fatalf("permissions weight=%q want fallback %q", got, want)
	}
	if got, want := cfg.General.SizeWeight, FontWeightBold; got != want {
		t.Fatalf("size weight=%q want %q", got, want)
	}
	if got, want := cfg.General.DateWeight, FontWeightRegular; got != want {
		t.Fatalf("date weight=%q want %q", got, want)
	}
}

func TestTerminalAcceleratedKeysDefaultsOnAndCanBeDisabled(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Terminal.AcceleratedKeys {
		t.Fatal("default terminal accelerated keys should be enabled")
	}
	if cfg.Terminal.Maximized {
		t.Fatal("default terminal should not be maximized")
	}

	raw := `
general:
  font_size_sp: 16
`
	cfg = DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config without terminal settings: %v", err)
	}
	cfg.normalize()
	if !cfg.Terminal.AcceleratedKeys {
		t.Fatal("decoded config should keep terminal accelerated keys enabled when omitted")
	}

	raw = `
terminal:
  accelerated_keys: false
`
	cfg = DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()
	if cfg.Terminal.AcceleratedKeys {
		t.Fatal("terminal accelerated keys should preserve explicit false")
	}

	raw = `
terminal:
  maximized: true
`
	cfg = DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal maximized terminal config: %v", err)
	}
	cfg.normalize()
	if !cfg.Terminal.Maximized {
		t.Fatal("terminal maximized state should preserve explicit true")
	}
}

func TestConfigNormalizesTerminalHeightAndSortPerDir(t *testing.T) {
	raw := `
sort:
  default_key: name
  descending: false
  per_dir:
    /tmp/by-date: date:desc
    /tmp/default: n+
    /tmp/bad: sideways
terminal:
  height_rows: 2
  typeface: Imaginary Sans
  font_size_sp: 1
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if got, want := cfg.Terminal.HeightRows, 4; got != want {
		t.Fatalf("Terminal.HeightRows=%d want %d", got, want)
	}
	if got, want := cfg.Terminal.Typeface, resources.BundledFontFamilyFiraCodeNerdFontMono; got != want {
		t.Fatalf("Terminal.Typeface=%q want %q", got, want)
	}
	if got, want := cfg.Terminal.FontSizeSp, float32(13); got != want {
		t.Fatalf("Terminal.FontSizeSp=%v want %v", got, want)
	}
	if got, want := cfg.Sort.PerDir[filepath.Clean("/tmp/by-date")], "d-"; got != want {
		t.Fatalf("sort per dir code=%q want %q", got, want)
	}
	if _, ok := cfg.Sort.PerDir[filepath.Clean("/tmp/default")]; ok {
		t.Fatalf("default sort override should be omitted: %#v", cfg.Sort.PerDir)
	}
	if _, ok := cfg.Sort.PerDir[filepath.Clean("/tmp/bad")]; ok {
		t.Fatalf("invalid sort override should be omitted: %#v", cfg.Sort.PerDir)
	}
}

func TestConfigNormalizesTabs(t *testing.T) {
	raw := `
tabs:
  width_mode: fixed
  min_width_dp: 12
  fixed_width_dp: 999
  max_width_dp: 90
  typeface: Iosevka Nerd Font Mono
  font_size_sp: 11
  color: aa3366
  alt_color: nope
  active_color: '#112233'
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if got, want := cfg.Tabs.WidthMode, "fixed"; got != want {
		t.Fatalf("Tabs.WidthMode=%q want %q", got, want)
	}
	if got, want := cfg.Tabs.MinWidthDp, 44; got != want {
		t.Fatalf("Tabs.MinWidthDp=%d want %d", got, want)
	}
	if got, want := cfg.Tabs.MaxWidthDp, 90; got != want {
		t.Fatalf("Tabs.MaxWidthDp=%d want %d", got, want)
	}
	if got, want := cfg.Tabs.FixedWidthDp, 90; got != want {
		t.Fatalf("Tabs.FixedWidthDp=%d want %d", got, want)
	}
	if got, want := cfg.Tabs.Typeface, resources.BundledFontFamilyIosevkaNerdFontMono; got != want {
		t.Fatalf("Tabs.Typeface=%q want %q", got, want)
	}
	if got, want := cfg.Tabs.FontSizeSp, float32(11); got != want {
		t.Fatalf("Tabs.FontSizeSp=%v want %v", got, want)
	}
	if got, want := cfg.Tabs.Color, "#AA3366"; got != want {
		t.Fatalf("Tabs.Color=%q want %q", got, want)
	}
	if cfg.Tabs.AltColor != "" {
		t.Fatalf("Tabs.AltColor=%q want empty invalid color", cfg.Tabs.AltColor)
	}
	if got, want := cfg.Tabs.ActiveColor, "#112233"; got != want {
		t.Fatalf("Tabs.ActiveColor=%q want %q", got, want)
	}
}

func TestDefaultConfigSerializesTabs(t *testing.T) {
	out := string(mustMarshalConfig(t, DefaultConfig()))

	for _, want := range []string{
		"tabs:",
		"width_mode: variable",
		"max_width_dp:",
		"typeface: Iosevka Nerd Font Mono",
		"font_size_sp: 12",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("serialized config missing %q:\n%s", want, out)
		}
	}
}

func TestConfigDropsStaleFieldsOnSave(t *testing.T) {
	raw := `
tabs:
  alternating_colors: true
viewer:
  mode: file
  associated_extensions:
    - txt
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	out := string(mustMarshalConfig(t, cfg))
	for _, stale := range []string{"alternating_colors:", "associated_extensions:", "mode: file"} {
		if strings.Contains(out, stale) {
			t.Fatalf("serialized config retained stale field %q:\n%s", stale, out)
		}
	}
}

func TestTabsFontDefaultsToPaneFontForExistingConfigs(t *testing.T) {
	raw := `
general:
  typeface: Iosevka Nerd Font Mono
  font_size_sp: 16.8
tabs:
  width_mode: variable
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if got, want := cfg.Tabs.Typeface, resources.BundledFontFamilyIosevkaNerdFontMono; got != want {
		t.Fatalf("Tabs.Typeface=%q want inherited %q", got, want)
	}
	if got, want := cfg.Tabs.FontSizeSp, float32(12); got != want {
		t.Fatalf("Tabs.FontSizeSp=%v want inherited scaled size %v", got, want)
	}
}

func TestNormalizeViewerShellSupportsPowerShellCmdAndWSL(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{raw: "", want: "auto"},
		{raw: " AUTO ", want: "auto"},
		{raw: "bash", want: "sh"},
		{raw: "pwsh.exe", want: "pwsh"},
		{raw: "PowerShell.EXE", want: "powershell"},
		{raw: "cmd.exe", want: "cmd"},
		{raw: "wsl.exe", want: "wsl"},
		{raw: "wsl:Ubuntu-24.04", want: "wsl:Ubuntu-24.04"},
		{raw: "unknown", want: "auto"},
	}
	for _, tc := range cases {
		if got := NormalizeViewerShell(tc.raw); got != tc.want {
			t.Fatalf("NormalizeViewerShell(%q)=%q want %q", tc.raw, got, tc.want)
		}
	}
}

func TestLegacyColumnWidthsMigrateToChars(t *testing.T) {
	raw := `
columns:
  name_width_dp: 180
  name_min_width_dp: 52
  perm_width_dp: 92
  full_pad_dp: 4
  size_width_dp: 92
  date_width_dp: 128
  brief_width_dp: 180
  brief_gap_dp: 4
  show_permissions: false
  permission_format: octal
  full_drop_priority:
    - size
    - date
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if got, want := cfg.Columns.NameChars, float32(20); got != want {
		t.Fatalf("Columns.NameChars=%v, want %v", got, want)
	}
	if got, want := cfg.Columns.BriefChars, float32(20); got != want {
		t.Fatalf("Columns.BriefChars=%v, want %v", got, want)
	}
	if cfg.Columns.ShowPermissions {
		t.Fatal("Columns.ShowPermissions should preserve explicit false")
	}
	if cfg.Columns.PermissionFormat != "octal" {
		t.Fatalf("Columns.PermissionFormat=%q, want octal", cfg.Columns.PermissionFormat)
	}

	out := string(mustMarshalConfig(t, cfg))
	if strings.Contains(out, "name_width_dp:") || strings.Contains(out, "brief_width_dp:") {
		t.Fatalf("serialized config should not contain legacy dp widths:\n%s", out)
	}
	if !strings.Contains(out, "full_chars: 20") || !strings.Contains(out, "brief_chars: 20") {
		t.Fatalf("serialized config missing migrated character widths:\n%s", out)
	}
}

func TestLegacyNameCharsMigratesToFullChars(t *testing.T) {
	raw := `
columns:
  name_chars: 24
  brief_chars: 16
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if got, want := cfg.Columns.NameChars, float32(24); got != want {
		t.Fatalf("Columns.NameChars=%v, want %v", got, want)
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "full_chars: 24") {
		t.Fatalf("serialized config missing full_chars:\n%s", out)
	}
	if strings.Contains(out, "name_chars:") {
		t.Fatalf("serialized config should not contain legacy name_chars:\n%s", out)
	}
}

func TestLegacyFontBlockMigratesToGeneral(t *testing.T) {
	raw := `
font:
  typeface: Hack Nerd Font Mono
  size_sp: 16
general:
  dim_inactive_panes: true
viewer:
  shell: auto
  command: cat {path}
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if cfg.General.Typeface != resources.BundledFontFamilyHackNerdFontMono {
		t.Fatalf("General.Typeface=%q, want %s", cfg.General.Typeface, resources.BundledFontFamilyHackNerdFontMono)
	}
	if cfg.General.FontSizeSp != 16 {
		t.Fatalf("General.FontSizeSp=%v, want 16", cfg.General.FontSizeSp)
	}
	if cfg.Terminal.Typeface != resources.BundledFontFamilyHackNerdFontMono {
		t.Fatalf("Terminal.Typeface=%q, want %s", cfg.Terminal.Typeface, resources.BundledFontFamilyHackNerdFontMono)
	}
	if got, want := cfg.Terminal.FontSizeSp, cfg.General.FontSizeSp*(13.0/14.0); got != want {
		t.Fatalf("Terminal.FontSizeSp=%v, want %v", got, want)
	}
	if !cfg.General.DimInactivePanes {
		t.Fatal("General.DimInactivePanes should preserve general block values")
	}

	out := string(mustMarshalConfig(t, cfg))
	if strings.Contains(out, "\nfont:\n") {
		t.Fatalf("serialized config should not contain legacy font block:\n%s", out)
	}
	if !strings.Contains(out, "general:") || !strings.Contains(out, "typeface: Hack Nerd Font Mono") || !strings.Contains(out, "font_size_sp: 16") {
		t.Fatalf("serialized config missing migrated general font settings:\n%s", out)
	}
}

func TestNormalizeViewerAssociations(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.Associations = []ViewerAssociation{
		{Extension: " PDF ", AppPath: ` "C:\Apps\SumatraPDF\SumatraPDF.exe" `},
		{Extension: ".png", AppPath: `C:\Apps\ImageView\viewer.exe`},
		{Extension: "*.tar.gz", AppPath: `C:\Apps\Archive\archive.exe`},
		{Extension: ".PNG", AppPath: `C:\Apps\ImageView\new-viewer.exe`},
		{Extension: "", AppPath: `C:\bad.exe`},
		{Extension: ".", AppPath: `C:\bad.exe`},
		{Extension: `dir\bad`, AppPath: `C:\bad.exe`},
		{Extension: ".txt", AppPath: ""},
	}

	cfg.normalize()

	if len(cfg.Viewer.Associations) != 0 {
		t.Fatalf("viewer.associations should be cleared after normalize, got %#v", cfg.Viewer.Associations)
	}

	got := make([]string, 0, len(FlattenAssociationPrograms(cfg.Associations)))
	for _, assoc := range FlattenAssociationPrograms(cfg.Associations) {
		got = append(got, assoc.Extension+"="+assoc.AppPath)
	}
	want := ".pdf=C:\\Apps\\SumatraPDF\\SumatraPDF.exe,.png=C:\\Apps\\ImageView\\new-viewer.exe,.tar.gz=C:\\Apps\\Archive\\archive.exe"
	if strings.Join(got, ",") != want {
		t.Fatalf("Associations=%q, want %q", strings.Join(got, ","), want)
	}

	out := string(mustMarshalConfig(t, cfg))
	if strings.Contains(out, "viewer:\n    associations:") {
		t.Fatalf("serialized config should not keep viewer.associations:\n%s", out)
	}
	if !strings.Contains(out, "associations:") {
		t.Fatalf("serialized config missing top-level associations:\n%s", out)
	}
}

func TestNormalizeTopLevelAssociationPrograms(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Associations = []AssociationProgram{
		{AppPath: ` "C:\Apps\Player\player.exe" `, Extensions: []string{"MP3", ".mkv"}},
		{AppPath: `C:\Apps\Player\player.exe`, Extensions: []string{"*.mp4", ".mp3"}},
		{AppPath: `C:\Apps\Viewer\view.exe`, Extensions: []string{".pdf"}},
	}

	cfg.normalize()

	got := make([]string, 0, len(FlattenAssociationPrograms(cfg.Associations)))
	for _, assoc := range FlattenAssociationPrograms(cfg.Associations) {
		got = append(got, assoc.Extension+"="+assoc.AppPath)
	}
	want := ".mkv=C:\\Apps\\Player\\player.exe,.mp3=C:\\Apps\\Player\\player.exe,.mp4=C:\\Apps\\Player\\player.exe,.pdf=C:\\Apps\\Viewer\\view.exe"
	if strings.Join(got, ",") != want {
		t.Fatalf("FlattenAssociationPrograms=%q, want %q", strings.Join(got, ","), want)
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "extensions: mkv, mp3, mp4") {
		t.Fatalf("serialized grouped associations should use compact csv extensions:\n%s", out)
	}
	if strings.Contains(out, "- .mkv") || strings.Contains(out, "extensions:\n") {
		t.Fatalf("serialized grouped associations should not use a multiline extension list:\n%s", out)
	}
}

func TestLoadTopLevelAssociationProgramsFromCompactAndLegacyForms(t *testing.T) {
	raw := `
associations:
  - app_path: C:\Apps\Player\player.exe
    extensions: mkv, mp3, mp4
  - app_path: C:\Apps\Viewer\view.exe
    extensions:
      - .pdf
      - txt
viewer:
  shell: auto
  command: cat {path}
`
	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	got := make([]string, 0, len(FlattenAssociationPrograms(cfg.Associations)))
	for _, assoc := range FlattenAssociationPrograms(cfg.Associations) {
		got = append(got, assoc.Extension+"="+assoc.AppPath)
	}
	want := ".mkv=C:\\Apps\\Player\\player.exe,.mp3=C:\\Apps\\Player\\player.exe,.mp4=C:\\Apps\\Player\\player.exe,.pdf=C:\\Apps\\Viewer\\view.exe,.txt=C:\\Apps\\Viewer\\view.exe"
	if strings.Join(got, ",") != want {
		t.Fatalf("loaded grouped associations=%q, want %q", strings.Join(got, ","), want)
	}
}

func TestNormalizeViewerCommandRules(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.CommandRules = []ViewerCommandRule{
		{Pattern: ` (?i)\.log$ `, Command: ` tail -f {path} `},
		{Pattern: `[`, Command: `broken`},
		{Pattern: `^access\.log$`, Command: `grep ERROR {path}`},
		{Pattern: `(?i)\.log$`, Command: `tail -n 200 {path}`},
		{Pattern: `^debug\.log$`, Command: ``},
	}

	cfg.normalize()

	got := make([]string, 0, len(cfg.Viewer.CommandRules))
	for _, rule := range cfg.Viewer.CommandRules {
		got = append(got, rule.Pattern+"="+rule.Command)
	}
	want := `^access\.log$=grep ERROR {path},(?i)\.log$=tail -n 200 {path}`
	if strings.Join(got, ",") != want {
		t.Fatalf("Viewer.CommandRules=%q, want %q", strings.Join(got, ","), want)
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "command_rules:") {
		t.Fatalf("serialized config missing viewer command_rules:\n%s", out)
	}
	if strings.Contains(out, "pattern: [") {
		t.Fatalf("serialized config should drop invalid command rule patterns:\n%s", out)
	}
}

func TestNormalizeCustomCommandsKeepsTenFixedSlots(t *testing.T) {
	cfg := DefaultConfig()
	for i := 0; i < 12; i++ {
		cfg.CustomCommands = append(cfg.CustomCommands, CustomCommand{
			Name:    strings.TrimSpace(" cmd "),
			Command: strings.TrimSpace("echo old"),
		})
		cfg.CustomCommands = append(cfg.CustomCommands, CustomCommand{
			Name:    "cmd" + string(rune('a'+i)),
			Command: " echo " + string(rune('a'+i)) + " ",
		})
	}
	cfg.CustomCommands = append(cfg.CustomCommands,
		CustomCommand{Name: "", Command: "  python - <<'PY'\nprint('hi')\nPY  "},
		CustomCommand{Name: "empty", Command: "   "},
	)

	cfg.normalize()

	if len(cfg.CustomCommands) != 10 {
		t.Fatalf("custom command count=%d want 10", len(cfg.CustomCommands))
	}
	for _, cmd := range cfg.CustomCommands {
		if strings.TrimSpace(cmd.Name) == "" || strings.TrimSpace(cmd.Command) == "" {
			t.Fatalf("custom command should be normalized, got %#v", cmd)
		}
		if cmd.Slot < 1 || cmd.Slot > 10 {
			t.Fatalf("custom command slot=%d want 1..10", cmd.Slot)
		}
	}
	for i, cmd := range cfg.CustomCommands {
		if cmd.Slot != i+1 {
			t.Fatalf("custom command slot at index %d=%d want %d", i, cmd.Slot, i+1)
		}
	}
}

func TestNormalizeCustomCommandDerivesNameFromFirstCommandLine(t *testing.T) {
	cmd, ok := NormalizeCustomCommand(CustomCommand{
		Command: "\n\n  echo hello | head -n 1\n",
	})
	if !ok {
		t.Fatal("expected command to normalize")
	}
	if cmd.Name != "echo hello | head -n 1" {
		t.Fatalf("derived name=%q", cmd.Name)
	}
}

func TestNormalizeViewerRemoteSearchCommandDefaultsAndAllowsOff(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Viewer.RemoteSearchMode != ViewerRemoteSearchModeRemote {
		t.Fatalf("default remote search mode=%q want %q", cfg.Viewer.RemoteSearchMode, ViewerRemoteSearchModeRemote)
	}
	if cfg.Viewer.RemoteSearchCommand != DefaultViewerRemoteSearchCommand {
		t.Fatalf("default remote search command=%q want %q", cfg.Viewer.RemoteSearchCommand, DefaultViewerRemoteSearchCommand)
	}

	cfg.Viewer.RemoteSearchCommand = ""
	cfg.normalize()
	if cfg.Viewer.RemoteSearchCommand != DefaultViewerRemoteSearchCommand {
		t.Fatalf("normalized remote search command=%q want default", cfg.Viewer.RemoteSearchCommand)
	}

	cfg.Viewer.RemoteSearchCommand = "off"
	cfg.normalize()
	if cfg.Viewer.RemoteSearchCommand != "off" {
		t.Fatalf("remote search disable value=%q want %q", cfg.Viewer.RemoteSearchCommand, "off")
	}
	if got := EffectiveViewerRemoteSearchCommand(cfg.Viewer.RemoteSearchCommand); got != "" {
		t.Fatalf("effective remote search command=%q want empty", got)
	}
}

func TestNormalizeViewerRemoteSearchModeDefaultsAndAcceptsAliases(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.RemoteSearchMode = ""
	cfg.normalize()
	if cfg.Viewer.RemoteSearchMode != ViewerRemoteSearchModeRemote {
		t.Fatalf("normalized remote search mode=%q want %q", cfg.Viewer.RemoteSearchMode, ViewerRemoteSearchModeRemote)
	}

	cfg.Viewer.RemoteSearchMode = "utility"
	cfg.normalize()
	if cfg.Viewer.RemoteSearchMode != ViewerRemoteSearchModeRemote {
		t.Fatalf("utility remote search mode=%q want %q", cfg.Viewer.RemoteSearchMode, ViewerRemoteSearchModeRemote)
	}

	cfg.Viewer.RemoteSearchMode = "internal"
	cfg.normalize()
	if cfg.Viewer.RemoteSearchMode != ViewerRemoteSearchModeLocal {
		t.Fatalf("internal remote search mode=%q want %q", cfg.Viewer.RemoteSearchMode, ViewerRemoteSearchModeLocal)
	}
}

func TestNormalizeViewerRemoteSearchCommandMigratesLegacyDefault(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.RemoteSearchCommand = legacyViewerRemoteSearchCommand
	cfg.normalize()
	if cfg.Viewer.RemoteSearchCommand != DefaultViewerRemoteSearchCommand {
		t.Fatalf("legacy remote search command=%q want %q", cfg.Viewer.RemoteSearchCommand, DefaultViewerRemoteSearchCommand)
	}
}

func TestMatchViewerCommandRulesUsesLastMatch(t *testing.T) {
	rules := []ViewerCommandRule{
		{Pattern: `\.log$`, Command: `tail -f {path}`},
		{Pattern: `^error\.log$`, Command: `grep ERROR {path}`},
	}

	got, ok := MatchViewerCommandRules(rules, "error.log")
	if !ok {
		t.Fatal("MatchViewerCommandRules should match error.log")
	}
	if got != `grep ERROR {path}` {
		t.Fatalf("MatchViewerCommandRules=%q, want %q", got, `grep ERROR {path}`)
	}

	if got, ok := MatchViewerCommandRules(rules, "notes.txt"); ok || got != "" {
		t.Fatalf("MatchViewerCommandRules(notes.txt) = (%q, %v), want no match", got, ok)
	}
}

func TestNormalizeViewerFileEncodingAcceptsUTF16Variants(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.FileEncoding = "utf16-be"

	cfg.normalize()

	if cfg.Viewer.FileEncoding != ViewerFileEncodingUTF16BE {
		t.Fatalf("Viewer.FileEncoding=%q, want %q", cfg.Viewer.FileEncoding, ViewerFileEncodingUTF16BE)
	}
}

func TestNormalizeViewerFileEncodingAcceptsCP437(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.FileEncoding = "ibm437"

	cfg.normalize()

	if cfg.Viewer.FileEncoding != ViewerFileEncodingCP437 {
		t.Fatalf("Viewer.FileEncoding=%q, want %q", cfg.Viewer.FileEncoding, ViewerFileEncodingCP437)
	}
}

func TestDefaultConfigUsesAutoViewerFileEncoding(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Viewer.FileEncoding != ViewerFileEncodingAuto {
		t.Fatalf("Viewer.FileEncoding=%q, want %q", cfg.Viewer.FileEncoding, ViewerFileEncodingAuto)
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "file_encoding: auto") {
		t.Fatalf("serialized config missing viewer file_encoding:\n%s", out)
	}
}

func TestDefaultConfigHidesFunctionBarWhenViewerOpen(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Viewer.HideFunctionBarWhenOpen {
		t.Fatal("viewer hide_function_bar_when_open should default to true")
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "hide_function_bar_when_open: true") {
		t.Fatalf("serialized config missing viewer hide_function_bar_when_open:\n%s", out)
	}
}

func TestDefaultConfigEnablesViewerSmoothScrolling(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Viewer.SmoothScrolling {
		t.Fatal("viewer smooth_scrolling should default to true")
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "smooth_scrolling: true") {
		t.Fatalf("serialized config missing viewer smooth_scrolling:\n%s", out)
	}
}

func TestDefaultConfigShowsViewerLineNumbers(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Viewer.ShowLineNumbers {
		t.Fatal("viewer show_line_numbers should default to true")
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "show_line_numbers: true") {
		t.Fatalf("serialized config missing viewer show_line_numbers:\n%s", out)
	}
}

func TestLoadConfigDefaultsViewerLineNumbersWhenFieldMissing(t *testing.T) {
	raw := `
viewer:
  shell: auto
  command: cat {path}
`
	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if !cfg.Viewer.ShowLineNumbers {
		t.Fatal("viewer show_line_numbers should stay enabled when yaml omits the field")
	}
}

func TestLoadConfigDefaultsViewerSmoothScrollingWhenFieldMissing(t *testing.T) {
	raw := `
viewer:
  shell: auto
  command: cat {path}
`
	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if !cfg.Viewer.SmoothScrolling {
		t.Fatal("viewer smooth_scrolling should stay enabled when yaml omits the field")
	}
}

func TestDefaultConfigDimsInactivePanes(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.General.DimInactivePanes {
		t.Fatal("general dim_inactive_panes should default to true")
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "dim_inactive_panes: true") {
		t.Fatalf("serialized config missing general dim_inactive_panes:\n%s", out)
	}
}

func TestDefaultConfigOpensFavoritesInNewTab(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.General.OpenFavoritesInNewTab {
		t.Fatal("general open_favorites_in_new_tab should default to true")
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "open_favorites_in_new_tab: true") {
		t.Fatalf("serialized config missing general open_favorites_in_new_tab:\n%s", out)
	}
}

func TestDefaultConfigCompletionSoundBackground(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.General.CompletionSound != CompletionSoundBackground {
		t.Fatalf("general completion_sound=%q want %q", cfg.General.CompletionSound, CompletionSoundBackground)
	}

	out := string(mustMarshalConfig(t, cfg))
	if !strings.Contains(out, "completion_sound: background") {
		t.Fatalf("serialized config missing general completion_sound:\n%s", out)
	}
}

func TestNormalizeCompletionSound(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "", want: CompletionSoundBackground},
		{raw: "never", want: CompletionSoundNever},
		{raw: "off", want: CompletionSoundNever},
		{raw: "always", want: CompletionSoundAlways},
		{raw: "on", want: CompletionSoundAlways},
		{raw: "background only", want: CompletionSoundBackground},
		{raw: "app_not_focused", want: CompletionSoundBackground},
		{raw: "surprise", want: CompletionSoundBackground},
	}
	for _, tt := range tests {
		if got := NormalizeCompletionSound(tt.raw); got != tt.want {
			t.Fatalf("NormalizeCompletionSound(%q)=%q want %q", tt.raw, got, tt.want)
		}
	}
}

func TestLoadConfigDefaultsFavoritesNewTabWhenFieldMissing(t *testing.T) {
	raw := `
general:
  dim_inactive_panes: true
`
	cfg := DefaultConfig()
	cfg.General.OpenFavoritesInNewTab = false
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if !cfg.General.OpenFavoritesInNewTab {
		t.Fatal("general open_favorites_in_new_tab should stay enabled when yaml omits the field")
	}
}

func TestLoadConfigAllowsFavoritesNewTabOptOut(t *testing.T) {
	raw := `
general:
  open_favorites_in_new_tab: false
`
	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	if cfg.General.OpenFavoritesInNewTab {
		t.Fatal("general open_favorites_in_new_tab=false should be preserved")
	}
}

func TestLoadConfigEnsuringFileCreatesDefaultWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")

	cfg, err := LoadConfigEnsuringFile(path)
	if err != nil {
		t.Fatalf("LoadConfigEnsuringFile: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfigEnsuringFile returned nil config")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}

	saved := LoadConfig(path)
	if saved.Viewer.HideFunctionBarWhenOpen != cfg.Viewer.HideFunctionBarWhenOpen {
		t.Fatal("saved config should round-trip defaults")
	}
}

func TestLoadConfigEnsuringFilePreservesInvalidExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	original := "viewer:\n  command_by_target: [broken\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	cfg, err := LoadConfigEnsuringFile(path)
	if err == nil {
		t.Fatal("expected invalid existing config to report an error")
	}
	if cfg == nil {
		t.Fatal("expected config value even when load fails")
	}
	if cfg.LoadIssue() == nil {
		t.Fatal("expected config to retain load issue metadata")
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("os.ReadFile: %v", readErr)
	}
	if string(data) != original {
		t.Fatalf("invalid config should remain untouched, got:\n%s", string(data))
	}
}

func TestSaveConfigCreatesBackupWhenReplacingExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")

	first := DefaultConfig()
	first.FavoriteLocations = []string{"/tmp/alpha"}
	if err := SaveConfig(path, first); err != nil {
		t.Fatalf("SaveConfig(first): %v", err)
	}

	second := DefaultConfig()
	second.FavoriteLocations = []string{"/tmp/bravo"}
	if err := SaveConfig(path, second); err != nil {
		t.Fatalf("SaveConfig(second): %v", err)
	}

	backup := LoadConfig(configBackupPath(path))
	if got, want := strings.Join(backup.FavoriteLocations, ","), "/tmp/alpha"; got != want {
		t.Fatalf("backup favorites=%q want %q", got, want)
	}

	current := LoadConfig(path)
	if got, want := strings.Join(current.FavoriteLocations, ","), "/tmp/bravo"; got != want {
		t.Fatalf("current favorites=%q want %q", got, want)
	}
}

func TestSaveConfigOmitsSSHAuthenticationSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")
	cfg := DefaultConfig()
	cfg.SSH.Setups = []SSHSetup{{
		Name:          "alice@example.test:22",
		Host:          "example.test",
		Port:          22,
		User:          "alice",
		Password:      "plain-password",
		KeyPath:       "/home/alice/.ssh/id_ed25519",
		KeyPassphrase: "plain-passphrase",
		CredentialID:  "credential-id",
	}}

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(data)
	for _, forbidden := range []string{"plain-password", "plain-passphrase", "password:", "key_passphrase:"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("saved config contains %q:\n%s", forbidden, text)
		}
	}
	if !strings.Contains(text, "credential_id: credential-id") {
		t.Fatalf("saved config is missing credential reference:\n%s", text)
	}
}

func TestRewriteConfigWithoutBackupRemovesSensitiveBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hexone.yaml")
	legacy := []byte("ssh:\n  setups:\n    - host: example.test\n      port: 22\n      user: alice\n      password: plain-password\n")
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(path+configBackupSuffix, legacy, 0o644); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	cfg, err := decodeConfigData(legacy)
	if err != nil {
		t.Fatalf("decodeConfigData: %v", err)
	}
	cfg.SSH.Setups[0].CredentialID = "credential-id"
	if err := RewriteConfigWithoutBackup(path, cfg); err != nil {
		t.Fatalf("RewriteConfigWithoutBackup: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rewritten config: %v", err)
	}
	if strings.Contains(string(data), "plain-password") {
		t.Fatalf("rewritten config retained secret:\n%s", data)
	}
	if _, err := os.Stat(path + configBackupSuffix); !os.IsNotExist(err) {
		t.Fatalf("sensitive backup still exists: %v", err)
	}
}

func TestLoadConfigEnsuringFileFallsBackToBackupWhenPrimaryInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hexone.yaml")

	first := DefaultConfig()
	first.FavoriteLocations = []string{"/tmp/alpha"}
	if err := SaveConfig(path, first); err != nil {
		t.Fatalf("SaveConfig(first): %v", err)
	}

	second := DefaultConfig()
	second.FavoriteLocations = []string{"/tmp/bravo"}
	if err := SaveConfig(path, second); err != nil {
		t.Fatalf("SaveConfig(second): %v", err)
	}

	corrupt := "viewer:\n  command_by_target: [broken\n"
	if err := os.WriteFile(path, []byte(corrupt), 0o644); err != nil {
		t.Fatalf("os.WriteFile(corrupt): %v", err)
	}

	cfg, err := LoadConfigEnsuringFile(path)
	if err == nil {
		t.Fatal("expected load to report primary config parse failure")
	}
	if cfg == nil {
		t.Fatal("expected recovered config when backup exists")
	}
	if cfg.LoadIssue() == nil {
		t.Fatal("expected recovered config to retain load issue metadata")
	}
	if got, want := strings.Join(cfg.FavoriteLocations, ","), "/tmp/alpha"; got != want {
		t.Fatalf("recovered favorites=%q want %q", got, want)
	}
	if !strings.Contains(err.Error(), configBackupPath(path)) {
		t.Fatalf("load error should mention recovery backup, got %q", err)
	}
}

func TestLegacyKeyBindingsAreIgnoredOnLoad(t *testing.T) {
	raw := `
key_bindings:
  focus_next_pane: f3
  move_up: home
  view: f11
`

	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(raw), cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	cfg.normalize()

	out := string(mustMarshalConfig(t, cfg))
	if strings.Contains(out, "key_bindings:") {
		t.Fatalf("serialized config should drop legacy key_bindings block:\n%s", out)
	}
}

func TestNormalizeColors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Colors.FilePaneBackground = "161e2"
	cfg.Colors.FilePaneText = "badtext"
	cfg.Colors.Hover = "hover"
	cfg.Colors.HoverText = "hovertext"
	cfg.Colors.PopupHover = "popuphover"
	cfg.Colors.PopupHoverText = "popuphovertext"
	cfg.Colors.Selection = "bad"
	cfg.Colors.SelectionText = "oops"
	cfg.Colors.SelectedFiles = "selected"
	cfg.Colors.SelectedFilesText = "oops"
	cfg.Colors.FocusedSelected = "focused"
	cfg.Colors.FocusedSelectedText = "focusedtext"
	cfg.Colors.CurrentDirBg = "currdir"
	cfg.Colors.CurrentDirText = "currtext"
	cfg.Colors.ScrollbarThumb = "scrollthumb"
	cfg.Colors.ScrollbarTrack = "scrolltrack"

	cfg.normalize()

	if cfg.Colors.FilePaneBackground != DefaultFilePaneBackgroundHex {
		t.Fatalf("FilePaneBackground=%q, want %q", cfg.Colors.FilePaneBackground, DefaultFilePaneBackgroundHex)
	}
	if cfg.Colors.FilePaneText != DefaultFilePaneTextHex {
		t.Fatalf("FilePaneText=%q, want %q", cfg.Colors.FilePaneText, DefaultFilePaneTextHex)
	}
	if cfg.Colors.Hover != DefaultFilePaneHoverHex {
		t.Fatalf("Hover=%q, want %q", cfg.Colors.Hover, DefaultFilePaneHoverHex)
	}
	if cfg.Colors.HoverText != TransparentColor {
		t.Fatalf("HoverText=%q, want %q", cfg.Colors.HoverText, TransparentColor)
	}
	if cfg.Colors.PopupHover != DefaultPopupHoverHex {
		t.Fatalf("PopupHover=%q, want %q", cfg.Colors.PopupHover, DefaultPopupHoverHex)
	}
	if cfg.Colors.PopupHoverText != DefaultPopupHoverTextHex {
		t.Fatalf("PopupHoverText=%q, want %q", cfg.Colors.PopupHoverText, DefaultPopupHoverTextHex)
	}
	if cfg.Colors.Selection != DefaultFilePaneSelectionHex {
		t.Fatalf("Selection=%q, want %q", cfg.Colors.Selection, DefaultFilePaneSelectionHex)
	}
	if cfg.Colors.SelectionText != TransparentColor {
		t.Fatalf("SelectionText=%q, want %q", cfg.Colors.SelectionText, TransparentColor)
	}
	if cfg.Colors.SelectedFiles != DefaultFilePaneSelectedFilesHex {
		t.Fatalf("SelectedFiles=%q, want %q", cfg.Colors.SelectedFiles, DefaultFilePaneSelectedFilesHex)
	}
	if cfg.Colors.SelectedFilesText != TransparentColor {
		t.Fatalf("SelectedFilesText=%q, want %q", cfg.Colors.SelectedFilesText, TransparentColor)
	}
	if cfg.Colors.FocusedSelected != DefaultFilePaneFocusedSelectedHex {
		t.Fatalf("FocusedSelected=%q, want %q", cfg.Colors.FocusedSelected, DefaultFilePaneFocusedSelectedHex)
	}
	if cfg.Colors.FocusedSelectedText != TransparentColor {
		t.Fatalf("FocusedSelectedText=%q, want %q", cfg.Colors.FocusedSelectedText, TransparentColor)
	}
	if cfg.Colors.CurrentDirBg != DefaultCurrentDirBackgroundHex {
		t.Fatalf("CurrentDirBg=%q, want %q", cfg.Colors.CurrentDirBg, DefaultCurrentDirBackgroundHex)
	}
	if cfg.Colors.CurrentDirText != DefaultCurrentDirTextHex {
		t.Fatalf("CurrentDirText=%q, want %q", cfg.Colors.CurrentDirText, DefaultCurrentDirTextHex)
	}
	if cfg.Colors.ScrollbarThumb != "" {
		t.Fatalf("ScrollbarThumb=%q, want empty optional color", cfg.Colors.ScrollbarThumb)
	}
	if cfg.Colors.ScrollbarTrack != "" {
		t.Fatalf("ScrollbarTrack=%q, want empty optional color", cfg.Colors.ScrollbarTrack)
	}
}

func TestNormalizeColorsPreservesTransparentRowText(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Colors.HoverText = "Transparent"
	cfg.Colors.SelectionText = "transparent"
	cfg.Colors.SelectedFilesText = "TRANSPARENT"
	cfg.Colors.FocusedSelectedText = "transparent"
	cfg.Colors.PopupHoverText = "transparent"

	cfg.normalize()

	if cfg.Colors.HoverText != TransparentColor {
		t.Fatalf("HoverText=%q want %q", cfg.Colors.HoverText, TransparentColor)
	}
	if cfg.Colors.SelectionText != TransparentColor {
		t.Fatalf("SelectionText=%q want %q", cfg.Colors.SelectionText, TransparentColor)
	}
	if cfg.Colors.SelectedFilesText != TransparentColor {
		t.Fatalf("SelectedFilesText=%q want %q", cfg.Colors.SelectedFilesText, TransparentColor)
	}
	if cfg.Colors.FocusedSelectedText != TransparentColor {
		t.Fatalf("FocusedSelectedText=%q want %q", cfg.Colors.FocusedSelectedText, TransparentColor)
	}
	if cfg.Colors.PopupHoverText != DefaultPopupHoverTextHex {
		t.Fatalf("PopupHoverText=%q want fallback %q", cfg.Colors.PopupHoverText, DefaultPopupHoverTextHex)
	}
}

func TestDefaultConfigPreservesFilenameColorsForHoverAndFocus(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Colors.HoverText != TransparentColor {
		t.Fatalf("HoverText=%q want %q", cfg.Colors.HoverText, TransparentColor)
	}
	if cfg.Colors.SelectionText != TransparentColor {
		t.Fatalf("SelectionText=%q want %q", cfg.Colors.SelectionText, TransparentColor)
	}
	if cfg.Colors.SelectedFilesText != DefaultFilePaneSelectedTextHex {
		t.Fatalf("SelectedFilesText=%q want %q", cfg.Colors.SelectedFilesText, DefaultFilePaneSelectedTextHex)
	}
	if cfg.Colors.FocusedSelectedText != DefaultFilePaneFocusedSelectedTextHex {
		t.Fatalf("FocusedSelectedText=%q want %q", cfg.Colors.FocusedSelectedText, DefaultFilePaneFocusedSelectedTextHex)
	}
}

func TestNormalizeViewerThemeOverrides(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Viewer.Background = "viewerbg"
	cfg.Viewer.Text = "viewtext"
	cfg.Viewer.Selection = "viewsel"
	cfg.Viewer.HexSelection = "hexsel"
	cfg.Viewer.HexOffsetText = "offset"
	cfg.Viewer.HexBytesText = "bytes"
	cfg.Viewer.HexASCIIText = "ascii"

	cfg.normalize()

	if cfg.Viewer.Background != DefaultViewerBackgroundHex {
		t.Fatalf("Viewer.Background=%q, want %q", cfg.Viewer.Background, DefaultViewerBackgroundHex)
	}
	if cfg.Viewer.Text != DefaultViewerTextHex {
		t.Fatalf("Viewer.Text=%q, want %q", cfg.Viewer.Text, DefaultViewerTextHex)
	}
	if cfg.Viewer.Selection != DefaultViewerSelectionHex {
		t.Fatalf("Viewer.Selection=%q, want %q", cfg.Viewer.Selection, DefaultViewerSelectionHex)
	}
	if cfg.Viewer.HexSelection != "" || cfg.Viewer.HexOffsetText != "" || cfg.Viewer.HexBytesText != "" || cfg.Viewer.HexASCIIText != "" {
		t.Fatal("invalid optional hex colors should normalize to empty")
	}

	cfg.Viewer.Background = "#112233"
	cfg.Viewer.Text = "aabbcc"
	cfg.Viewer.Selection = "3355dd"
	cfg.Viewer.HexSelection = "223344"
	cfg.Viewer.HexOffsetText = "445566"
	cfg.Viewer.HexBytesText = "#778899"
	cfg.Viewer.HexASCIIText = "aabbcc"
	cfg.normalize()

	if cfg.Viewer.Background != "#112233" {
		t.Fatalf("Viewer.Background=%q, want %q", cfg.Viewer.Background, "#112233")
	}
	if cfg.Viewer.Text != "#AABBCC" {
		t.Fatalf("Viewer.Text=%q, want %q", cfg.Viewer.Text, "#AABBCC")
	}
	if cfg.Viewer.Selection != "#3355DD" {
		t.Fatalf("Viewer.Selection=%q, want %q", cfg.Viewer.Selection, "#3355DD")
	}
	if cfg.Viewer.HexSelection != "#223344" {
		t.Fatalf("Viewer.HexSelection=%q, want #223344", cfg.Viewer.HexSelection)
	}
	if cfg.Viewer.HexOffsetText != "#445566" || cfg.Viewer.HexBytesText != "#778899" || cfg.Viewer.HexASCIIText != "#AABBCC" {
		t.Fatalf("normalized hex text colors = %q, %q, %q", cfg.Viewer.HexOffsetText, cfg.Viewer.HexBytesText, cfg.Viewer.HexASCIIText)
	}
}

func TestNormalizeFilenameColors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Colors.Filenames.Target = "dirs"
	cfg.Colors.Filenames.Text = "oops"
	cfg.Colors.Filenames.Icon = "mystery"
	cfg.Colors.Filenames.AgeRules = []FilenameAgeRule{
		{MaxAge: "24h", Target: "regular", Text: "aabbcc", Icon: "schedule"},
		{MaxAge: "bad", Text: "#112233", Icon: "lock"},
		{MaxAge: "1w", Text: "", Icon: ""},
	}
	cfg.Colors.Filenames.PermissionRules = []FilenamePermissionRule{
		{Permissions: "755", Match: "partial", Target: "folders", Text: "#556677", Icon: "lock"},
		{Permissions: "oops", Text: "#123456", Icon: "document"},
		{Permissions: "0644", Text: "bad", Icon: "description"},
		{Permissions: "0000", Match: "any", Text: "#654321", Icon: "document"},
	}
	cfg.Colors.Filenames.ExtensionRules = []FilenameExtensionRule{
		{Extension: "GO", Text: "abcdef", Icon: "edit"},
		{Extension: "*.tar.gz", Text: "", Icon: "archive"},
		{Extension: "bad/name", Text: "#123456", Icon: "document"},
	}
	cfg.Colors.Filenames.SizeRules = []FilenameSizeRule{
		{Size: "10mb", Match: "max", Target: "dir", Text: "102030", Icon: "movie"},
		{Size: "1g", Text: "", Icon: ""},
		{Size: "oops", Text: "#123456", Icon: "lock"},
	}

	cfg.normalize()

	if cfg.Colors.Filenames.Text != "" {
		t.Fatalf("Filenames.Text=%q want empty", cfg.Colors.Filenames.Text)
	}
	if cfg.Colors.Filenames.Icon != "" {
		t.Fatalf("Filenames.Icon=%q want empty", cfg.Colors.Filenames.Icon)
	}
	if cfg.Colors.Filenames.Target != FilenameTargetDirs {
		t.Fatalf("Filenames.Target=%q want %q", cfg.Colors.Filenames.Target, FilenameTargetDirs)
	}
	if len(cfg.Colors.Filenames.AgeRules) != 1 {
		t.Fatalf("len(Filenames.AgeRules)=%d want 1", len(cfg.Colors.Filenames.AgeRules))
	}
	if got := cfg.Colors.Filenames.AgeRules[0].MaxAge; got != "1d" {
		t.Fatalf("AgeRules[0].MaxAge=%q want %q", got, "1d")
	}
	if got := cfg.Colors.Filenames.AgeRules[0].Text; got != "#AABBCC" {
		t.Fatalf("AgeRules[0].Text=%q want %q", got, "#AABBCC")
	}
	if got := cfg.Colors.Filenames.AgeRules[0].Icon; got != FilenameIconRecent {
		t.Fatalf("AgeRules[0].Icon=%q want %q", got, FilenameIconRecent)
	}
	if got := cfg.Colors.Filenames.AgeRules[0].Target; got != FilenameTargetFiles {
		t.Fatalf("AgeRules[0].Target=%q want %q", got, FilenameTargetFiles)
	}
	if len(cfg.Colors.Filenames.PermissionRules) != 2 {
		t.Fatalf("len(Filenames.PermissionRules)=%d want 2", len(cfg.Colors.Filenames.PermissionRules))
	}
	if got := cfg.Colors.Filenames.PermissionRules[0].Permissions; got != "0755" {
		t.Fatalf("PermissionRules[0].Permissions=%q want %q", got, "0755")
	}
	if got := cfg.Colors.Filenames.PermissionRules[0].Match; got != FilenamePermissionMatchAny {
		t.Fatalf("PermissionRules[0].Match=%q want %q", got, FilenamePermissionMatchAny)
	}
	if got := cfg.Colors.Filenames.PermissionRules[0].Icon; got != FilenameIconLocked {
		t.Fatalf("PermissionRules[0].Icon=%q want %q", got, FilenameIconLocked)
	}
	if got := cfg.Colors.Filenames.PermissionRules[0].Target; got != FilenameTargetDirs {
		t.Fatalf("PermissionRules[0].Target=%q want %q", got, FilenameTargetDirs)
	}
	if got := cfg.Colors.Filenames.PermissionRules[1].Permissions; got != "0644" {
		t.Fatalf("PermissionRules[1].Permissions=%q want %q", got, "0644")
	}
	if got := cfg.Colors.Filenames.PermissionRules[1].Match; got != FilenamePermissionMatchExact {
		t.Fatalf("PermissionRules[1].Match=%q want exact", got)
	}
	if got := cfg.Colors.Filenames.PermissionRules[1].Text; got != "" {
		t.Fatalf("PermissionRules[1].Text=%q want empty", got)
	}
	if got := cfg.Colors.Filenames.PermissionRules[1].Icon; got != FilenameIconDocument {
		t.Fatalf("PermissionRules[1].Icon=%q want %q", got, FilenameIconDocument)
	}
	if len(cfg.Colors.Filenames.ExtensionRules) != 2 {
		t.Fatalf("len(Filenames.ExtensionRules)=%d want 2", len(cfg.Colors.Filenames.ExtensionRules))
	}
	if got := cfg.Colors.Filenames.ExtensionRules[0].Extension; got != ".go" {
		t.Fatalf("ExtensionRules[0].Extension=%q want %q", got, ".go")
	}
	if got := cfg.Colors.Filenames.ExtensionRules[0].Text; got != "#ABCDEF" {
		t.Fatalf("ExtensionRules[0].Text=%q want %q", got, "#ABCDEF")
	}
	if got := cfg.Colors.Filenames.ExtensionRules[0].Icon; got != FilenameIconCode {
		t.Fatalf("ExtensionRules[0].Icon=%q want %q", got, FilenameIconCode)
	}
	if got := cfg.Colors.Filenames.ExtensionRules[1].Extension; got != ".tar.gz" {
		t.Fatalf("ExtensionRules[1].Extension=%q want %q", got, ".tar.gz")
	}
	if got := cfg.Colors.Filenames.ExtensionRules[1].Icon; got != FilenameIconArchive {
		t.Fatalf("ExtensionRules[1].Icon=%q want %q", got, FilenameIconArchive)
	}
	if len(cfg.Colors.Filenames.SizeRules) != 1 {
		t.Fatalf("len(Filenames.SizeRules)=%d want 1", len(cfg.Colors.Filenames.SizeRules))
	}
	if got := cfg.Colors.Filenames.SizeRules[0].Size; got != "10m" {
		t.Fatalf("SizeRules[0].Size=%q want %q", got, "10m")
	}
	if got := cfg.Colors.Filenames.SizeRules[0].Match; got != FilenameSizeMatchAtMost {
		t.Fatalf("SizeRules[0].Match=%q want %q", got, FilenameSizeMatchAtMost)
	}
	if got := cfg.Colors.Filenames.SizeRules[0].Text; got != "#102030" {
		t.Fatalf("SizeRules[0].Text=%q want %q", got, "#102030")
	}
	if got := cfg.Colors.Filenames.SizeRules[0].Icon; got != FilenameIconVideo {
		t.Fatalf("SizeRules[0].Icon=%q want %q", got, FilenameIconVideo)
	}
	if got := cfg.Colors.Filenames.SizeRules[0].Target; got != "" {
		t.Fatalf("SizeRules[0].Target=%q want empty file-only rule", got)
	}
}

func TestNormalizeFilenameAgeRulesSortsAndDedupes(t *testing.T) {
	got := NormalizeFilenameAgeRules([]FilenameAgeRule{
		{MaxAge: "1w", Text: "#334455"},
		{MaxAge: "24h", Text: "#112233"},
		{MaxAge: "1d", Text: "#556677", Icon: "schedule"},
		{MaxAge: "45m", Icon: "lock"},
	})

	if len(got) != 3 {
		t.Fatalf("len(NormalizeFilenameAgeRules)=%d want 3", len(got))
	}
	if got[0].MaxAge != "45m" || got[1].MaxAge != "1d" || got[2].MaxAge != "1w" {
		t.Fatalf("NormalizeFilenameAgeRules order=%#v want 45m, 1d, 1w", got)
	}
	if got[1].Text != "#556677" || got[1].Icon != FilenameIconRecent {
		t.Fatalf("NormalizeFilenameAgeRules duplicate merge=%#v want last 1d rule", got[1])
	}
}

func TestNormalizeFilenameRulesTargetsOnlyApplyToAgeAndPermissions(t *testing.T) {
	age := NormalizeFilenameAgeRules([]FilenameAgeRule{
		{MaxAge: "1d", Target: "files", Text: "#112233"},
		{MaxAge: "1d", Target: "dirs", Text: "#445566"},
		{MaxAge: "1d", Target: "both", Text: "#778899"},
	})
	if len(age) != 3 {
		t.Fatalf("len(NormalizeFilenameAgeRules)=%d want 3 target variants", len(age))
	}
	if age[0].Target != FilenameTargetFiles || age[1].Target != FilenameTargetDirs || age[2].Target != FilenameTargetBoth {
		t.Fatalf("age targets=%#v want files, dirs, both", age)
	}

	ext := NormalizeFilenameExtensionRules([]FilenameExtensionRule{
		{Extension: ".go", Target: "files", Text: "#112233"},
		{Extension: ".go", Target: "dirs", Text: "#445566"},
		{Extension: ".go", Target: "both", Text: "#778899"},
	})
	if len(ext) != 1 {
		t.Fatalf("len(NormalizeFilenameExtensionRules)=%d want 1 file-only rule", len(ext))
	}
	if ext[0].Target != "" || ext[0].Text != "#778899" {
		t.Fatalf("extension rule=%#v want targetless last rule", ext[0])
	}

	size := NormalizeFilenameSizeRules([]FilenameSizeRule{
		{Size: "1m", Match: FilenameSizeMatchAtMost, Target: "files", Text: "#112233"},
		{Size: "1m", Match: FilenameSizeMatchAtMost, Target: "dirs", Text: "#445566"},
	})
	if len(size) != 1 {
		t.Fatalf("len(NormalizeFilenameSizeRules)=%d want 1 file-only rule", len(size))
	}
	if size[0].Target != "" || size[0].Text != "#445566" {
		t.Fatalf("size rule=%#v want targetless last rule", size[0])
	}
}

func TestNormalizeFilenameIconSupportsExtendedChoices(t *testing.T) {
	cases := map[string]string{
		"music":       FilenameIconAudio,
		"book":        FilenameIconBook,
		"spreadsheet": FilenameIconTable,
		"application": FilenameIconApp,
		"shortcut":    FilenameIconLink,
	}

	for raw, want := range cases {
		if got := NormalizeFilenameIcon(raw); got != want {
			t.Fatalf("NormalizeFilenameIcon(%q)=%q want %q", raw, got, want)
		}
	}
}

func TestFilenamePermissionAndSizeMatchesSupportPartialRules(t *testing.T) {
	if !FilenamePermissionMatches("0755", FilenamePermissionRule{Permissions: "0111", Match: FilenamePermissionMatchAny}) {
		t.Fatal("any-match permissions should detect executables")
	}
	if !FilenamePermissionMatches("0444", FilenamePermissionRule{Permissions: "0222", Match: FilenamePermissionMatchNone}) {
		t.Fatal("none-match permissions should detect readonly files")
	}
	if FilenamePermissionMatches("0644", FilenamePermissionRule{Permissions: "0111", Match: FilenamePermissionMatchAny}) {
		t.Fatal("any-match permissions should not match non-executables")
	}
	if !FilenamePermissionMatches("0644", FilenamePermissionRule{Permissions: "0644"}) {
		t.Fatal("exact permissions should still work")
	}
	if !FilenameSizeMatches(10<<20, FilenameSizeRule{Size: "10m"}) {
		t.Fatal("at-least size match should include equal values")
	}
	if !FilenameSizeMatches(512, FilenameSizeRule{Size: "1k", Match: FilenameSizeMatchAtMost}) {
		t.Fatal("at-most size match should include smaller values")
	}
	if FilenameSizeMatches(2048, FilenameSizeRule{Size: "1k", Match: FilenameSizeMatchAtMost}) {
		t.Fatal("at-most size match should reject larger values")
	}
}

func TestNormalizeTerminalSnippetsUsesExplicitScopes(t *testing.T) {
	got := NormalizeTerminalSnippets([]TerminalSnippet{
		{Name: "Global", Command: "date", Scope: TerminalSnippetScopeGlobal, Context: "/ignored"},
		{Name: "Repo", Command: "go test ./...", Scope: TerminalSnippetScopeRepository, Context: "/src/app"},
		{Name: "Invalid", Command: "pwd", Scope: TerminalSnippetScopeDirectory},
		{Name: "Multiline", Command: "echo one\necho two", Scope: TerminalSnippetScopeGlobal},
		{Name: "Repo", Command: "go test -race ./...", Scope: TerminalSnippetScopeRepository, Context: "/src/app"},
	})
	if len(got) != 2 {
		t.Fatalf("snippet count=%d want 2", len(got))
	}
	if got[0].Scope != TerminalSnippetScopeGlobal || got[0].Context != "" {
		t.Fatalf("global snippet=%+v", got[0])
	}
	if got[1].Command != "go test -race ./..." {
		t.Fatalf("duplicate scoped snippet was not replaced: %+v", got[1])
	}
}

func mustMarshalConfig(t *testing.T, cfg *Config) []byte {
	t.Helper()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return data
}
