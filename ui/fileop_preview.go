package ui

import (
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func fileOpPreviewLabel(name, path string) string {
	label := strings.TrimSpace(name)
	if label != "" {
		return label
	}
	return strings.TrimSpace(path)
}

func fileOpPreviewLines(labels []string) []string {
	filtered := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label != "" {
			filtered = append(filtered, label)
		}
	}
	switch len(filtered) {
	case 0:
		return nil
	case 1, 2, 3:
		return filtered
	default:
		return []string{filtered[0], filtered[1], "...", filtered[len(filtered)-1]}
	}
}

func (ui *UI) layoutFileOpPreviewList(th *material.Theme, gtx layout.Context, lines []string) layout.Dimensions {
	if len(lines) == 0 {
		return layout.Dimensions{}
	}
	children := make([]layout.FlexChild, 0, len(lines)*2-1)
	for i, line := range lines {
		line := line
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, line)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleDialogThemeFontSize(th, 9)
			lbl.Color = color.NRGBA{R: 172, G: 172, B: 172, A: 255}
			lbl.MaxLines = 1
			lbl.Truncator = "..."
			return lbl.Layout(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
