package ui

import (
	"fmt"
	"hexone/fm"
	"image"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type sshModalState struct {
	backdropClick widget.Clickable
	closeClick    widget.Clickable
	saveClick     widget.Clickable
	connectClick  widget.Clickable
	cancelClick   widget.Clickable
	addClick      widget.Clickable

	setups            []fm.SSHSetup
	selected          int
	setupClicks       []widget.Clickable
	setupRemoveClicks []widget.Clickable
	setupList         layout.List

	nameEdit    widget.Editor
	hostEdit    widget.Editor
	portEdit    widget.Editor
	userEdit    widget.Editor
	passEdit    widget.Editor
	keyPathEdit widget.Editor
	keyPassEdit widget.Editor

	footerAnim segmentedAnimState
	errText    string
}

func (st *sshModalState) currentEditorSetup() (fm.SSHSetup, bool) {
	if st == nil {
		return fm.SSHSetup{}, false
	}
	setup := fm.SSHSetup{
		Name:          strings.TrimSpace(st.nameEdit.Text()),
		Host:          strings.TrimSpace(st.hostEdit.Text()),
		User:          strings.TrimSpace(st.userEdit.Text()),
		Password:      st.passEdit.Text(),
		KeyPath:       strings.TrimSpace(st.keyPathEdit.Text()),
		KeyPassphrase: st.keyPassEdit.Text(),
	}
	portText := strings.TrimSpace(st.portEdit.Text())
	if p, err := strconv.Atoi(portText); err == nil && p > 0 && p <= 65535 {
		setup.Port = p
	} else {
		setup.Port = 22
	}
	nonEmpty := setup.Name != "" || setup.Host != "" || setup.User != "" ||
		setup.Password != "" || setup.KeyPath != "" || setup.KeyPassphrase != ""
	return setup, nonEmpty
}

func (ui *UI) openSSHModal() {
	if ui == nil {
		return
	}
	if ui.fmCfg == nil {
		ui.fmCfg = fm.DefaultConfig()
	}

	st := ui.sshModal
	if st == nil {
		st = &sshModalState{}
		st.setupList.Axis = layout.Vertical

		st.nameEdit.SingleLine = true
		st.hostEdit.SingleLine = true
		st.portEdit.SingleLine = true
		st.portEdit.Filter = "0123456789"
		st.userEdit.SingleLine = true
		st.passEdit.SingleLine = true
		st.passEdit.Mask = '*'
		st.keyPathEdit.SingleLine = true
		st.keyPassEdit.SingleLine = true
		st.keyPassEdit.Mask = '*'
	}

	st.loadFromConfig(ui.fmCfg)
	ui.sshModal = st
}

func (ui *UI) closeSSHModal() {
	ui.sshModal = nil
}

func (st *sshModalState) loadFromConfig(cfg *fm.Config) {
	if st == nil || cfg == nil {
		return
	}
	st.setups = cloneSSHSetups(cfg.SSH.Setups)
	if len(st.setups) > 0 {
		st.selected = 0
	} else {
		st.selected = -1
	}
	st.loadEditorsFromSelected()
	st.errText = ""
}

func cloneSSHSetups(src []fm.SSHSetup) []fm.SSHSetup {
	if len(src) == 0 {
		return nil
	}
	out := make([]fm.SSHSetup, len(src))
	copy(out, src)
	return out
}

func sshSetupIdentity(setup fm.SSHSetup) string {
	user := strings.TrimSpace(setup.User)
	host := strings.TrimSpace(setup.Host)
	port := setup.Port
	if port <= 0 {
		port = 22
	}
	switch {
	case user != "" && host != "":
		return user + "@" + host + ":" + strconv.Itoa(port)
	case host != "":
		return host + ":" + strconv.Itoa(port)
	case user != "":
		return user + "@?:" + strconv.Itoa(port)
	default:
		return "?:22"
	}
}

func (st *sshModalState) clearEditors() {
	st.nameEdit.SetText("")
	st.hostEdit.SetText("")
	st.portEdit.SetText("22")
	st.userEdit.SetText("")
	st.passEdit.SetText("")
	st.keyPathEdit.SetText("")
	st.keyPassEdit.SetText("")
}

func (st *sshModalState) loadEditorsFromSelected() {
	if st == nil {
		return
	}
	if st.selected < 0 || st.selected >= len(st.setups) {
		st.clearEditors()
		return
	}
	cur := st.setups[st.selected]
	st.nameEdit.SetText(cur.Name)
	st.hostEdit.SetText(cur.Host)
	port := cur.Port
	if port <= 0 {
		port = 22
	}
	st.portEdit.SetText(strconv.Itoa(port))
	st.userEdit.SetText(cur.User)
	st.passEdit.SetText(cur.Password)
	st.keyPathEdit.SetText(cur.KeyPath)
	st.keyPassEdit.SetText(cur.KeyPassphrase)
}

func (st *sshModalState) syncSelectedFromEditors() {
	if st == nil || st.selected < 0 || st.selected >= len(st.setups) {
		return
	}
	cur := &st.setups[st.selected]
	cur.Name = strings.TrimSpace(st.nameEdit.Text())
	cur.Host = strings.TrimSpace(st.hostEdit.Text())
	cur.User = strings.TrimSpace(st.userEdit.Text())
	cur.Password = st.passEdit.Text()
	cur.KeyPath = strings.TrimSpace(st.keyPathEdit.Text())
	cur.KeyPassphrase = st.keyPassEdit.Text()

	portText := strings.TrimSpace(st.portEdit.Text())
	if p, err := strconv.Atoi(portText); err == nil && p > 0 && p <= 65535 {
		cur.Port = p
	} else if cur.Port <= 0 {
		cur.Port = 22
	}
}

func (st *sshModalState) validatedSetups() ([]fm.SSHSetup, error) {
	if st == nil {
		return nil, nil
	}
	out := make([]fm.SSHSetup, 0, len(st.setups))
	for i, raw := range st.setups {
		setup := fm.SSHSetup{
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
		if setup.Port > 65535 {
			return nil, fmt.Errorf("setup %d port must be between 1 and 65535", i+1)
		}
		empty := setup.Name == "" && setup.Host == "" && setup.User == "" &&
			setup.Password == "" && setup.KeyPath == "" && setup.KeyPassphrase == ""
		if empty {
			continue
		}
		if setup.Host == "" {
			return nil, fmt.Errorf("setup %d host is required", i+1)
		}
		if setup.User == "" {
			return nil, fmt.Errorf("setup %d user is required", i+1)
		}
		// Keep name derived from connection identity.
		setup.Name = sshSetupIdentity(setup)
		out = append(out, setup)
	}
	return out, nil
}

func (st *sshModalState) ensureSetupClicks(n int) {
	if st == nil {
		return
	}
	if n <= 0 {
		st.setupClicks = nil
		return
	}
	if len(st.setupClicks) == n {
		return
	}
	next := make([]widget.Clickable, n)
	copy(next, st.setupClicks)
	st.setupClicks = next
}

func (st *sshModalState) ensureSetupRemoveClicks(n int) {
	if st == nil {
		return
	}
	if n <= 0 {
		st.setupRemoveClicks = nil
		return
	}
	if len(st.setupRemoveClicks) == n {
		return
	}
	next := make([]widget.Clickable, n)
	copy(next, st.setupRemoveClicks)
	st.setupRemoveClicks = next
}

func (ui *UI) saveSSHModal() error {
	st := ui.sshModal
	if st == nil {
		return nil
	}
	if ui.fmCfg == nil {
		ui.fmCfg = fm.DefaultConfig()
	}
	if st.selected >= 0 && st.selected < len(st.setups) {
		st.syncSelectedFromEditors()
	} else {
		setup, hasInput := st.currentEditorSetup()
		if hasInput {
			st.setups = append(st.setups, setup)
			st.selected = len(st.setups) - 1
		}
	}
	setups, err := st.validatedSetups()
	if err != nil {
		return err
	}
	ui.fmCfg.SSH.Setups = setups
	if err := fm.SaveConfig("fm.yaml", ui.fmCfg); err != nil {
		return err
	}
	st.loadFromConfig(ui.fmCfg)
	return nil
}

func (ui *UI) layoutSSHModal(th *material.Theme, gtx layout.Context) layout.Dimensions {
	st := ui.sshModal
	if st == nil {
		return layout.Dimensions{}
	}

	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if ok && ke.State == key.Press && ke.Name == key.NameEscape {
			ui.closeSSHModal()
			return layout.Dimensions{}
		}
	}

	for st.backdropClick.Clicked(gtx) {
	}
	if st.closeClick.Clicked(gtx) {
		ui.closeSSHModal()
		return layout.Dimensions{}
	}
	if st.cancelClick.Clicked(gtx) {
		st.footerAnim.setPulse("cancel", gtx.Now)
		ui.closeSSHModal()
		return layout.Dimensions{}
	}
	if st.saveClick.Clicked(gtx) {
		st.footerAnim.setPulse("save", gtx.Now)
		if err := ui.saveSSHModal(); err != nil {
			st.errText = err.Error()
		} else {
			ui.closeSSHModal()
			return layout.Dimensions{}
		}
	}
	if st.connectClick.Clicked(gtx) {
		st.footerAnim.setPulse("connect", gtx.Now)
		if err := ui.connectSSHModalToActivePane(gtx.Now); err != nil {
			st.errText = err.Error()
		} else {
			ui.closeSSHModal()
			return layout.Dimensions{}
		}
	}
	if st.addClick.Clicked(gtx) {
		if st.selected >= 0 && st.selected < len(st.setups) {
			st.syncSelectedFromEditors()
		}
		setup, hasInput := st.currentEditorSetup()
		if hasInput && st.selected < 0 {
			st.setups = append(st.setups, setup)
		} else {
			st.setups = append(st.setups, fm.SSHSetup{Port: 22})
		}
		st.selected = len(st.setups) - 1
		st.loadEditorsFromSelected()
		st.errText = ""
		gtx.Execute(op.InvalidateCmd{})
	}

	st.ensureSetupClicks(len(st.setups))
	st.ensureSetupRemoveClicks(len(st.setups))
	removed := map[int]struct{}{}
	for i := range st.setupRemoveClicks {
		if !st.setupRemoveClicks[i].Clicked(gtx) {
			continue
		}
		removed[i] = struct{}{}
		st.setups = append(st.setups[:i], st.setups[i+1:]...)
		if len(st.setups) == 0 {
			st.selected = -1
		} else {
			if st.selected == i {
				if i >= len(st.setups) {
					st.selected = len(st.setups) - 1
				}
			} else if st.selected > i {
				st.selected--
			}
		}
		st.loadEditorsFromSelected()
		st.errText = ""
		gtx.Execute(op.InvalidateCmd{})
		break
	}
	for i := range st.setupClicks {
		if _, ok := removed[i]; ok {
			continue
		}
		if st.setupClicks[i].Clicked(gtx) {
			if st.selected != i {
				st.syncSelectedFromEditors()
				st.selected = i
				st.loadEditorsFromSelected()
				st.errText = ""
			}
		}
	}

	return st.backdropClick.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
		paint.FillShape(gtx.Ops, color.NRGBA{A: 140}, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

		width := gtx.Dp(unit.Dp(760))
		height := gtx.Dp(unit.Dp(240))
		maxW := gtx.Constraints.Max.X - gtx.Dp(unit.Dp(20))
		maxH := gtx.Constraints.Max.Y - gtx.Dp(unit.Dp(20))
		if width > maxW {
			width = maxW
		}
		if height > maxH {
			height = maxH
		}
		if width < 560 {
			width = 560
		}
		if height < 210 {
			height = 210
		}

		m := op.Record(gtx.Ops)
		card := fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
				return fillRoundedBox(
					gtx,
					gtx.Dp(unit.Dp(filePaneOverlayCornerDp)),
					color.NRGBA{R: 20, G: 20, B: 20, A: 252},
					color.NRGBA{R: 255, G: 255, B: 255, A: 18},
					func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSSHModalHeader(th, gtx, st)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSSHModalBody(th, gtx, st)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSSHModalFooter(th, gtx, st)
								}),
							)
						})
					},
				)
			})
		})
		call := m.Stop()

		x := (gtx.Constraints.Max.X - card.Size.X) / 2
		y := (gtx.Constraints.Max.Y - card.Size.Y) / 2
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		return layout.Dimensions{Size: gtx.Constraints.Max}
	})
}

func (ui *UI) layoutSSHModalHeader(th *material.Theme, gtx layout.Context, st *sshModalState) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body1(th, "SSH Sessions")
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.Font.Weight = font.Bold
			lbl.TextSize = scaleThemeFontSize(th, 12)
			lbl.Color = txtColor
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutTinyIconModeButton(th, gtx, &st.closeClick, uiCloseIcon(), false)
		}),
	)
}

func (ui *UI) layoutSSHModalBody(th *material.Theme, gtx layout.Context, st *sshModalState) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, gtx.Dp(unit.Dp(190)), func(gtx layout.Context) layout.Dimensions {
				return ui.layoutSSHSetupsList(th, gtx, st)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return ui.layoutSSHSetupForm(th, gtx, st)
		}),
	)
}

func (ui *UI) layoutSSHSetupsList(th *material.Theme, gtx layout.Context, st *sshModalState) layout.Dimensions {
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 24, G: 24, B: 24, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(5)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Caption(th, "Saved setups")
								lbl.Font.Typeface = ui.mainTypeface()
								lbl.TextSize = scaleThemeFontSize(th, 9)
								lbl.Color = hintColor
								return lbl.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSSHAddButton(th, gtx, &st.addClick)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						if len(st.setups) == 0 {
							lbl := material.Body2(th, "No setups yet. Press + to add one.")
							lbl.Font.Typeface = ui.mainTypeface()
							lbl.TextSize = scaleThemeFontSize(th, 10)
							lbl.Color = hintColor
							lbl.MaxLines = 3
							return lbl.Layout(gtx)
						}
						return st.setupList.Layout(gtx, len(st.setups), func(gtx layout.Context, index int) layout.Dimensions {
							return ui.layoutSSHSetupRow(th, gtx, st, index)
						})
					}),
				)
			})
		},
	)
}

func (ui *UI) layoutSSHAddButton(th *material.Theme, gtx layout.Context, c *widget.Clickable) layout.Dimensions {
	return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bg := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
		border := color.NRGBA{R: 255, G: 255, B: 255, A: 18}
		fg := color.NRGBA{R: 232, G: 232, B: 232, A: 255}
		if c.Hovered() {
			bg = color.NRGBA{R: 34, G: 34, B: 34, A: 255}
			border = color.NRGBA{R: 255, G: 255, B: 255, A: 30}
		}
		if c.Pressed() {
			bg = color.NRGBA{R: 44, G: 44, B: 44, A: 255}
		}
		return fillRoundedBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneControlCornerDp)),
			bg,
			border,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(1), Bottom: unit.Dp(1)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body1(th, "+")
					lbl.Font.Typeface = ui.mainTypeface()
					lbl.Font.Weight = font.Bold
					lbl.TextSize = scaleThemeFontSize(th, 12)
					lbl.Color = fg
					return lbl.Layout(gtx)
				})
			},
		)
	})
}

func (ui *UI) layoutSSHSetupRow(th *material.Theme, gtx layout.Context, st *sshModalState, index int) layout.Dimensions {
	setup := st.setups[index]
	label := sshSetupIdentity(setup)

	active := index == st.selected
	bg := color.NRGBA{R: 24, G: 24, B: 24, A: 240}
	bd := color.NRGBA{R: 255, G: 255, B: 255, A: 14}
	if active {
		bg = color.NRGBA{R: 40, G: 40, B: 40, A: 255}
		bd = color.NRGBA{R: 255, G: 255, B: 255, A: 42}
	} else if st.setupClicks[index].Hovered() {
		bg = color.NRGBA{R: 32, G: 32, B: 32, A: 255}
	}

	return layout.Inset{Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return fillRoundedBox(
			gtx,
			gtx.Dp(unit.Dp(filePaneControlCornerDp)),
			bg,
			bd,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(5), Right: unit.Dp(4), Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return st.setupClicks[index].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, label)
								lbl.Font.Typeface = ui.mainTypeface()
								lbl.TextSize = scaleThemeFontSize(th, 10)
								lbl.Font.Weight = font.Medium
								lbl.Color = txtColor
								lbl.MaxLines = 1
								lbl.Truncator = "..."
								return lbl.Layout(gtx)
							})
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layoutTinyIconModeButton(th, gtx, &st.setupRemoveClicks[index], uiCloseIcon(), false)
						}),
					)
				})
			},
		)
	})
}

func (ui *UI) layoutSSHSetupForm(th *material.Theme, gtx layout.Context, st *sshModalState) layout.Dimensions {
	identity, _ := st.currentEditorSetup()
	identityLabel := sshSetupIdentity(identity)
	return fillRoundedBox(
		gtx,
		gtx.Dp(unit.Dp(filePaneControlCornerDp)),
		color.NRGBA{R: 24, G: 24, B: 24, A: 255},
		color.NRGBA{R: 255, G: 255, B: 255, A: 18},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(5)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "Setup details")
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.TextSize = scaleThemeFontSize(th, 9)
						lbl.Color = hintColor
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, identityLabel)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.Font.Weight = font.Medium
						lbl.TextSize = scaleThemeFontSize(th, 10)
						lbl.Color = color.NRGBA{R: 220, G: 220, B: 220, A: 255}
						lbl.MaxLines = 1
						lbl.Truncator = "..."
						return lbl.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(1.0, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSSHField(th, gtx, "IP / Host", &st.hostEdit, "example.com", true)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, gtx.Dp(unit.Dp(72)), func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSSHField(th, gtx, "Port", &st.portEdit, "22", true)
								})
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSSHField(th, gtx, "User", &st.userEdit, "root", true)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSSHField(th, gtx, "Password", &st.passEdit, "optional", true)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(3)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
							layout.Flexed(1.3, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSSHField(th, gtx, "Key path", &st.keyPathEdit, "C:\\Users\\me\\.ssh\\id_ed25519", true)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Flexed(0.7, func(gtx layout.Context) layout.Dimensions {
								return ui.layoutSSHField(th, gtx, "Passphrase", &st.keyPassEdit, "optional", true)
							}),
						)
					}),
				)
			})
		},
	)
}

func (ui *UI) layoutSSHField(th *material.Theme, gtx layout.Context, label string, edState *widget.Editor, hint string, enabled bool) layout.Dimensions {
	rowLabel := material.Caption(th, label)
	rowLabel.Font.Typeface = ui.mainTypeface()
	rowLabel.TextSize = scaleThemeFontSize(th, 9)
	rowLabel.Color = hintColor

	edState.ReadOnly = !enabled
	ed := material.Editor(th, edState, hint)
	ed.Font.Typeface = ui.mainTypeface()
	ed.TextSize = scaleThemeFontSize(th, 10)
	ed.Color = txtColor
	ed.HintColor = hintColor
	if !enabled {
		ed.Color = color.NRGBA{R: 132, G: 132, B: 132, A: 255}
		ed.HintColor = color.NRGBA{R: 98, G: 98, B: 98, A: 255}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(rowLabel.Layout),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutNeutralEditorBox(gtx, gtx.Focused(edState), enabled, ed.Layout)
		}),
	)
}

func (ui *UI) layoutSSHModalFooter(th *material.Theme, gtx layout.Context, st *sshModalState) layout.Dimensions {
	hoverFooterKey := ""
	if st.cancelClick.Hovered() {
		hoverFooterKey = "cancel"
	}
	if st.saveClick.Hovered() {
		hoverFooterKey = "save"
	}
	if st.connectClick.Hovered() {
		hoverFooterKey = "connect"
	}
	st.footerAnim.setHover(hoverFooterKey, gtx.Now)
	hoverCancel, hoverAnimCancel := st.footerAnim.hoverFill(gtx.Now, "cancel")
	hoverSave, hoverAnimSave := st.footerAnim.hoverFill(gtx.Now, "save")
	hoverConnect, hoverAnimConnect := st.footerAnim.hoverFill(gtx.Now, "connect")
	pulseCancel, pulseAnimCancel := st.footerAnim.pulseFill(gtx.Now, "cancel")
	pulseSave, pulseAnimSave := st.footerAnim.pulseFill(gtx.Now, "save")
	pulseConnect, pulseAnimConnect := st.footerAnim.pulseFill(gtx.Now, "connect")
	if hoverAnimCancel || hoverAnimSave || hoverAnimConnect || pulseAnimCancel || pulseAnimSave || pulseAnimConnect {
		gtx.Execute(op.InvalidateCmd{})
	}

	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if st.errText == "" {
				return layout.Dimensions{}
			}
			lbl := material.Caption(th, st.errText)
			lbl.Font.Typeface = ui.mainTypeface()
			lbl.TextSize = scaleThemeFontSize(th, 9)
			lbl.Color = color.NRGBA{R: 255, G: 170, B: 170, A: 255}
			lbl.MaxLines = 2
			lbl.Truncator = "..."
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			stripH := gtx.Dp(unit.Dp(22))
			if stripH < 1 {
				stripH = 1
			}
			return fillRoundedBox(
				gtx,
				gtx.Dp(unit.Dp(filePaneControlCornerDp)),
				color.NRGBA{R: 24, G: 24, B: 24, A: 255},
				color.NRGBA{R: 255, G: 255, B: 255, A: 22},
				func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSSHFooterSegment(th, gtx, &st.cancelClick, "Cancel", hoverCancel, pulseCancel, stripH, true, false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return toolbarSeparator(gtx, stripH)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSSHFooterSegment(th, gtx, &st.saveClick, "Save", hoverSave, pulseSave, stripH, false, false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return toolbarSeparator(gtx, stripH)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return ui.layoutSSHFooterSegment(th, gtx, &st.connectClick, "Connect", hoverConnect, pulseConnect, stripH, false, true)
								}),
							)
						})
					})
				},
			)
		}),
	)
}

func (ui *UI) layoutSSHFooterSegment(th *material.Theme, gtx layout.Context, c *widget.Clickable, label string, hoverFill, pulseFill float32, stripH int, roundLeft, roundRight bool) layout.Dimensions {
	if c == nil {
		return layout.Dimensions{}
	}
	dims := fixedHeight(gtx, stripH, func(gtx layout.Context) layout.Dimensions {
		return c.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			hoverFill = clamp01(hoverFill)
			pulseFill = clamp01(pulseFill)
			if c.Pressed() && pulseFill < 0.45 {
				pulseFill = 0.45
			}

			base := color.NRGBA{R: 24, G: 24, B: 24, A: 255}
			hover := color.NRGBA{R: 34, G: 34, B: 34, A: 255}
			pulse := color.NRGBA{R: 44, G: 44, B: 44, A: 255}

			bg := mixNRGBA(base, hover, hoverFill)
			bg = mixNRGBA(bg, pulse, pulseFill*0.55)

			fg := mixNRGBA(txtColor, color.NRGBA{R: 238, G: 238, B: 238, A: 255}, hoverFill*0.6)
			fg = mixNRGBA(fg, color.NRGBA{R: 248, G: 248, B: 248, A: 255}, pulseFill*0.35)

			radius := gtx.Dp(unit.Dp(filePaneControlCornerDp - 1))
			return fillSegmentBg(gtx, bg, radius, roundLeft, roundRight, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(9), Right: unit.Dp(9)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, label)
						lbl.Font.Typeface = ui.mainTypeface()
						lbl.Font.Weight = font.Medium
						lbl.TextSize = scaleThemeFontSize(th, 10)
						lbl.Color = fg
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					})
				})
			})
		})
	})
	if dims.Size.X <= 0 || dims.Size.Y <= 0 {
		return dims
	}
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}
