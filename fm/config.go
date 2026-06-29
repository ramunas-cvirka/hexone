// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package fm

import (
	"errors"
	"fmt"
	resources "hexone"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v4"
)

const (
	defaultApproxCharPx       = 8
	defaultNameKeepStartChars = 6
	defaultNameCompactMarker  = ".."
	defaultColumnPadDp        = 4
	// Brief mode already has left/right cell padding; keep the explicit
	// inter-column gap at zero so the visible separation stays near 1ch.
	defaultBriefGapDp         = 0
	defaultNameIconReserveDp  = 14
	defaultNameChars          = 20.0
	defaultBriefChars         = 16.0
	defaultNameMinWidthDp     = 52
	defaultPermWidthChars     = 10.5
	defaultSizeWidthChars     = 10.5
	defaultDateWidthChars     = 15.0
	defaultNameTextReserveDp  = defaultApproxCharPx/2 + 2
	configBackupSuffix        = ".bak"
	defaultTerminalHeightRows = 24
	minTerminalHeightRows     = 4
	maxTerminalHeightRows     = 80
	defaultTabMinWidthDp      = 72
	defaultTabFixedWidthDp    = 118
	defaultTabMaxWidthDp      = 168
)

const (
	ViewerFileEncodingAuto    = "auto"
	ViewerFileEncodingUTF8    = "utf-8"
	ViewerFileEncodingUTF16LE = "utf-16le"
	ViewerFileEncodingUTF16BE = "utf-16be"
	ViewerFileEncodingCP437   = "cp437"
)

type NameCompact struct {
	KeepStartChars int    `yaml:"keep_start_chars"`
	Marker         string `yaml:"marker"`
}

type nameCompactCompat struct {
	KeepStartChars int    `yaml:"keep_start_chars"`
	MinHead        int    `yaml:"min_head"`
	Marker         string `yaml:"marker"`
}

func (n *NameCompact) UnmarshalYAML(node *yaml.Node) error {
	var raw nameCompactCompat
	if err := node.Decode(&raw); err != nil {
		return err
	}
	n.KeepStartChars = raw.KeepStartChars
	if n.KeepStartChars < 1 {
		n.KeepStartChars = raw.MinHead
	}
	n.Marker = raw.Marker
	return nil
}

type ColumnWidths struct {
	NameChars        float32  `yaml:"full_chars"`
	BriefChars       float32  `yaml:"brief_chars"`
	FullDropPriority []string `yaml:"full_drop_priority"`
	ShowPermissions  bool     `yaml:"show_permissions"`
	PermissionFormat string   `yaml:"permission_format"`
}

type columnWidthsCompat struct {
	FullChars        float32  `yaml:"full_chars"`
	NameChars        float32  `yaml:"name_chars"`
	BriefChars       float32  `yaml:"brief_chars"`
	FullDropPriority []string `yaml:"full_drop_priority"`
	ShowPermissions  *bool    `yaml:"show_permissions"`
	PermissionFormat string   `yaml:"permission_format"`

	NameWidthDp    int `yaml:"name_width_dp"`
	NameMinWidthDp int `yaml:"name_min_width_dp"`
	PermWidthDp    int `yaml:"perm_width_dp"`
	FullPadDp      int `yaml:"full_pad_dp"`
	SizeWidthDp    int `yaml:"size_width_dp"`
	DateWidthDp    int `yaml:"date_width_dp"`
	BriefWidthDp   int `yaml:"brief_width_dp"`
	BriefGapDp     int `yaml:"brief_gap_dp"`
}

func (c *ColumnWidths) UnmarshalYAML(node *yaml.Node) error {
	raw := columnWidthsCompat{}
	if err := node.Decode(&raw); err != nil {
		return err
	}

	out := defaultColumnWidths()
	if raw.FullChars > 0 {
		out.NameChars = normalizeColumnChars(raw.FullChars, defaultNameChars)
	} else if raw.NameChars > 0 {
		out.NameChars = normalizeColumnChars(raw.NameChars, defaultNameChars)
	} else if raw.NameWidthDp > 0 {
		out.NameChars = legacyWidthDpToChars(raw.NameWidthDp, raw.FullPadDp, true)
	}
	if raw.BriefChars > 0 {
		out.BriefChars = normalizeColumnChars(raw.BriefChars, defaultBriefChars)
	} else if raw.BriefWidthDp > 0 {
		out.BriefChars = legacyWidthDpToChars(raw.BriefWidthDp, raw.FullPadDp, true)
	}
	if raw.ShowPermissions != nil {
		out.ShowPermissions = *raw.ShowPermissions
	}
	if strings.TrimSpace(raw.PermissionFormat) != "" {
		out.PermissionFormat = raw.PermissionFormat
	}
	if len(raw.FullDropPriority) > 0 {
		out.FullDropPriority = append([]string(nil), raw.FullDropPriority...)
	}

	*c = out
	return nil
}

type SortConfig struct {
	DefaultKey       string            `yaml:"default_key"`
	Descending       bool              `yaml:"descending"`
	DirectoriesFirst bool              `yaml:"directories_first"`
	PerDir           map[string]string `yaml:"per_dir,omitempty"`
}

type TerminalConfig struct {
	HeightRows int     `yaml:"height_rows"`
	Typeface   string  `yaml:"typeface"`
	FontSizeSp float32 `yaml:"font_size_sp"`
}

type TabsConfig struct {
	WidthMode         string  `yaml:"width_mode"`
	MinWidthDp        int     `yaml:"min_width_dp"`
	FixedWidthDp      int     `yaml:"fixed_width_dp"`
	MaxWidthDp        int     `yaml:"max_width_dp"`
	Typeface          string  `yaml:"typeface"`
	FontSizeSp        float32 `yaml:"font_size_sp"`
	AlternatingColors bool    `yaml:"alternating_colors"`
	Color             string  `yaml:"color,omitempty"`
	AltColor          string  `yaml:"alt_color,omitempty"`
	ActiveColor       string  `yaml:"active_color,omitempty"`
}

type InterfaceConfig struct {
	Typeface   string  `yaml:"typeface"`
	FontSizeSp float32 `yaml:"font_size_sp"`
}

func NormalizeViewerShell(raw string) string {
	shell, ok := NormalizeKnownViewerShell(raw)
	if !ok {
		return "auto"
	}
	return shell
}

func NormalizeKnownViewerShell(raw string) (string, bool) {
	shell := strings.TrimSpace(raw)
	lower := strings.ToLower(shell)
	switch lower {
	case "", "auto":
		return "auto", true
	case "sh", "bash":
		return "sh", true
	case "pwsh", "pwsh.exe":
		return "pwsh", true
	case "powershell", "powershell.exe":
		return "powershell", true
	case "cmd", "cmd.exe":
		return "cmd", true
	case "wsl", "wsl.exe":
		return "wsl", true
	}
	if strings.HasPrefix(lower, "wsl:") {
		distro := strings.TrimSpace(shell[len("wsl:"):])
		if distro == "" {
			return "wsl", true
		}
		return "wsl:" + distro, true
	}
	return "", false
}

func ViewerShellIsWSL(shell string) bool {
	normalized, ok := NormalizeKnownViewerShell(shell)
	if !ok {
		return false
	}
	lower := strings.ToLower(normalized)
	return lower == "wsl" || strings.HasPrefix(lower, "wsl:")
}

func ViewerShellWSLDistro(shell string) string {
	normalized, ok := NormalizeKnownViewerShell(shell)
	if !ok {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(normalized), "wsl:") {
		return ""
	}
	return strings.TrimSpace(normalized[len("wsl:"):])
}

func NormalizeSortKey(raw string) string {
	key, ok := normalizeSortKey(raw)
	if !ok {
		return "name"
	}
	return key
}

func normalizeSortKey(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "name", "filename", "file":
		return "name", true
	case "ext", "extension", "type":
		return "ext", true
	case "size":
		return "size", true
	case "date", "time", "datetime":
		return "date", true
	default:
		return "", false
	}
}

func SortOrderCode(key string, descending bool) string {
	prefix := "n"
	switch NormalizeSortKey(key) {
	case "ext":
		prefix = "e"
	case "size":
		prefix = "s"
	case "date":
		prefix = "d"
	}
	if descending {
		return prefix + "-"
	}
	return prefix + "+"
}

func ParseSortOrderCode(raw string) (key string, descending bool, ok bool) {
	token := strings.ToLower(strings.TrimSpace(raw))
	if token == "" {
		return "", false, false
	}
	if len(token) == 2 && (token[1] == '+' || token[1] == '-') {
		switch token[0] {
		case 'n':
			key = "name"
		case 'e':
			key = "ext"
		case 's':
			key = "size"
		case 'd':
			key = "date"
		default:
			return "", false, false
		}
		return key, token[1] == '-', true
	}

	parts := strings.FieldsFunc(token, func(r rune) bool {
		return r == ':' || r == ',' || r == ' '
	})
	if len(parts) == 0 {
		return "", false, false
	}
	key, ok = normalizeSortKey(parts[0])
	if !ok {
		return "", false, false
	}
	if len(parts) == 1 {
		return key, false, true
	}
	switch parts[1] {
	case "desc", "descending", "down", "-":
		return key, true, true
	case "asc", "ascending", "up", "+":
		return key, false, true
	default:
		return "", false, false
	}
}

func SortOrderIsDefault(cfg SortConfig, key string, descending bool) bool {
	return NormalizeSortKey(cfg.DefaultKey) == NormalizeSortKey(key) && cfg.Descending == descending
}

func NormalizeSortPerDir(raw map[string]string, cfg SortConfig) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]string, len(raw))
	for rawDir, rawOrder := range raw {
		dir := strings.TrimSpace(rawDir)
		if dir == "" {
			continue
		}
		dir = filepath.Clean(dir)
		if dir == "." {
			continue
		}
		key, descending, ok := ParseSortOrderCode(rawOrder)
		if !ok || SortOrderIsDefault(cfg, key, descending) {
			continue
		}
		out[dir] = SortOrderCode(key, descending)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func NormalizeTerminalHeightRows(rows int) int {
	if rows <= 0 {
		return defaultTerminalHeightRows
	}
	if rows < minTerminalHeightRows {
		return minTerminalHeightRows
	}
	if rows > maxTerminalHeightRows {
		return maxTerminalHeightRows
	}
	return rows
}

func NormalizeTabWidthMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fixed", "uniform", "same":
		return "fixed"
	default:
		return "variable"
	}
}

func NormalizeTabWidthDp(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	if v < 44 {
		return 44
	}
	if v > 320 {
		return 320
	}
	return v
}

type GeneralConfig struct {
	Typeface              string  `yaml:"typeface"`
	FontSizeSp            float32 `yaml:"font_size_sp"`
	DimInactivePanes      bool    `yaml:"dim_inactive_panes"`
	OpenFavoritesInNewTab bool    `yaml:"open_favorites_in_new_tab"`
}

func (g *GeneralConfig) UnmarshalYAML(node *yaml.Node) error {
	var raw struct {
		Typeface              string  `yaml:"typeface"`
		FontSizeSp            float32 `yaml:"font_size_sp"`
		DimInactivePanes      bool    `yaml:"dim_inactive_panes"`
		OpenFavoritesInNewTab *bool   `yaml:"open_favorites_in_new_tab"`
	}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	g.Typeface = raw.Typeface
	g.FontSizeSp = raw.FontSizeSp
	g.DimInactivePanes = raw.DimInactivePanes
	g.OpenFavoritesInNewTab = true
	if raw.OpenFavoritesInNewTab != nil {
		g.OpenFavoritesInNewTab = *raw.OpenFavoritesInNewTab
	}
	return nil
}

type legacyFontConfig struct {
	Typeface string  `yaml:"typeface"`
	SizeSp   float32 `yaml:"size_sp"`
}

type ViewerAssociation struct {
	Extension string `yaml:"extension"`
	AppPath   string `yaml:"app_path"`
}

type ViewerCommandRule struct {
	Pattern string `yaml:"pattern"`
	Command string `yaml:"command"`
}

type CustomCommand struct {
	Slot    int    `yaml:"slot,omitempty"`
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
}

type AssociationProgram struct {
	AppPath    string   `yaml:"app_path"`
	Extensions []string `yaml:"extensions"`
}

type associationProgramYAML struct {
	AppPath    string `yaml:"app_path"`
	Extensions string `yaml:"extensions"`
}

func (a *AssociationProgram) UnmarshalYAML(node *yaml.Node) error {
	var raw struct {
		AppPath    string `yaml:"app_path"`
		Extensions any    `yaml:"extensions"`
	}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	a.AppPath = raw.AppPath
	a.Extensions = parseAssociationProgramExtensions(raw.Extensions)
	return nil
}

func (a AssociationProgram) MarshalYAML() (any, error) {
	return associationProgramYAML{
		AppPath:    NormalizeViewerAssociationAppPath(a.AppPath),
		Extensions: associationProgramExtensionsCSV(a.Extensions),
	}, nil
}

type ViewerConfig struct {
	FileEncoding            string              `yaml:"file_encoding"`
	Typeface                string              `yaml:"typeface"`
	Background              string              `yaml:"background,omitempty"`
	Text                    string              `yaml:"text,omitempty"`
	Selection               string              `yaml:"selection,omitempty"`
	SmoothScrolling         bool                `yaml:"smooth_scrolling"`
	Shell                   string              `yaml:"shell"`
	Command                 string              `yaml:"command"`
	RemoteSearchMode        string              `yaml:"remote_search_mode"`
	RemoteSearchCommand     string              `yaml:"remote_search_command"`
	Associations            []ViewerAssociation `yaml:"associations,omitempty"`
	AssociatedExtensions    []string            `yaml:"associated_extensions,omitempty"`
	CommandRules            []ViewerCommandRule `yaml:"command_rules,omitempty"`
	CommandByTarget         map[string]string   `yaml:"command_by_target"`
	CommandHistory          []string            `yaml:"command_history"`
	WordSelectRegex         string              `yaml:"word_select_regex"`
	FontSizeSp              float32             `yaml:"font_size_sp"`
	WordWrap                bool                `yaml:"word_wrap"`
	MaxReadMB               float32             `yaml:"max_read_mb"`
	CommandAutoRefresh      bool                `yaml:"command_auto_refresh"`
	CommandRefreshMs        int                 `yaml:"command_refresh_ms"`
	HideFunctionBarWhenOpen bool                `yaml:"hide_function_bar_when_open"`
}

type SSHSetup struct {
	Name          string `yaml:"name"`
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	User          string `yaml:"user"`
	Password      string `yaml:"password"`
	KeyPath       string `yaml:"key_path"`
	KeyPassphrase string `yaml:"key_passphrase"`
}

type SSHConfig struct {
	Setups []SSHSetup `yaml:"setups"`
}

type Config struct {
	DateFormats       []string             `yaml:"date_formats"`
	FavoriteLocations []string             `yaml:"favorite_locations"`
	NameCompact       NameCompact          `yaml:"name_compact"`
	Columns           ColumnWidths         `yaml:"columns"`
	Sort              SortConfig           `yaml:"sort"`
	Terminal          TerminalConfig       `yaml:"terminal"`
	Tabs              TabsConfig           `yaml:"tabs"`
	Interface         InterfaceConfig      `yaml:"interface"`
	General           GeneralConfig        `yaml:"general"`
	Colors            ColorsConfig         `yaml:"colors"`
	Associations      []AssociationProgram `yaml:"associations,omitempty"`
	CustomCommands    []CustomCommand      `yaml:"custom_commands,omitempty"`
	Viewer            ViewerConfig         `yaml:"viewer"`
	SSH               SSHConfig            `yaml:"ssh"`

	loadIssue error
}

func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	raw := struct {
		DateFormats       []string             `yaml:"date_formats"`
		FavoriteLocations []string             `yaml:"favorite_locations"`
		NameCompact       NameCompact          `yaml:"name_compact"`
		Columns           *ColumnWidths        `yaml:"columns"`
		Sort              SortConfig           `yaml:"sort"`
		Terminal          TerminalConfig       `yaml:"terminal"`
		Tabs              TabsConfig           `yaml:"tabs"`
		Interface         InterfaceConfig      `yaml:"interface"`
		Font              legacyFontConfig     `yaml:"font"`
		General           GeneralConfig        `yaml:"general"`
		Colors            ColorsConfig         `yaml:"colors"`
		Associations      []AssociationProgram `yaml:"associations,omitempty"`
		CustomCommands    []CustomCommand      `yaml:"custom_commands,omitempty"`
		Viewer            ViewerConfig         `yaml:"viewer"`
		SSH               SSHConfig            `yaml:"ssh"`
	}{
		General: GeneralConfig{
			OpenFavoritesInNewTab: true,
		},
		Viewer: ViewerConfig{
			SmoothScrolling: true,
		},
	}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	if strings.TrimSpace(raw.General.Typeface) == "" {
		raw.General.Typeface = raw.Font.Typeface
	}
	if raw.General.FontSizeSp <= 0 {
		raw.General.FontSizeSp = raw.Font.SizeSp
	}
	columns := defaultColumnWidths()
	if raw.Columns != nil {
		columns = *raw.Columns
	}
	*c = Config{
		DateFormats:       raw.DateFormats,
		FavoriteLocations: raw.FavoriteLocations,
		NameCompact:       raw.NameCompact,
		Columns:           columns,
		Sort:              raw.Sort,
		Terminal:          raw.Terminal,
		Tabs:              raw.Tabs,
		Interface:         raw.Interface,
		General:           raw.General,
		Colors:            raw.Colors,
		Associations:      raw.Associations,
		CustomCommands:    raw.CustomCommands,
		Viewer:            raw.Viewer,
		SSH:               raw.SSH,
	}
	return nil
}

func DefaultConfig() *Config {
	cfg := &Config{
		DateFormats: []string{
			"Jan 02 2006 15:04",
			"Jan 02 15:04",
			"Jan 02 2006",
			"Jan 02",
			"01-02",
		},
		FavoriteLocations: []string{},
		NameCompact: NameCompact{
			KeepStartChars: defaultNameKeepStartChars,
			Marker:         defaultNameCompactMarker,
		},
		Columns: defaultColumnWidths(),
		Sort: SortConfig{
			DefaultKey:       "name",
			Descending:       false,
			DirectoriesFirst: true,
		},
		Terminal: TerminalConfig{
			HeightRows: defaultTerminalHeightRows,
			Typeface:   resources.BundledFontFamilyFiraCodeNerdFontMono,
			FontSizeSp: 13,
		},
		Tabs: TabsConfig{
			WidthMode:    "variable",
			MinWidthDp:   defaultTabMinWidthDp,
			FixedWidthDp: defaultTabFixedWidthDp,
			MaxWidthDp:   defaultTabMaxWidthDp,
			Typeface:     resources.BundledFontFamilyFiraCodeNerdFontMono,
			FontSizeSp:   10,
		},
		Interface: InterfaceConfig{
			Typeface:   resources.BundledFontFamilyFiraCodeNerdFontMono,
			FontSizeSp: 14,
		},
		General: GeneralConfig{
			Typeface:              resources.BundledFontFamilyFiraCodeNerdFontMono,
			FontSizeSp:            14,
			DimInactivePanes:      true,
			OpenFavoritesInNewTab: true,
		},
		Colors: ColorsConfig{
			FilePaneBackground:  DefaultFilePaneBackgroundHex,
			FilePaneText:        DefaultFilePaneTextHex,
			Hover:               DefaultFilePaneHoverHex,
			HoverText:           DefaultFilePaneHoverTextHex,
			PopupHover:          DefaultPopupHoverHex,
			PopupHoverText:      DefaultPopupHoverTextHex,
			Selection:           DefaultFilePaneSelectionHex,
			SelectionText:       DefaultFilePaneSelectionTextHex,
			SelectedFiles:       DefaultFilePaneSelectedFilesHex,
			SelectedFilesText:   DefaultFilePaneSelectedTextHex,
			FocusedSelected:     DefaultFilePaneFocusedSelectedHex,
			FocusedSelectedText: DefaultFilePaneFocusedSelectedTextHex,
			CurrentDirBg:        DefaultCurrentDirBackgroundHex,
			CurrentDirText:      DefaultCurrentDirTextHex,
		},
		Associations:   nil,
		CustomCommands: nil,
		Viewer: ViewerConfig{
			FileEncoding:            ViewerFileEncodingAuto,
			Typeface:                resources.BundledFontFamilyFiraCodeNerdFontMono,
			Background:              DefaultFilePaneBackgroundHex,
			Text:                    DefaultFilePaneTextHex,
			Selection:               DefaultFilePaneSelectionHex,
			SmoothScrolling:         true,
			Shell:                   "auto",
			Command:                 "cat {path}",
			RemoteSearchMode:        ViewerRemoteSearchModeRemote,
			RemoteSearchCommand:     DefaultViewerRemoteSearchCommand,
			Associations:            nil,
			CommandRules:            nil,
			CommandByTarget:         map[string]string{},
			CommandHistory:          []string{},
			WordSelectRegex:         "[a-zA-Z0-9]+",
			FontSizeSp:              13,
			WordWrap:                false,
			MaxReadMB:               1,
			CommandAutoRefresh:      true,
			CommandRefreshMs:        1500,
			HideFunctionBarWhenOpen: true,
		},
		SSH: SSHConfig{
			Setups: []SSHSetup{},
		},
	}
	cfg.normalize()
	return cfg
}

func SaveConfig(path string, cfg *Config) error {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := cfg.LoadIssue(); err != nil {
		return fmt.Errorf("refusing to overwrite config because the existing file did not load cleanly: %w", err)
	}
	cfg.normalize()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return writeConfigFileAtomic(path, data, 0o644)
}

func LoadConfig(path string) *Config {
	cfg, _ := loadConfigFile(path)
	return cfg
}

func LoadConfigEnsuringFile(path string) (*Config, error) {
	cfg, err := loadConfigFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, SaveConfig(path, cfg)
		}
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) LoadIssue() error {
	if c == nil {
		return nil
	}
	return c.loadIssue
}

func (c *Config) setLoadIssue(err error) {
	if c == nil {
		return
	}
	c.loadIssue = err
}

func loadConfigFile(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, err
		}
		loadErr := fmt.Errorf("read config %s: %w", path, err)
		cfg.setLoadIssue(loadErr)
		return cfg, loadErr
	}
	loaded, err := decodeConfigData(data)
	if err == nil {
		return loaded, nil
	}

	loadErr := fmt.Errorf("parse config %s: %w", path, err)
	if recovered, ok := loadConfigBackup(path, loadErr); ok {
		return recovered, recovered.LoadIssue()
	}
	cfg.setLoadIssue(loadErr)
	return cfg, loadErr
}

func decodeConfigData(data []byte) (*Config, error) {
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.normalize()
	return cfg, nil
}

func loadConfigBackup(path string, primaryErr error) (*Config, bool) {
	backupPath := configBackupPath(path)
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return nil, false
	}
	cfg, err := decodeConfigData(data)
	if err != nil {
		return nil, false
	}
	cfg.setLoadIssue(fmt.Errorf("%v; loaded recovery backup %s", primaryErr, backupPath))
	return cfg, true
}

func configBackupPath(path string) string {
	return path + configBackupSuffix
}

func writeConfigFileAtomic(path string, data []byte, defaultMode os.FileMode) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("config path is empty")
	}

	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	mode := defaultMode
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
		if err := copyFile(path, configBackupPath(path), mode); err != nil {
			return fmt.Errorf("backup config %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := renameReplace(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	tmpPath := dst
	cleanup := true
	defer func() {
		if cleanup {
			_ = out.Close()
			_ = os.Remove(tmpPath)
		}
	}()
	if err := out.Chmod(mode); err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func renameReplace(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if info, statErr := os.Stat(dst); statErr == nil && !info.IsDir() {
		if removeErr := os.Remove(dst); removeErr != nil {
			return removeErr
		}
		return os.Rename(src, dst)
	}
	return os.Rename(src, dst)
}

func (c *Config) normalize() {
	if c == nil {
		return
	}
	if len(c.DateFormats) == 0 {
		c.DateFormats = DefaultConfig().DateFormats
	}
	c.normalizeFavoriteLocations()
	if c.NameCompact.KeepStartChars < 1 {
		c.NameCompact.KeepStartChars = defaultNameKeepStartChars
	}
	if c.NameCompact.Marker == "" {
		c.NameCompact.Marker = defaultNameCompactMarker
	}

	if c.Columns.NameChars <= 0 {
		c.Columns.NameChars = defaultNameChars
	}
	if c.Columns.BriefChars <= 0 {
		c.Columns.BriefChars = defaultBriefChars
	}
	c.Columns.FullDropPriority = normalizeFullDropPriority(c.Columns.FullDropPriority)
	switch c.Columns.PermissionFormat {
	case "symbolic", "octal", "auto":
	default:
		c.Columns.PermissionFormat = "auto"
	}

	c.Sort.DefaultKey = NormalizeSortKey(c.Sort.DefaultKey)
	c.Sort.PerDir = NormalizeSortPerDir(c.Sort.PerDir, c.Sort)
	c.Terminal.HeightRows = NormalizeTerminalHeightRows(c.Terminal.HeightRows)
	c.Tabs.WidthMode = NormalizeTabWidthMode(c.Tabs.WidthMode)
	c.Tabs.MinWidthDp = NormalizeTabWidthDp(c.Tabs.MinWidthDp, defaultTabMinWidthDp)
	c.Tabs.FixedWidthDp = NormalizeTabWidthDp(c.Tabs.FixedWidthDp, defaultTabFixedWidthDp)
	c.Tabs.MaxWidthDp = NormalizeTabWidthDp(c.Tabs.MaxWidthDp, defaultTabMaxWidthDp)
	if c.Tabs.MaxWidthDp < c.Tabs.MinWidthDp {
		c.Tabs.MaxWidthDp = c.Tabs.MinWidthDp
	}
	if c.Tabs.FixedWidthDp < c.Tabs.MinWidthDp {
		c.Tabs.FixedWidthDp = c.Tabs.MinWidthDp
	}
	if c.Tabs.FixedWidthDp > c.Tabs.MaxWidthDp {
		c.Tabs.FixedWidthDp = c.Tabs.MaxWidthDp
	}
	c.Tabs.Color = NormalizeOptionalHexColor(c.Tabs.Color)
	c.Tabs.AltColor = NormalizeOptionalHexColor(c.Tabs.AltColor)
	c.Tabs.ActiveColor = NormalizeOptionalHexColor(c.Tabs.ActiveColor)

	if c.General.Typeface == "" || !resources.IsBundledFontFamily(c.General.Typeface) {
		c.General.Typeface = resources.BundledFontFamilyFiraCodeNerdFontMono
	}
	if c.General.FontSizeSp <= 0 {
		c.General.FontSizeSp = 14
	}
	if c.Interface.Typeface == "" || !resources.IsBundledFontFamily(c.Interface.Typeface) {
		c.Interface.Typeface = resources.BundledFontFamilyFiraCodeNerdFontMono
	}
	if c.Interface.FontSizeSp < 6 {
		c.Interface.FontSizeSp = 14
	}
	if c.Tabs.Typeface == "" {
		c.Tabs.Typeface = c.General.Typeface
	}
	if !resources.IsBundledFontFamily(c.Tabs.Typeface) {
		c.Tabs.Typeface = c.General.Typeface
	}
	if c.Tabs.FontSizeSp < 6 {
		c.Tabs.FontSizeSp = c.General.FontSizeSp * (10.0 / 14.0)
		if c.Tabs.FontSizeSp < 6 {
			c.Tabs.FontSizeSp = 10
		}
	}
	if c.Terminal.Typeface == "" {
		c.Terminal.Typeface = c.General.Typeface
	}
	if !resources.IsBundledMonospaceFontFamily(c.Terminal.Typeface) {
		c.Terminal.Typeface = resources.BundledFontFamilyFiraCodeNerdFontMono
	}
	if c.Terminal.FontSizeSp < 6 {
		c.Terminal.FontSizeSp = c.General.FontSizeSp * (13.0 / 14.0)
		if c.Terminal.FontSizeSp < 6 {
			c.Terminal.FontSizeSp = 13
		}
	}
	if c.Viewer.Typeface == "" {
		c.Viewer.Typeface = c.General.Typeface
	}
	if c.Viewer.Typeface != c.General.Typeface && !resources.IsBundledFontFamily(c.Viewer.Typeface) {
		c.Viewer.Typeface = c.General.Typeface
	}
	c.Viewer.Background = NormalizeHexColor(c.Viewer.Background, c.Colors.FilePaneBackground)
	c.Viewer.Text = NormalizeHexColor(c.Viewer.Text, c.Colors.FilePaneText)
	c.Viewer.Selection = NormalizeHexColor(c.Viewer.Selection, c.Colors.Selection)
	c.Colors.FilePaneBackground = NormalizeHexColor(c.Colors.FilePaneBackground, DefaultFilePaneBackgroundHex)
	c.Colors.FilePaneText = NormalizeHexColor(c.Colors.FilePaneText, DefaultFilePaneTextHex)
	c.Colors.Hover = NormalizeHexColor(c.Colors.Hover, DefaultFilePaneHoverHex)
	c.Colors.HoverText = NormalizeHexColor(c.Colors.HoverText, DefaultFilePaneHoverTextHex)
	c.Colors.PopupHover = NormalizeHexColor(c.Colors.PopupHover, DefaultPopupHoverHex)
	c.Colors.PopupHoverText = NormalizeHexColor(c.Colors.PopupHoverText, DefaultPopupHoverTextHex)
	c.Colors.Selection = NormalizeHexColor(c.Colors.Selection, DefaultFilePaneSelectionHex)
	c.Colors.SelectionText = NormalizeHexColor(c.Colors.SelectionText, DefaultFilePaneSelectionTextHex)
	c.Colors.SelectedFiles = NormalizeHexColor(c.Colors.SelectedFiles, DefaultFilePaneSelectedFilesHex)
	c.Colors.SelectedFilesText = NormalizeHexColor(c.Colors.SelectedFilesText, DefaultFilePaneSelectedTextHex)
	c.Colors.FocusedSelected = NormalizeHexColor(c.Colors.FocusedSelected, DefaultFilePaneFocusedSelectedHex)
	c.Colors.FocusedSelectedText = NormalizeHexColor(c.Colors.FocusedSelectedText, DefaultFilePaneFocusedSelectedTextHex)
	c.Colors.CurrentDirBg = NormalizeHexColor(c.Colors.CurrentDirBg, DefaultCurrentDirBackgroundHex)
	c.Colors.CurrentDirText = NormalizeHexColor(c.Colors.CurrentDirText, DefaultCurrentDirTextHex)
	c.Colors.ScrollbarThumb = NormalizeOptionalHexColor(c.Colors.ScrollbarThumb)
	c.Colors.ScrollbarTrack = NormalizeOptionalHexColor(c.Colors.ScrollbarTrack)
	c.Colors.Filenames.Text = NormalizeOptionalHexColor(c.Colors.Filenames.Text)
	c.Colors.Filenames.Icon = NormalizeFilenameIcon(c.Colors.Filenames.Icon)
	c.Colors.Filenames.AgeRules = NormalizeFilenameAgeRules(c.Colors.Filenames.AgeRules)
	c.Colors.Filenames.PermissionRules = NormalizeFilenamePermissionRules(c.Colors.Filenames.PermissionRules)
	c.Colors.Filenames.ExtensionRules = NormalizeFilenameExtensionRules(c.Colors.Filenames.ExtensionRules)
	c.Colors.Filenames.SizeRules = NormalizeFilenameSizeRules(c.Colors.Filenames.SizeRules)
	c.CustomCommands = NormalizeCustomCommands(c.CustomCommands)

	c.Viewer.FileEncoding = NormalizeViewerFileEncoding(c.Viewer.FileEncoding)
	c.Viewer.Shell = NormalizeViewerShell(c.Viewer.Shell)
	if c.Viewer.Command == "" {
		c.Viewer.Command = "cat {path}"
	}
	legacyAssociations := NormalizeViewerAssociations(c.Viewer.Associations)
	c.Associations = NormalizeAssociationPrograms(c.Associations)
	if len(legacyAssociations) > 0 {
		combined := append(FlattenAssociationPrograms(c.Associations), legacyAssociations...)
		c.Associations = GroupViewerAssociations(combined)
	} else if len(c.Associations) == 0 {
		c.Associations = GroupViewerAssociations(legacyAssociations)
	}
	c.Viewer.Associations = nil
	c.Viewer.AssociatedExtensions = nil
	c.Viewer.CommandRules = NormalizeViewerCommandRules(c.Viewer.CommandRules)
	if len(c.Viewer.CommandByTarget) > 0 {
		normalized := make(map[string]string, len(c.Viewer.CommandByTarget))
		for rawKey, rawCmd := range c.Viewer.CommandByTarget {
			key := strings.TrimSpace(rawKey)
			cmd := strings.TrimSpace(rawCmd)
			if key == "" || cmd == "" {
				continue
			}
			normalized[key] = cmd
		}
		if len(normalized) == 0 {
			c.Viewer.CommandByTarget = nil
		} else {
			c.Viewer.CommandByTarget = normalized
		}
	} else {
		c.Viewer.CommandByTarget = nil
	}
	if len(c.Viewer.CommandHistory) > 0 {
		history := make([]string, 0, len(c.Viewer.CommandHistory))
		for _, raw := range c.Viewer.CommandHistory {
			cmd := strings.TrimSpace(raw)
			if cmd == "" {
				continue
			}
			duplicate := false
			for _, existing := range history {
				if existing == cmd {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}
			history = append(history, cmd)
			if len(history) >= 100 {
				break
			}
		}
		if len(history) == 0 {
			c.Viewer.CommandHistory = nil
		} else {
			c.Viewer.CommandHistory = history
		}
	} else {
		c.Viewer.CommandHistory = nil
	}
	if strings.TrimSpace(c.Viewer.WordSelectRegex) == "" {
		c.Viewer.WordSelectRegex = "[a-zA-Z0-9]+"
	}
	if c.Viewer.FontSizeSp < 6 {
		c.Viewer.FontSizeSp = c.General.FontSizeSp * (13.0 / 14.0)
		if c.Viewer.FontSizeSp < 6 {
			c.Viewer.FontSizeSp = 13
		}
	}
	if c.Viewer.MaxReadMB <= 0 {
		c.Viewer.MaxReadMB = 1
	}
	if c.Viewer.CommandRefreshMs < 200 {
		c.Viewer.CommandRefreshMs = 1500
	}
	c.Viewer.RemoteSearchMode = NormalizeViewerRemoteSearchMode(c.Viewer.RemoteSearchMode)
	c.Viewer.RemoteSearchCommand = NormalizeViewerRemoteSearchCommand(c.Viewer.RemoteSearchCommand)

	c.normalizeSSHSetups()
}

const (
	ViewerRemoteSearchModeRemote = "remote"
	ViewerRemoteSearchModeLocal  = "local"

	DefaultViewerRemoteSearchCommand = "tail -c +{range_start_1based} {path} | head -c {range_len} | LC_ALL=C grep -aobF {match_limit} -- {pattern} | {result_select}"
	legacyViewerRemoteSearchCommand  = "tail -c +{range_start1} {path} | head -c {range_len} | LC_ALL=C grep -aobF {match_limit} -- {pattern} | {result_select}"
	viewerRemoteSearchDisabledValue  = "off"
)

func NormalizeViewerRemoteSearchMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "default", "auto", ViewerRemoteSearchModeRemote, "command", "utility", "remote-command":
		return ViewerRemoteSearchModeRemote
	case ViewerRemoteSearchModeLocal, "builtin", "internal", "sftp":
		return ViewerRemoteSearchModeLocal
	default:
		return ViewerRemoteSearchModeRemote
	}
}

func NormalizeViewerRemoteSearchCommand(raw string) string {
	cmd := strings.TrimSpace(raw)
	switch strings.ToLower(cmd) {
	case "", "default":
		return DefaultViewerRemoteSearchCommand
	case "off", "none", "disabled":
		return viewerRemoteSearchDisabledValue
	default:
		if cmd == legacyViewerRemoteSearchCommand {
			return DefaultViewerRemoteSearchCommand
		}
		return cmd
	}
}

func EffectiveViewerRemoteSearchCommand(raw string) string {
	cmd := NormalizeViewerRemoteSearchCommand(raw)
	if strings.EqualFold(cmd, viewerRemoteSearchDisabledValue) {
		return ""
	}
	return cmd
}

func (c *Config) normalizeFavoriteLocations() {
	if c == nil {
		return
	}
	if len(c.FavoriteLocations) == 0 {
		c.FavoriteLocations = nil
		return
	}
	out := make([]string, 0, len(c.FavoriteLocations))
	for _, raw := range c.FavoriteLocations {
		loc := strings.TrimSpace(raw)
		if loc == "" {
			continue
		}
		duplicate := false
		for _, existing := range out {
			if sameFavoriteLocation(existing, loc) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		out = append(out, loc)
	}
	c.FavoriteLocations = out
}

func sameFavoriteLocation(a, b string) bool {
	if filepath.IsAbs(a) && filepath.IsAbs(b) {
		a = filepath.Clean(a)
		b = filepath.Clean(b)
		if os.PathSeparator == '\\' {
			return strings.EqualFold(a, b)
		}
		return a == b
	}
	return a == b
}

func (c *Config) normalizeSSHSetups() {
	if c == nil {
		return
	}
	if len(c.SSH.Setups) == 0 {
		c.SSH.Setups = nil
		return
	}
	out := make([]SSHSetup, 0, len(c.SSH.Setups))
	for _, raw := range c.SSH.Setups {
		setup := SSHSetup{
			Name:          strings.TrimSpace(raw.Name),
			Host:          strings.TrimSpace(raw.Host),
			Port:          raw.Port,
			User:          strings.TrimSpace(raw.User),
			Password:      raw.Password,
			KeyPath:       strings.TrimSpace(raw.KeyPath),
			KeyPassphrase: raw.KeyPassphrase,
		}
		if setup.Port <= 0 {
			setup.Port = 22
		}
		out = append(out, setup)
	}
	c.SSH.Setups = out
}

func normalizeFullDropPriority(raw []string) []string {
	defaultOrder := []string{"date", "size", "permissions", "name"}
	if len(raw) == 0 {
		return append([]string(nil), defaultOrder...)
	}

	normalize := func(v string) string {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "date", "time", "datetime":
			return "date"
		case "size":
			return "size"
		case "permissions", "permission", "perms", "perm":
			return "permissions"
		case "name", "filename", "file":
			return "name"
		default:
			return ""
		}
	}

	seen := make(map[string]struct{}, len(defaultOrder))
	out := make([]string, 0, len(defaultOrder))
	for _, item := range raw {
		key := normalize(item)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	for _, key := range defaultOrder {
		if _, exists := seen[key]; exists {
			continue
		}
		out = append(out, key)
	}
	return out
}

func NormalizeViewerFileEncoding(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", ViewerFileEncodingAuto, "detect", "auto-detect", "autodetect":
		return ViewerFileEncodingAuto
	case ViewerFileEncodingUTF8, "utf8":
		return ViewerFileEncodingUTF8
	case ViewerFileEncodingUTF16LE, "utf-16-le", "utf16le", "utf16-le":
		return ViewerFileEncodingUTF16LE
	case ViewerFileEncodingUTF16BE, "utf-16-be", "utf16be", "utf16-be":
		return ViewerFileEncodingUTF16BE
	case ViewerFileEncodingCP437, "437", "ibm437", "codepage437", "oem437":
		return ViewerFileEncodingCP437
	default:
		return ViewerFileEncodingAuto
	}
}

func NormalizeCustomCommand(raw CustomCommand) (CustomCommand, bool) {
	command := strings.TrimSpace(raw.Command)
	if command == "" {
		return CustomCommand{}, false
	}
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		name = customCommandFallbackName(command)
	}
	if name == "" {
		return CustomCommand{}, false
	}
	slot := raw.Slot
	if slot < 1 || slot > 10 {
		slot = 0
	}
	return CustomCommand{
		Slot:    slot,
		Name:    name,
		Command: command,
	}, true
}

func NormalizeCustomCommands(raw []CustomCommand) []CustomCommand {
	if len(raw) == 0 {
		return nil
	}
	slots := make([]CustomCommand, 10)
	used := make([]bool, 10)
	nextSlot := 1
	for _, rawCmd := range raw {
		cmd, ok := NormalizeCustomCommand(rawCmd)
		if !ok {
			continue
		}
		slot := cmd.Slot
		if slot < 1 || slot > 10 {
			for nextSlot <= 10 && used[nextSlot-1] {
				nextSlot++
			}
			if nextSlot > 10 {
				continue
			}
			slot = nextSlot
			nextSlot++
		}
		cmd.Slot = slot
		slots[slot-1] = cmd
		used[slot-1] = true
	}
	out := make([]CustomCommand, 0, 10)
	for i, cmd := range slots {
		if !used[i] {
			continue
		}
		cmd.Slot = i + 1
		out = append(out, cmd)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func customCommandFallbackName(command string) string {
	for _, line := range strings.Split(command, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) > 48 {
			runes := []rune(line)
			line = strings.TrimSpace(string(runes[:48]))
		}
		return line
	}
	return ""
}

// NormalizeViewerAssociationExtension accepts forms like ".pdf", "pdf", or
// "*.pdf" and converts them to a lowercase suffix match key such as ".pdf".
// Invalid values are dropped by returning an empty string.
func NormalizeViewerAssociationExtension(raw string) string {
	ext := strings.ToLower(strings.TrimSpace(raw))
	if ext == "" {
		return ""
	}
	if strings.HasPrefix(ext, "*.") {
		ext = ext[2:]
	} else if strings.HasPrefix(ext, "*") {
		ext = strings.TrimPrefix(ext, "*")
	}
	ext = strings.TrimLeft(ext, ".")
	if ext == "" {
		return ""
	}
	if strings.ContainsAny(ext, `/\:`) {
		return ""
	}
	return "." + ext
}

func NormalizeViewerAssociationAppPath(raw string) string {
	app := strings.TrimSpace(raw)
	if len(app) >= 2 {
		if (app[0] == '"' && app[len(app)-1] == '"') || (app[0] == '\'' && app[len(app)-1] == '\'') {
			app = strings.TrimSpace(app[1 : len(app)-1])
		}
	}
	return app
}

func NormalizeViewerAssociation(raw ViewerAssociation) (ViewerAssociation, bool) {
	ext := NormalizeViewerAssociationExtension(raw.Extension)
	app := NormalizeViewerAssociationAppPath(raw.AppPath)
	if ext == "" || app == "" {
		return ViewerAssociation{}, false
	}
	return ViewerAssociation{
		Extension: ext,
		AppPath:   app,
	}, true
}

func NormalizeViewerAssociations(raw []ViewerAssociation) []ViewerAssociation {
	if len(raw) == 0 {
		return nil
	}
	byExt := make(map[string]ViewerAssociation, len(raw))
	for _, item := range raw {
		assoc, ok := NormalizeViewerAssociation(item)
		if !ok {
			continue
		}
		byExt[assoc.Extension] = assoc
	}
	if len(byExt) == 0 {
		return nil
	}
	keys := make([]string, 0, len(byExt))
	for ext := range byExt {
		keys = append(keys, ext)
	}
	sort.Strings(keys)
	out := make([]ViewerAssociation, 0, len(keys))
	for _, ext := range keys {
		out = append(out, byExt[ext])
	}
	return out
}

func NormalizeViewerCommandRule(raw ViewerCommandRule) (ViewerCommandRule, bool) {
	pattern := strings.TrimSpace(raw.Pattern)
	command := strings.TrimSpace(raw.Command)
	if pattern == "" || command == "" {
		return ViewerCommandRule{}, false
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return ViewerCommandRule{}, false
	}
	return ViewerCommandRule{
		Pattern: pattern,
		Command: command,
	}, true
}

func NormalizeViewerCommandRules(raw []ViewerCommandRule) []ViewerCommandRule {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	reversed := make([]ViewerCommandRule, 0, len(raw))
	for i := len(raw) - 1; i >= 0; i-- {
		rule, ok := NormalizeViewerCommandRule(raw[i])
		if !ok {
			continue
		}
		if _, exists := seen[rule.Pattern]; exists {
			continue
		}
		seen[rule.Pattern] = struct{}{}
		reversed = append(reversed, rule)
	}
	if len(reversed) == 0 {
		return nil
	}
	out := make([]ViewerCommandRule, 0, len(reversed))
	for i := len(reversed) - 1; i >= 0; i-- {
		out = append(out, reversed[i])
	}
	return out
}

func MatchViewerCommandRules(raw []ViewerCommandRule, filename string) (string, bool) {
	filename = strings.TrimSpace(filename)
	if filename == "" || len(raw) == 0 {
		return "", false
	}
	rules := NormalizeViewerCommandRules(raw)
	if len(rules) == 0 {
		return "", false
	}
	command := ""
	matched := false
	for _, rule := range rules {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}
		if re.MatchString(filename) {
			command = rule.Command
			matched = true
		}
	}
	return command, matched
}

func parseAssociationProgramExtensions(raw any) []string {
	switch v := raw.(type) {
	case nil:
		return nil
	case string:
		return splitAssociationProgramExtensions(v)
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, splitAssociationProgramExtensions(item)...)
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, splitAssociationProgramExtensions(s)...)
			}
		}
		return out
	default:
		return nil
	}
}

func splitAssociationProgramExtensions(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func associationProgramExtensionsCSV(exts []string) string {
	if len(exts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(exts))
	for _, raw := range exts {
		ext := NormalizeViewerAssociationExtension(raw)
		if ext == "" {
			continue
		}
		parts = append(parts, strings.TrimPrefix(ext, "."))
	}
	return strings.Join(parts, ", ")
}

func NormalizeAssociationPrograms(raw []AssociationProgram) []AssociationProgram {
	if len(raw) == 0 {
		return nil
	}
	byApp := make(map[string]map[string]struct{}, len(raw))
	for _, item := range raw {
		app := NormalizeViewerAssociationAppPath(item.AppPath)
		if app == "" {
			continue
		}
		exts := byApp[app]
		if exts == nil {
			exts = make(map[string]struct{})
			byApp[app] = exts
		}
		for _, rawExt := range item.Extensions {
			ext := NormalizeViewerAssociationExtension(rawExt)
			if ext == "" {
				continue
			}
			exts[ext] = struct{}{}
		}
	}
	if len(byApp) == 0 {
		return nil
	}
	apps := make([]string, 0, len(byApp))
	for app := range byApp {
		apps = append(apps, app)
	}
	sort.Strings(apps)
	out := make([]AssociationProgram, 0, len(apps))
	for _, app := range apps {
		extMap := byApp[app]
		if len(extMap) == 0 {
			continue
		}
		exts := make([]string, 0, len(extMap))
		for ext := range extMap {
			exts = append(exts, ext)
		}
		sort.Strings(exts)
		out = append(out, AssociationProgram{
			AppPath:    app,
			Extensions: exts,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func FlattenAssociationPrograms(raw []AssociationProgram) []ViewerAssociation {
	if len(raw) == 0 {
		return nil
	}
	flat := make([]ViewerAssociation, 0)
	for _, group := range NormalizeAssociationPrograms(raw) {
		for _, ext := range group.Extensions {
			flat = append(flat, ViewerAssociation{
				Extension: ext,
				AppPath:   group.AppPath,
			})
		}
	}
	return NormalizeViewerAssociations(flat)
}

func GroupViewerAssociations(raw []ViewerAssociation) []AssociationProgram {
	flat := NormalizeViewerAssociations(raw)
	if len(flat) == 0 {
		return nil
	}
	byApp := make(map[string][]string, len(flat))
	for _, assoc := range flat {
		byApp[assoc.AppPath] = append(byApp[assoc.AppPath], assoc.Extension)
	}
	apps := make([]string, 0, len(byApp))
	for app := range byApp {
		apps = append(apps, app)
	}
	sort.Strings(apps)
	out := make([]AssociationProgram, 0, len(apps))
	for _, app := range apps {
		exts := byApp[app]
		sort.Strings(exts)
		out = append(out, AssociationProgram{
			AppPath:    app,
			Extensions: exts,
		})
	}
	return out
}

func defaultColumnWidths() ColumnWidths {
	return ColumnWidths{
		NameChars:        defaultNameChars,
		BriefChars:       defaultBriefChars,
		FullDropPriority: []string{"date", "size", "permissions", "name"},
		ShowPermissions:  true,
		PermissionFormat: "auto",
	}
}

func normalizeColumnChars(v, fallback float32) float32 {
	if v <= 0 {
		return fallback
	}
	if v < 1 {
		return 1
	}
	return v
}

func roundLegacyColumnChars(v float32) float32 {
	rounded := float32(math.Round(float64(v*2.0))) / 2.0
	if rounded < 1 {
		return 1
	}
	return rounded
}

func legacyWidthDpToChars(widthDp, padDp int, includeIcon bool) float32 {
	if widthDp < 1 {
		return 0
	}
	if padDp < 1 {
		padDp = defaultColumnPadDp
	}
	textWidth := widthDp - 2*padDp
	if includeIcon {
		textWidth -= defaultNameIconReserveDp
	}
	if textWidth < 1 {
		textWidth = 1
	}
	return roundLegacyColumnChars(float32(textWidth) / float32(defaultApproxCharPx))
}

func columnWidthDp(chars float32, includeIcon bool) int {
	textWidth := int(math.Round(float64(normalizeColumnChars(chars, 1) * float32(defaultApproxCharPx))))
	if textWidth < 1 {
		textWidth = 1
	}
	width := textWidth + columnPadReserveDp()
	if includeIcon {
		width += defaultNameIconReserveDp
	}
	return width
}

func ColumnPadDp() int {
	return defaultColumnPadDp
}

func BriefGapDp() int {
	return defaultBriefGapDp
}

func NameWidthDp(cfg *Config) int {
	if cfg == nil {
		return columnWidthDp(defaultNameChars, true)
	}
	return columnWidthDp(cfg.Columns.NameChars, true)
}

func NameMinWidthDp(cfg *Config) int {
	width := NameWidthDp(cfg)
	if width < defaultNameMinWidthDp {
		return width
	}
	return defaultNameMinWidthDp
}

func PermWidthDp(cfg *Config) int {
	return columnWidthDp(defaultPermWidthChars, false)
}

func SizeWidthDp(cfg *Config) int {
	return columnWidthDp(defaultSizeWidthChars, false)
}

func DateWidthDp(cfg *Config) int {
	return columnWidthDp(defaultDateWidthChars, false)
}

func BriefWidthDp(cfg *Config) int {
	if cfg == nil {
		return briefMinWidthDp(defaultBriefChars)
	}
	return briefMinWidthDp(cfg.Columns.BriefChars)
}

func SizeMinWidthDp(cfg *Config) int {
	padReserve := columnPadReserveDp()
	return defaultApproxCharPx*5 + 8 + padReserve
}

func PermMinWidthDp(cfg *Config) int {
	permChars := 4
	if cfg != nil && cfg.Columns.PermissionFormat == "symbolic" {
		permChars = 9
	}
	padReserve := columnPadReserveDp()
	return defaultApproxCharPx*permChars + 12 + padReserve
}

func DateMinWidthDp(cfg *Config) int {
	shortestDate := 5
	if cfg != nil && len(cfg.DateFormats) > 0 {
		shortestDate = utf8.RuneCountInString(cfg.DateFormats[0])
		for _, format := range cfg.DateFormats[1:] {
			n := utf8.RuneCountInString(format)
			if n < shortestDate {
				shortestDate = n
			}
		}
	}
	padReserve := columnPadReserveDp()
	return defaultApproxCharPx*shortestDate + 16 + padReserve
}

func columnPadReserveDp() int {
	return 2 * defaultColumnPadDp
}

func briefMinWidthDp(chars float32) int {
	textWidth := int(math.Round(float64(normalizeColumnChars(chars, 1) * float32(defaultApproxCharPx))))
	if textWidth < 1 {
		textWidth = 1
	}
	return textWidth + defaultNameTextReserveDp + columnPadReserveDp() + defaultNameIconReserveDp
}
