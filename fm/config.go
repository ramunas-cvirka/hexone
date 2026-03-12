package fm

import (
	"errors"
	resources "hexone"
	"math"
	"os"
	"path/filepath"
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
	defaultBriefGapDp        = 0
	defaultNameIconReserveDp = 14
	defaultNameChars         = 20.0
	defaultBriefChars        = 16.0
	defaultNameMinWidthDp    = 52
	defaultPermWidthChars    = 10.5
	defaultSizeWidthChars    = 10.5
	defaultDateWidthChars    = 15.0
	defaultNameTextReserveDp = defaultApproxCharPx/2 + 2
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
	DefaultKey       string `yaml:"default_key"`
	Descending       bool   `yaml:"descending"`
	DirectoriesFirst bool   `yaml:"directories_first"`
}

type GeneralConfig struct {
	Typeface         string  `yaml:"typeface"`
	FontSizeSp       float32 `yaml:"font_size_sp"`
	DimInactivePanes bool    `yaml:"dim_inactive_panes"`
}

type legacyFontConfig struct {
	Typeface string  `yaml:"typeface"`
	SizeSp   float32 `yaml:"size_sp"`
}

type ViewerAssociation struct {
	Extension string `yaml:"extension"`
	AppPath   string `yaml:"app_path"`
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
	Mode                    string              `yaml:"mode"`
	FileEncoding            string              `yaml:"file_encoding"`
	Typeface                string              `yaml:"typeface"`
	Background              string              `yaml:"background,omitempty"`
	Text                    string              `yaml:"text,omitempty"`
	Selection               string              `yaml:"selection,omitempty"`
	Shell                   string              `yaml:"shell"`
	Command                 string              `yaml:"command"`
	Associations            []ViewerAssociation `yaml:"associations,omitempty"`
	AssociatedExtensions    []string            `yaml:"associated_extensions,omitempty"`
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
	General           GeneralConfig        `yaml:"general"`
	Colors            ColorsConfig         `yaml:"colors"`
	Associations      []AssociationProgram `yaml:"associations,omitempty"`
	Viewer            ViewerConfig         `yaml:"viewer"`
	SSH               SSHConfig            `yaml:"ssh"`
}

func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	var raw struct {
		DateFormats       []string             `yaml:"date_formats"`
		FavoriteLocations []string             `yaml:"favorite_locations"`
		NameCompact       NameCompact          `yaml:"name_compact"`
		Columns           *ColumnWidths        `yaml:"columns"`
		Sort              SortConfig           `yaml:"sort"`
		Font              legacyFontConfig     `yaml:"font"`
		General           GeneralConfig        `yaml:"general"`
		Colors            ColorsConfig         `yaml:"colors"`
		Associations      []AssociationProgram `yaml:"associations,omitempty"`
		Viewer            ViewerConfig         `yaml:"viewer"`
		SSH               SSHConfig            `yaml:"ssh"`
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
		General:           raw.General,
		Colors:            raw.Colors,
		Associations:      raw.Associations,
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
		General: GeneralConfig{
			Typeface:         resources.BundledFontFamilyFiraCode,
			FontSizeSp:       14,
			DimInactivePanes: false,
		},
		Colors: ColorsConfig{
			FilePaneBackground:  DefaultFilePaneBackgroundHex,
			FilePaneText:        DefaultFilePaneTextHex,
			Hover:               DefaultFilePaneHoverHex,
			HoverText:           DefaultFilePaneHoverTextHex,
			Selection:           DefaultFilePaneSelectionHex,
			SelectionText:       DefaultFilePaneSelectionTextHex,
			SelectedFiles:       DefaultFilePaneSelectedFilesHex,
			SelectedFilesText:   DefaultFilePaneSelectedTextHex,
			FocusedSelected:     DefaultFilePaneFocusedSelectedHex,
			FocusedSelectedText: DefaultFilePaneFocusedSelectedTextHex,
			CurrentDirBg:        DefaultCurrentDirBackgroundHex,
			CurrentDirText:      DefaultCurrentDirTextHex,
		},
		Associations: nil,
		Viewer: ViewerConfig{
			Mode:                    "file",
			FileEncoding:            ViewerFileEncodingAuto,
			Typeface:                resources.BundledFontFamilyFiraCode,
			Background:              DefaultFilePaneBackgroundHex,
			Text:                    DefaultFilePaneTextHex,
			Selection:               DefaultFilePaneSelectionHex,
			Shell:                   "auto",
			Command:                 "cat {path}",
			Associations:            nil,
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
	cfg.normalize()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func LoadConfig(path string) *Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg
	}

	cfg.normalize()
	return cfg
}

func LoadConfigEnsuringFile(path string) (*Config, error) {
	cfg := LoadConfig(path)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, SaveConfig(path, cfg)
		}
	}
	return cfg, nil
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

	if c.Sort.DefaultKey == "" {
		c.Sort.DefaultKey = "name"
	}

	if c.General.Typeface == "" || c.General.Typeface == resources.BundledFontFamilyBlockZone || !resources.IsBundledFontFamily(c.General.Typeface) {
		c.General.Typeface = resources.BundledFontFamilyFiraCode
	}
	if c.General.FontSizeSp <= 0 {
		c.General.FontSizeSp = 14
	}
	if c.Viewer.Typeface == "" {
		c.Viewer.Typeface = c.General.Typeface
	}
	if c.Viewer.Typeface == resources.BundledFontFamilyBlockZone {
		c.Viewer.Typeface = resources.BundledFontFamilyFiraCode
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
	c.Colors.Selection = NormalizeHexColor(c.Colors.Selection, DefaultFilePaneSelectionHex)
	c.Colors.SelectionText = NormalizeHexColor(c.Colors.SelectionText, DefaultFilePaneSelectionTextHex)
	c.Colors.SelectedFiles = NormalizeHexColor(c.Colors.SelectedFiles, DefaultFilePaneSelectedFilesHex)
	c.Colors.SelectedFilesText = NormalizeHexColor(c.Colors.SelectedFilesText, DefaultFilePaneSelectedTextHex)
	c.Colors.FocusedSelected = NormalizeHexColor(c.Colors.FocusedSelected, DefaultFilePaneFocusedSelectedHex)
	c.Colors.FocusedSelectedText = NormalizeHexColor(c.Colors.FocusedSelectedText, DefaultFilePaneFocusedSelectedTextHex)
	c.Colors.CurrentDirBg = NormalizeHexColor(c.Colors.CurrentDirBg, DefaultCurrentDirBackgroundHex)
	c.Colors.CurrentDirText = NormalizeHexColor(c.Colors.CurrentDirText, DefaultCurrentDirTextHex)

	switch c.Viewer.Mode {
	case "file", "hex", "command":
	default:
		c.Viewer.Mode = "file"
	}
	c.Viewer.FileEncoding = NormalizeViewerFileEncoding(c.Viewer.FileEncoding)
	switch strings.ToLower(strings.TrimSpace(c.Viewer.Shell)) {
	case "", "auto":
		c.Viewer.Shell = "auto"
	case "sh":
		c.Viewer.Shell = "sh"
	case "pwsh", "powershell":
		c.Viewer.Shell = "powershell"
	default:
		c.Viewer.Shell = "auto"
	}
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

	c.normalizeSSHSetups()
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
