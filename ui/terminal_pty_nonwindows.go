// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package ui

import (
	"errors"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type unixTerminalProcess struct {
	f   *os.File
	cmd *exec.Cmd
}

func startTerminalProcess(name string, args []string, cwd string, env []string, rows, cols int) (terminalProcess, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	if cwd != "" {
		cmd.Dir = cwd
	}
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, err
	}
	return &unixTerminalProcess{f: f, cmd: cmd}, nil
}

func (p *unixTerminalProcess) Read(b []byte) (int, error) {
	if p == nil || p.f == nil {
		return 0, os.ErrClosed
	}
	return p.f.Read(b)
}

func (p *unixTerminalProcess) Write(b []byte) (int, error) {
	if p == nil || p.f == nil {
		return 0, os.ErrClosed
	}
	return p.f.Write(b)
}

func (p *unixTerminalProcess) Close() error {
	if p == nil || p.f == nil {
		return nil
	}
	return p.f.Close()
}

func (p *unixTerminalProcess) Resize(rows, cols int) error {
	if p == nil || p.f == nil {
		return nil
	}
	return pty.Setsize(p.f, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (p *unixTerminalProcess) Wait() error {
	if p == nil || p.cmd == nil {
		return nil
	}
	return p.cmd.Wait()
}

func (p *unixTerminalProcess) Kill() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

func (p *unixTerminalProcess) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func terminalProcessUnsupported(err error) bool {
	return errors.Is(err, pty.ErrUnsupported)
}
