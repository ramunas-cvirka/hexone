package fm

import (
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"
)

type SessionWindow struct {
	X           int     `yaml:"x"`
	Y           int     `yaml:"y"`
	HasPosition bool    `yaml:"has_position"`
	Width       int     `yaml:"width"`
	Height      int     `yaml:"height"`
	PxPerDp     float32 `yaml:"px_per_dp,omitempty"`
	Mode        string  `yaml:"mode"`
}

type SessionPane struct {
	Dir            string `yaml:"dir"`
	SelectedPath   string `yaml:"selected_path"`
	SortKey        string `yaml:"sort_key"`
	SortDescending bool   `yaml:"sort_desc"`
	Mode           string `yaml:"mode"`
}

type SessionState struct {
	Window     SessionWindow `yaml:"window"`
	ActiveTab  string        `yaml:"active_tab"`
	ActivePane int           `yaml:"active_pane"`
	Panes      []SessionPane `yaml:"panes"`
}

func DefaultSession() *SessionState {
	s := &SessionState{
		Window: SessionWindow{
			Mode: "windowed",
		},
		ActiveTab: "tab0",
	}
	s.normalize()
	return s
}

func LoadSession(path string) *SessionState {
	s := DefaultSession()
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := yaml.Unmarshal(data, s); err != nil {
		return s
	}
	s.normalize()
	return s
}

func SaveSession(path string, s *SessionState) error {
	if s == nil {
		s = DefaultSession()
	}
	s.normalize()
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *SessionState) normalize() {
	if s == nil {
		return
	}

	switch strings.ToLower(strings.TrimSpace(s.ActiveTab)) {
	case "tab0", "tab1", "tab2":
	default:
		s.ActiveTab = "tab0"
	}

	if s.ActivePane < 0 {
		s.ActivePane = 0
	}

	switch strings.ToLower(strings.TrimSpace(s.Window.Mode)) {
	case "windowed", "maximized", "fullscreen", "minimized":
		s.Window.Mode = strings.ToLower(strings.TrimSpace(s.Window.Mode))
	default:
		s.Window.Mode = "windowed"
	}

	if s.Window.Width < 120 {
		s.Window.Width = 0
	}
	if s.Window.Height < 120 {
		s.Window.Height = 0
	}
	if s.Window.PxPerDp < 0 {
		s.Window.PxPerDp = 0
	}

	out := make([]SessionPane, 0, len(s.Panes))
	for _, p := range s.Panes {
		dir := strings.TrimSpace(p.Dir)
		if dir != "" {
			dir = filepath.Clean(dir)
		}
		out = append(out, SessionPane{
			Dir:            dir,
			SelectedPath:   strings.TrimSpace(p.SelectedPath),
			SortKey:        normalizeSessionPaneSortKey(p.SortKey),
			SortDescending: p.SortDescending,
			Mode:           normalizeSessionPaneMode(p.Mode),
		})
	}
	s.Panes = out
}

func normalizeSessionPaneSortKey(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "ext", "extension", "type":
		return "ext"
	case "size":
		return "size"
	case "date", "time", "datetime":
		return "date"
	default:
		return "name"
	}
}

func normalizeSessionPaneMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "brief", "2c":
		return "brief"
	default:
		return "full"
	}
}
