// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package notify

import "os/exec"

func playOperationComplete() {
	path, err := operationCompleteSoundPath()
	if err != nil {
		return
	}
	for _, cmd := range []struct {
		name string
		args []string
	}{
		{name: "pw-play", args: []string{path}},
		{name: "paplay", args: []string{path}},
		{name: "aplay", args: []string{"-q", path}},
		{name: "canberra-gtk-play", args: []string{"-f", path}},
	} {
		if runIfAvailable(cmd.name, cmd.args...) {
			return
		}
	}
}

func runIfAvailable(name string, args ...string) bool {
	if _, err := exec.LookPath(name); err != nil {
		return false
	}
	return exec.Command(name, args...).Run() == nil
}
