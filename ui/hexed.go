package ui

import (
	"encoding/hex"
	"strconv"
	"strings"
)

func decodeHex(hexStr string) string {
	hexStr = strings.TrimSpace(hexStr)
	if hexStr == "" {
		return ""
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		// if not valid hex, just return original input
		return hexStr
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

	return b.String()
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

	if len(text)%2 == 1 {
		return
	}

	// Avoid pointless resets + cursor jumps.
	if text == ui.RightEd.Text() {
		return
	}

	// SetText updates the editor content.
	rText := decodeHex(text)

	ui.RightEd.SetText(rText)
	ui.LeftInfo = strconv.Itoa(len(text)/2) + " bytes"

	// Keep your change-detector in sync so it doesn't immediately fire
	// onRightTextChanged for this programmatic update (optional).
	ui.rightPrev = rText
}
func (ui *UI) onRightTextChanged(text string) {}
