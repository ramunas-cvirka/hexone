// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build darwin

package notify

import "os/exec"

func playOperationComplete() {
	path, err := operationCompleteSoundPath()
	if err == nil && exec.Command("/usr/bin/afplay", path).Run() == nil {
		return
	}
	_ = exec.Command("/usr/bin/osascript", "-e", "beep 1").Run()
}
