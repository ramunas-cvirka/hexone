// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package ui

import (
	"errors"
	"os"

	winpty "github.com/aymanbagabas/go-pty"
)

type windowsTerminalProcess struct {
	pty winpty.Pty
	cmd *winpty.Cmd
}

func startTerminalProcess(name string, args []string, cwd string, env []string, rows, cols int) (terminalProcess, error) {
	p, err := winpty.New()
	if err != nil {
		return nil, err
	}
	if rows > 0 && cols > 0 {
		if err := p.Resize(cols, rows); err != nil {
			_ = p.Close()
			return nil, err
		}
	}
	cmd := p.Command(name, args...)
	cmd.Env = env
	if cwd != "" {
		cmd.Dir = cwd
	}
	if err := cmd.Start(); err != nil {
		_ = p.Close()
		return nil, err
	}
	return &windowsTerminalProcess{pty: p, cmd: cmd}, nil
}

func (p *windowsTerminalProcess) Read(b []byte) (int, error) {
	if p == nil || p.pty == nil {
		return 0, os.ErrClosed
	}
	return p.pty.Read(b)
}

func (p *windowsTerminalProcess) Write(b []byte) (int, error) {
	if p == nil || p.pty == nil {
		return 0, os.ErrClosed
	}
	return p.pty.Write(b)
}

func (p *windowsTerminalProcess) Close() error {
	if p == nil || p.pty == nil {
		return nil
	}
	return p.pty.Close()
}

func (p *windowsTerminalProcess) Resize(rows, cols int) error {
	if p == nil || p.pty == nil {
		return nil
	}
	return p.pty.Resize(cols, rows)
}

func (p *windowsTerminalProcess) Wait() error {
	if p == nil || p.cmd == nil {
		return nil
	}
	return p.cmd.Wait()
}

func (p *windowsTerminalProcess) Kill() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

func (p *windowsTerminalProcess) PID() int {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func terminalProcessUnsupported(err error) bool {
	return errors.Is(err, winpty.ErrUnsupported)
}
