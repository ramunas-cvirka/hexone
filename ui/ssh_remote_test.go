// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"fmt"
	"hexone/filesys"
	"hexone/fm"
	"io"
	"net"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func TestShouldReconnectSSHTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "eof", err: io.EOF, want: true},
		{name: "unexpected eof", err: io.ErrUnexpectedEOF, want: true},
		{name: "net closed", err: net.ErrClosed, want: true},
		{name: "wrapped connection lost", err: fmt.Errorf("wrapped: %w", sftp.ErrSSHFxConnectionLost), want: true},
		{name: "wrapped no connection", err: fmt.Errorf("wrapped: %w", sftp.ErrSSHFxNoConnection), want: true},
		{name: "packet header eof", err: errors.New("failed to send packet header: EOF"), want: true},
		{name: "permission denied", err: sftp.ErrSSHFxPermissionDenied, want: false},
		{name: "no such file", err: sftp.ErrSSHFxNoSuchFile, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldReconnectSSHTransport(tt.err)
			if got != tt.want {
				t.Fatalf("shouldReconnectSSHTransport(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestPaneSSHSessionReadDirReconnectsOnTransportError(t *testing.T) {
	oldReadDir := readDirSFTPFunc
	oldOpen := openSSHClientsFunc
	oldCloseSFTP := closeSFTPClientFunc
	oldCloseSSH := closeSSHClientFunc
	t.Cleanup(func() {
		readDirSFTPFunc = oldReadDir
		openSSHClientsFunc = oldOpen
		closeSFTPClientFunc = oldCloseSFTP
		closeSSHClientFunc = oldCloseSSH
	})

	closeSFTPClientFunc = func(*sftp.Client) {}
	closeSSHClientFunc = func(*ssh.Client) {}

	setup := fm.SSHSetup{Host: "example.test", Port: 22, User: "ramunas"}
	oldClient := new(sftp.Client)
	newClient := new(sftp.Client)
	readCalls := 0
	dialCalls := 0

	readDirSFTPFunc = func(client *sftp.Client, dir string) (filesys.Listing, error) {
		readCalls++
		switch client {
		case oldClient:
			return filesys.Listing{}, sftp.ErrSSHFxConnectionLost
		case newClient:
			return filesys.Listing{Dir: dir}, nil
		default:
			t.Fatalf("unexpected client %p", client)
			return filesys.Listing{}, nil
		}
	}
	openSSHClientsFunc = func(got fm.SSHSetup) (sshClientBundle, error) {
		dialCalls++
		if got.Host != setup.Host || got.Port != setup.Port || got.User != setup.User {
			t.Fatalf("unexpected reconnect setup: %+v", got)
		}
		return sshClientBundle{
			sshClient: new(ssh.Client),
			sftpBase:  new(ssh.Client),
			sftp:      newClient,
		}, nil
	}

	session := &paneSSHSession{
		setup: setup,
		conn: newSharedSSHConn(sshClientBundle{
			sshClient: new(ssh.Client),
			sftpBase:  new(ssh.Client),
			sftp:      oldClient,
		}),
	}

	listing, err := session.readDir("/var/log")
	if err != nil {
		t.Fatalf("readDir returned error: %v", err)
	}
	if listing.Dir != "/var/log" {
		t.Fatalf("listing dir = %q, want %q", listing.Dir, "/var/log")
	}
	if readCalls != 2 {
		t.Fatalf("readDir calls = %d, want 2", readCalls)
	}
	if dialCalls != 1 {
		t.Fatalf("reconnect dials = %d, want 1", dialCalls)
	}
	if got := session.sftpClient(); got != newClient {
		t.Fatalf("session client = %p, want %p", got, newClient)
	}
}

func TestPaneSSHSessionReadDirDoesNotReconnectOnRegularSFTPError(t *testing.T) {
	oldReadDir := readDirSFTPFunc
	oldOpen := openSSHClientsFunc
	oldCloseSFTP := closeSFTPClientFunc
	oldCloseSSH := closeSSHClientFunc
	t.Cleanup(func() {
		readDirSFTPFunc = oldReadDir
		openSSHClientsFunc = oldOpen
		closeSFTPClientFunc = oldCloseSFTP
		closeSSHClientFunc = oldCloseSSH
	})

	closeSFTPClientFunc = func(*sftp.Client) {}
	closeSSHClientFunc = func(*ssh.Client) {}

	client := new(sftp.Client)
	readCalls := 0
	dialCalls := 0

	readDirSFTPFunc = func(got *sftp.Client, dir string) (filesys.Listing, error) {
		readCalls++
		if got != client {
			t.Fatalf("unexpected client %p", got)
		}
		return filesys.Listing{}, sftp.ErrSSHFxPermissionDenied
	}
	openSSHClientsFunc = func(fm.SSHSetup) (sshClientBundle, error) {
		dialCalls++
		return sshClientBundle{}, nil
	}

	session := &paneSSHSession{
		setup: fm.SSHSetup{Host: "example.test", Port: 22, User: "ramunas"},
		conn: newSharedSSHConn(sshClientBundle{
			sshClient: new(ssh.Client),
			sftpBase:  new(ssh.Client),
			sftp:      client,
		}),
	}

	_, err := session.readDir("/var/log")
	if !errors.Is(err, sftp.ErrSSHFxPermissionDenied) {
		t.Fatalf("readDir error = %v, want permission denied", err)
	}
	if readCalls != 1 {
		t.Fatalf("readDir calls = %d, want 1", readCalls)
	}
	if dialCalls != 0 {
		t.Fatalf("reconnect dials = %d, want 0", dialCalls)
	}
}

func TestSharedSSHConnReconnectIfCurrentSkipsStaleClient(t *testing.T) {
	oldOpen := openSSHClientsFunc
	oldCloseSFTP := closeSFTPClientFunc
	oldCloseSSH := closeSSHClientFunc
	t.Cleanup(func() {
		openSSHClientsFunc = oldOpen
		closeSFTPClientFunc = oldCloseSFTP
		closeSSHClientFunc = oldCloseSSH
	})

	closeSFTPClientFunc = func(*sftp.Client) {}
	closeSSHClientFunc = func(*ssh.Client) {}

	current := new(sftp.Client)
	stale := new(sftp.Client)
	dialCalls := 0
	openSSHClientsFunc = func(fm.SSHSetup) (sshClientBundle, error) {
		dialCalls++
		return sshClientBundle{}, nil
	}

	conn := newSharedSSHConn(sshClientBundle{
		sshClient: new(ssh.Client),
		sftpBase:  new(ssh.Client),
		sftp:      current,
	})

	if err := conn.reconnectIfCurrent(fm.SSHSetup{Host: "example.test", Port: 22, User: "ramunas"}, stale); err != nil {
		t.Fatalf("reconnectIfCurrent returned error: %v", err)
	}
	if dialCalls != 0 {
		t.Fatalf("reconnect dials = %d, want 0", dialCalls)
	}
	if got := conn.sftpClient(); got != current {
		t.Fatalf("conn client = %p, want %p", got, current)
	}
}

func TestNavigateRemoteFavoriteReusesCurrentPaneSSHSession(t *testing.T) {
	oldReadDir := readDirSFTPFunc
	oldOpen := openSSHClientsFunc
	t.Cleanup(func() {
		readDirSFTPFunc = oldReadDir
		openSSHClientsFunc = oldOpen
	})

	openSSHClientsFunc = func(fm.SSHSetup) (sshClientBundle, error) {
		t.Fatal("favorite on current SSH target should reuse the existing pane session")
		return sshClientBundle{}, nil
	}

	cfg := fm.DefaultConfig()
	setup := fm.SSHSetup{Host: "example.test", Port: 2222, User: "ramunas", Password: "secret"}
	client := new(sftp.Client)
	pane := newFilePaneState("/", cfg)
	pane.remote = &paneSSHSession{
		setup:    setup,
		identity: sshSetupIdentity(setup),
		address:  sshSetupAddress(setup),
		conn: newSharedSSHConn(sshClientBundle{
			sshClient: new(ssh.Client),
			sftpBase:  new(ssh.Client),
			sftp:      client,
		}),
	}
	pane.dir = "/home/ramunas"
	ui := &UI{
		fmCfg:     cfg,
		filePanes: []*filePaneState{pane},
	}

	readCalls := 0
	readDirSFTPFunc = func(got *sftp.Client, dir string) (filesys.Listing, error) {
		readCalls++
		if got != client {
			t.Fatalf("unexpected sftp client %p", got)
		}
		if dir != "/var/log" {
			t.Fatalf("readDir dir=%q want /var/log", dir)
		}
		return filesys.Listing{Dir: dir}, nil
	}

	if !ui.navigatePaneFavorite(0, "ssh://ramunas@example.test:2222/var/log") {
		t.Fatal("navigatePaneFavorite returned false")
	}
	if readCalls != 1 {
		t.Fatalf("readDir calls=%d want 1", readCalls)
	}
	if pane.remote == nil || pane.remote.sftpClient() != client {
		t.Fatal("pane should keep the existing SSH session")
	}
	if pane.dir != "/var/log" {
		t.Fatalf("pane dir=%q want /var/log", pane.dir)
	}
}
