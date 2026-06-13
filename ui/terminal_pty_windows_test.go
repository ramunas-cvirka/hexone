// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package ui

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWindowsTerminalProcessStartsCmd(t *testing.T) {
	name, args := terminalCommandForShellOnGOOS("windows", "cmd", "")
	proc, err := startTerminalProcess(name, args, "", terminalEnv(os.Environ(), 8, 40), 8, 40)
	if err != nil {
		if terminalProcessUnsupported(err) {
			t.Skipf("ConPTY unsupported on this Windows host: %v", err)
		}
		t.Fatalf("startTerminalProcess: %v", err)
	}
	defer func() {
		_ = proc.Close()
		_ = proc.Kill()
	}()

	chunks := make(chan string, 8)
	readErr := make(chan error, 1)
	go func() {
		buf := make([]byte, 512)
		for {
			n, err := proc.Read(buf)
			if n > 0 {
				chunks <- string(buf[:n])
			}
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
					err = nil
				}
				readErr <- err
				return
			}
		}
	}()
	waitErr := make(chan error, 1)
	go func() {
		waitErr <- proc.Wait()
	}()
	if _, err := proc.Write([]byte("echo hexone-conpty\r\nexit\r\n")); err != nil {
		t.Fatalf("write ConPTY input: %v", err)
	}

	var out strings.Builder
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case chunk := <-chunks:
			out.WriteString(chunk)
			if strings.Contains(out.String(), "hexone-conpty") {
				return
			}
		case err := <-readErr:
			if err != nil {
				t.Fatalf("read ConPTY output: %v", err)
			}
			if !strings.Contains(out.String(), "hexone-conpty") {
				t.Fatalf("ConPTY output=%q, want marker", out.String())
			}
			return
		case err := <-waitErr:
			if err != nil {
				t.Fatalf("ConPTY command exited with error: %v; output=%q", err, out.String())
			}
			if !strings.Contains(out.String(), "hexone-conpty") {
				t.Fatalf("ConPTY command exited without marker; output=%q", out.String())
			}
			return
		case <-timer.C:
			_ = proc.Close()
			_ = proc.Kill()
			t.Fatalf("timed out reading ConPTY output; partial=%q pid=%d", out.String(), proc.PID())
		}
	}
}
