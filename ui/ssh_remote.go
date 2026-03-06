package ui

import (
	"errors"
	"fmt"
	"hexone/filesys"
	"hexone/fm"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type paneSSHSession struct {
	setup     fm.SSHSetup
	identity  string
	address   string
	sshClient *ssh.Client

	sftpBase   *ssh.Client
	sftpClient *sftp.Client
}

func (s *paneSSHSession) close() {
	if s == nil {
		return
	}
	if s.sftpClient != nil {
		_ = s.sftpClient.Close()
		s.sftpClient = nil
	}
	if s.sftpBase != nil {
		_ = s.sftpBase.Close()
		s.sftpBase = nil
	}
	if s.sshClient != nil {
		_ = s.sshClient.Close()
		s.sshClient = nil
	}
}

func (s *paneSSHSession) homeDir() string {
	if s == nil || s.sftpClient == nil {
		return "/"
	}
	wd, err := s.sftpClient.Getwd()
	if err != nil || strings.TrimSpace(wd) == "" {
		return "/"
	}
	return wd
}

func (s *paneSSHSession) readDir(dir string) (filesys.Listing, error) {
	if s == nil || s.sftpClient == nil {
		return filesys.Listing{}, errors.New("sftp session is not connected")
	}
	return filesys.ReadDirSFTP(s.sftpClient, dir)
}

func (s *paneSSHSession) displayPrefix() string {
	if s == nil {
		return ""
	}
	return s.identity
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
	setup, err = normalizeConnectSSHSetup(setup)
	if err != nil {
		return err
	}
	next, err := newPaneSSHSession(setup)
	if err != nil {
		return err
	}

	prev := pane.remote
	prevLocal := pane.localDirBeforeRemote
	if prev == nil && pane.dir != "" {
		pane.localDirBeforeRemote = pane.dir
	}
	pane.remote = next

	targetDir := next.homeDir()
	if targetDir == "" {
		targetDir = "/"
	}
	if err := pane.load(targetDir); err != nil {
		pane.remote = prev
		pane.localDirBeforeRemote = prevLocal
		next.close()
		return err
	}
	if prev != nil {
		prev.close()
	}

	pane.sortMenuOpen = false
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	pane.stopPathEdit()
	pane.setNotice("connected: "+next.identity, now)
	return nil
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
	if err := pane.load(target); err != nil {
		pane.setNotice("disconnect failed: "+err.Error(), now)
		return
	}
	pane.sortMenuOpen = false
	pane.closeFavoriteMenu()
	pane.closeContextMenu()
	pane.stopPathEdit()
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
		setup:      setup,
		identity:   sshSetupIdentity(setup),
		address:    address,
		sshClient:  cmdClient,
		sftpBase:   sftpBase,
		sftpClient: sftpClient,
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
