// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !windows && !darwin && !(linux || freebsd || openbsd)

package appicon

import "gioui.org/app"

type Setter struct{}

func NewSetter() *Setter {
	return &Setter{}
}

func (s *Setter) HandleViewEvent(_ app.ViewEvent) {}
