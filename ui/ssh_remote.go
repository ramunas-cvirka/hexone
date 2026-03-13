// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"fmt"
	"hexone/filesys"
	"hexone/fm"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type sharedSSHConn struct {
	mu sync.Mutex

	refs int

	sshClient *ssh.Client
	sftpBase  *ssh.Client
	sftp      *sftp.Client
}

func (c *sharedSSHConn) retain() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.refs++
	c.mu.Unlock()
}

func (c *sharedSSHConn) release() {
	if c == nil {
		return
	}
	var (
		sshClient *ssh.Client
		sftpBase  *ssh.Client
		sftpCli   *sftp.Client
	)
	c.mu.Lock()
	if c.refs > 0 {
		c.refs--
	}
	if c.refs == 0 {
		sftpCli = c.sftp
		sftpBase = c.sftpBase
		sshClient = c.sshClient
		c.sftp = nil
		c.sftpBase = nil
		c.sshClient = nil
	}
	c.mu.Unlock()

	if sftpCli != nil {
		_ = sftpCli.Close()
	}
	if sftpBase != nil {
		_ = sftpBase.Close()
	}
	if sshClient != nil {
		_ = sshClient.Close()
	}
}

func (c *sharedSSHConn) sftpClient() *sftp.Client {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sftp
}

func (c *sharedSSHConn) commandClient() *ssh.Client {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sshClient
}

type paneSSHSession struct {
	setup    fm.SSHSetup
	identity string
	address  string
	conn     *sharedSSHConn
}

func (s *paneSSHSession) close() {
	if s == nil {
		return
	}
	if s.conn != nil {
		s.conn.release()
		s.conn = nil
	}
}

func (s *paneSSHSession) homeDir() string {
	client := s.sftpClient()
	if s == nil || client == nil {
		return "/"
	}
	wd, err := client.Getwd()
	if err != nil || strings.TrimSpace(wd) == "" {
		return "/"
	}
	return wd
}

func (s *paneSSHSession) readDir(dir string) (filesys.Listing, error) {
	client := s.sftpClient()
	if s == nil || client == nil {
		return filesys.Listing{}, errors.New("sftp session is not connected")
	}
	return filesys.ReadDirSFTP(client, dir)
}

func (s *paneSSHSession) sftpClient() *sftp.Client {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.sftpClient()
}

func (s *paneSSHSession) commandClient() *ssh.Client {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.commandClient()
}

func (s *paneSSHSession) clone() *paneSSHSession {
	if s == nil || s.conn == nil {
		return nil
	}
	s.conn.retain()
	return &paneSSHSession{
		setup:    s.setup,
		identity: s.identity,
		address:  s.address,
		conn:     s.conn,
	}
}

func (s *paneSSHSession) displayPrefix() string {
	if s == nil {
		return ""
	}
	user := strings.TrimSpace(s.setup.User)
	host := strings.TrimSpace(s.setup.Host)
	switch {
	case user != "" && host != "":
		return user + "@" + host
	case host != "":
		return host
	case user != "":
		return user + "@?"
	default:
		return "ssh"
	}
}

func (ui *UI) connectSSHModalToActivePane(now time.Time) error {
	if ui == nil {
		return errors.New("ui is nil")
	}
	if ui.sshModal == nil {
		return errors.New("ssh modal is not open")
	}
	pane := ui.activePane()
	if pane == nil {
		return errors.New("no active pane")
	}

	setup, err := ui.currentSSHModalSetup()
	if err != nil {
		return err
	}
	return ui.connectPaneSSH(ui.activeFilePane, setup, "", now)
}

func (ui *UI) connectPaneSSH(idx int, setup fm.SSHSetup, targetDir string, now time.Time) error {
	if ui == nil {
		return errors.New("ui is nil")
	}
	if idx < 0 || idx >= len(ui.filePanes) {
		return errors.New("invalid pane index")
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		return errors.New("pane is nil")
	}

	setup, err := normalizeConnectSSHSetup(setup)
	if err != nil {
		return err
	}
	var next *paneSSHSession
	if shared := ui.findReusableRemoteSession(idx, setup); shared != nil {
		next = shared.clone()
	}
	if next == nil {
		next, err = newPaneSSHSession(setup)
		if err != nil {
			return err
		}
	}

	prev := pane.remote
	prevLocal := pane.localDirBeforeRemote
	if prev == nil && pane.dir != "" {
		pane.localDirBeforeRemote = pane.dir
	}
	pane.remote = next

	target := strings.TrimSpace(targetDir)
	if target == "" {
		target = next.homeDir()
	}
	if target == "" {
		target = "/"
	}
	target = path.Clean(target)
	if target == "" || target == "." {
		target = "/"
	}
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}

	if err := pane.load(target); err != nil {
		pane.remote = prev
		pane.localDirBeforeRemote = prevLocal
		next.close()
		return err
	}
	if prev != nil {
		prev.close()
	}

	pane.sortMenuOpen = false
	pane.closeDriveMenu()
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	pane.stopPathEdit()
	pane.setNotice("connected: "+next.identity, now)
	return nil
}

func (ui *UI) findReusableRemoteSession(excludeIdx int, setup fm.SSHSetup) *paneSSHSession {
	if ui == nil {
		return nil
	}
	for i, pane := range ui.filePanes {
		if i == excludeIdx || pane == nil || pane.remote == nil {
			continue
		}
		if !sameSSHRemoteTarget(pane.remote.setup, setup) {
			continue
		}
		if pane.remote.sftpClient() == nil {
			continue
		}
		return pane.remote
	}
	return nil
}

func sameSSHRemoteTarget(a, b fm.SSHSetup) bool {
	hostA := strings.TrimSpace(a.Host)
	hostB := strings.TrimSpace(b.Host)
	userA := strings.TrimSpace(a.User)
	userB := strings.TrimSpace(b.User)
	portA := a.Port
	portB := b.Port
	if portA <= 0 {
		portA = 22
	}
	if portB <= 0 {
		portB = 22
	}
	return hostA == hostB && userA == userB && portA == portB
}

func (ui *UI) disconnectPaneSSH(idx int, now time.Time) {
	if ui == nil || idx < 0 || idx >= len(ui.filePanes) {
		return
	}
	pane := ui.filePanes[idx]
	if pane == nil || pane.remote == nil {
		return
	}
	prev := pane.remote
	pane.remote = nil
	prev.close()

	target := strings.TrimSpace(pane.localDirBeforeRemote)
	if target == "" {
		target = "."
	}
	pane.sortMenuOpen = false
	pane.closeDriveMenu()
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	pane.stopPathEdit()
	if !ui.requestPaneLoadWithSelection(idx, target, "", "", 0) {
		pane.setNotice("disconnect failed", now)
		return
	}
	pane.setNotice("disconnected: "+prev.identity, now)
}

func (ui *UI) currentSSHModalSetup() (fm.SSHSetup, error) {
	if ui == nil || ui.sshModal == nil {
		return fm.SSHSetup{}, errors.New("ssh modal is not open")
	}
	st := ui.sshModal
	if st.selected >= 0 && st.selected < len(st.setups) {
		st.syncSelectedFromEditors()
		return st.setups[st.selected], nil
	}
	setup, hasInput := st.currentEditorSetup()
	if !hasInput {
		return fm.SSHSetup{}, errors.New("no ssh setup selected")
	}
	return setup, nil
}

func normalizeConnectSSHSetup(raw fm.SSHSetup) (fm.SSHSetup, error) {
	setup := fm.SSHSetup{
		Name:          strings.TrimSpace(raw.Name),
		Host:          strings.TrimSpace(raw.Host),
		Port:          raw.Port,
		User:          strings.TrimSpace(raw.User),
		Password:      raw.Password,
		KeyPath:       strings.TrimSpace(raw.KeyPath),
		KeyPassphrase: raw.KeyPassphrase,
	}
	if setup.Host == "" {
		return fm.SSHSetup{}, errors.New("host is required")
	}
	if setup.User == "" {
		return fm.SSHSetup{}, errors.New("user is required")
	}
	if setup.Port <= 0 {
		setup.Port = 22
	}
	if setup.Port > 65535 {
		return fm.SSHSetup{}, errors.New("port must be between 1 and 65535")
	}
	if setup.Password == "" && setup.KeyPath == "" {
		return fm.SSHSetup{}, errors.New("provide password or key path")
	}
	if setup.Name == "" {
		setup.Name = sshSetupIdentity(setup)
	}
	return setup, nil
}

func newPaneSSHSession(setup fm.SSHSetup) (*paneSSHSession, error) {
	setup, err := normalizeConnectSSHSetup(setup)
	if err != nil {
		return nil, err
	}
	auth, err := sshAuthMethods(setup)
	if err != nil {
		return nil, err
	}

	cfg := &ssh.ClientConfig{
		User:            setup.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: replace with known_hosts verification.
		Timeout:         8 * time.Second,
	}
	address := net.JoinHostPort(setup.Host, strconv.Itoa(setup.Port))

	cmdClient, err := ssh.Dial("tcp", address, cfg)
	if err != nil {
		return nil, fmt.Errorf("ssh command session: %w", err)
	}
	sftpBase, err := ssh.Dial("tcp", address, cfg)
	if err != nil {
		_ = cmdClient.Close()
		return nil, fmt.Errorf("ssh sftp session: %w", err)
	}
	sftpClient, err := sftp.NewClient(sftpBase)
	if err != nil {
		_ = sftpBase.Close()
		_ = cmdClient.Close()
		return nil, fmt.Errorf("sftp init: %w", err)
	}

	return &paneSSHSession{
		setup:    setup,
		identity: sshSetupIdentity(setup),
		address:  address,
		conn: &sharedSSHConn{
			refs:      1,
			sshClient: cmdClient,
			sftpBase:  sftpBase,
			sftp:      sftpClient,
		},
	}, nil
}

func sshAuthMethods(setup fm.SSHSetup) ([]ssh.AuthMethod, error) {
	methods := make([]ssh.AuthMethod, 0, 2)

	if setup.KeyPath != "" {
		keyData, err := os.ReadFile(setup.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("read key %q: %w", setup.KeyPath, err)
		}

		var signer ssh.Signer
		if setup.KeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(setup.KeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyData)
		}
		if err != nil {
			return nil, fmt.Errorf("parse key %q: %w", setup.KeyPath, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if setup.Password != "" {
		methods = append(methods, ssh.Password(setup.Password))
	}

	if len(methods) == 0 {
		return nil, errors.New("no ssh authentication methods configured")
	}
	return methods, nil
}
