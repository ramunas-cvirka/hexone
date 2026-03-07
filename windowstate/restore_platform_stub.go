//go:build !darwin || ios

package windowstate

import "hexone/fm"

func preparePlatformWindowRestore(_ *fm.SessionState) {}
