//go:build !darwin || ios

package main

import "hexone/fm"

func preparePlatformWindowRestore(_ *fm.SessionState) {}
