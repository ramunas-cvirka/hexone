package ui

import (
	"strconv"
	"strings"
)

func decodeHex(hexStr string) (string, int, error) {
	data, err := parseHexText(hexStr)
	if err != nil {
		return "", 0, err
	}
	var b strings.Builder
	b.Grow(len(data))
	for _, v := range data {
		if v >= 0x20 && v <= 0x7E {
			b.WriteByte(v)
		} else {
			b.WriteRune('�')
		}
	}
	return b.String(), len(data), nil
}

// Call this each frame; if text changed, handle it.
func (ui *UI) handleEditorChanges() {
	lt := ui.LeftEd.Text()
	if lt != ui.leftPrev {
		ui.leftPrev = lt
		ui.onLeftTextChanged(lt)
	}

	rt := ui.RightEd.Text()
	if rt != ui.rightPrev {
		ui.rightPrev = rt
		ui.onRightTextChanged(rt)
	}
}

func (ui *UI) onLeftTextChanged(text string) {
	rText, byteCount, err := decodeHex(text)
	if err != nil {
		return
	}

	// Avoid pointless resets + cursor jumps.
	if rText == ui.RightEd.Text() {
		return
	}

	ui.RightEd.SetText(rText)
	ui.LeftInfo = strconv.Itoa(byteCount) + " bytes"

	// Keep your change-detector in sync so it doesn't immediately fire
	// onRightTextChanged for this programmatic update (optional).
	ui.rightPrev = rText
}
func (ui *UI) onRightTextChanged(text string) {}
