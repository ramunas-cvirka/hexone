package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

const filePanePopupAnimDur = 140 * time.Millisecond

type filePanePopupTheme struct {
	Bg           color.NRGBA
	Border       color.NRGBA
	Text         color.NRGBA
	Muted        color.NRGBA
	Title        color.NRGBA
	HoverBg      color.NRGBA
	HoverText    color.NRGBA
	ActiveBg     color.NRGBA
	ActiveText   color.NRGBA
	DisabledText color.NRGBA
	Divider      color.NRGBA
	ButtonBg     color.NRGBA
	ButtonBorder color.NRGBA
}

func (ui *UI) filePanePopupTheme() filePanePopupTheme {
	palette := filePanePaletteFromConfig(nil)
	if ui != nil {
		palette = filePanePaletteFromConfig(ui.fmCfg)
	}

	bg := mixNRGBA(palette.PaneBg, palette.CurrentDirBg, 0.16)
	bg.A = 234
	border := filePaneActiveBorderColor(bg)
	border.A = 76
	text := bestContrastColor(bg, palette.PaneFg, palette.CurrentDirFg, palette.HoverFg, palette.SelectedFg)
	muted := mixNRGBA(text, bg, 0.44)
	muted.A = 220
	title := mixNRGBA(text, bg, 0.52)
	title.A = 228
	hoverBg := mixNRGBA(bg, palette.HoverBg, 0.82)
	hoverBg.A = 246
	hoverText := bestContrastColor(hoverBg, palette.HoverFg, palette.SelectedFg, text)
	activeBg := mixNRGBA(bg, palette.SelectedBg, 0.72)
	activeBg.A = 246
	activeText := bestContrastColor(activeBg, palette.SelectedFg, palette.HoverFg, text)
	disabledText := mixNRGBA(text, bg, 0.66)
	disabledText.A = 170
	divider := mixNRGBA(border, bg, 0.35)
	divider.A = 60
	buttonBg := mixNRGBA(bg, palette.CurrentDirBg, 0.22)
	buttonBg.A = 238
	buttonBorder := mixNRGBA(border, palette.CurrentDirFg, 0.26)
	buttonBorder.A = 78

	return filePanePopupTheme{
		Bg:           bg,
		Border:       border,
		Text:         text,
		Muted:        muted,
		Title:        title,
		HoverBg:      hoverBg,
		HoverText:    hoverText,
		ActiveBg:     activeBg,
		ActiveText:   activeText,
		DisabledText: disabledText,
		Divider:      divider,
		ButtonBg:     buttonBg,
		ButtonBorder: buttonBorder,
	}
}

func popupOpenProgress(now, openedAt time.Time) (float32, int, bool) {
	if openedAt.IsZero() {
		return 1, 0, false
	}
	elapsed := now.Sub(openedAt)
	if elapsed >= filePanePopupAnimDur {
		return 1, 0, false
	}
	t := smoothstep01(clamp01(float32(elapsed) / float32(filePanePopupAnimDur)))
	offsetY := int((1 - t) * 6)
	return t, offsetY, true
}

func scaleColorAlpha(c color.NRGBA, alpha float32) color.NRGBA {
	c.A = uint8(float32(c.A) * clamp01(alpha))
	return c
}

func fillRoundedClipBox(gtx layout.Context, radius int, bg, border color.NRGBA, w layout.Widget) layout.Dimensions {
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	call := m.Stop()
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		call.Add(gtx.Ops)
		return dims
	}

	rect := image.Rect(0, 0, dims.Size.X, dims.Size.Y)
	rr := clip.UniformRRect(rect, radius)
	paint.FillShape(gtx.Ops, bg, rr.Op(gtx.Ops))
	defer rr.Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
	paint.FillShape(gtx.Ops, border, clip.Stroke{Path: rr.Path(gtx.Ops), Width: 1}.Op())
	return dims
}
