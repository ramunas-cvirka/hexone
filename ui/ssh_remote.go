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
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type sshClientBundle struct {
	sshClient *ssh.Client
	sftpBase  *ssh.Client
	sftp      *sftp.Client
}

func (b sshClientBundle) close() {
	if b.sftp != nil {
		closeSFTPClientFunc(b.sftp)
	}
	if b.sftpBase != nil && b.sftpBase != b.sshClient {
		closeSSHClientFunc(b.sftpBase)
	}
	if b.sshClient != nil {
		closeSSHClientFunc(b.sshClient)
	}
}

var readDirSFTPFunc = filesys.ReadDirSFTP

var sftpGetwdFunc = func(client *sftp.Client) (string, error) {
	return client.Getwd()
}

var closeSFTPClientFunc = func(client *sftp.Client) {
	_ = client.Close()
}

var closeSSHClientFunc = func(client *ssh.Client) {
	_ = client.Close()
}

var netDialTimeoutFunc = net.DialTimeout

var sshNewClientConnFunc = ssh.NewClientConn

var newSFTPClientFunc = sftp.NewClient

var dialSSHClientWithDeadlineFunc = dialSSHClientWithDeadline

var newSFTPClientWithDeadlineFunc = newSFTPClientWithDeadline

var timeNowFunc = time.Now

var openSSHClientsFunc = openSSHClients

var openSSHConnectionSpecFunc = openSSHConnectionSpec

const sshConnectBudget = 12 * time.Second

type sharedSSHConn struct {
	mu sync.Mutex

	refs int

	reconnecting  bool
	reconnectCond *sync.Cond

	sshClient *ssh.Client
	sftpBase  *ssh.Client
	sftp      *sftp.Client
}

func newSharedSSHConn(bundle sshClientBundle) *sharedSSHConn {
	conn := &sharedSSHConn{
		refs:      1,
		sshClient: bundle.sshClient,
		sftpBase:  bundle.sftpBase,
		sftp:      bundle.sftp,
	}
	conn.reconnectCond = sync.NewCond(&conn.mu)
	return conn
}

func (c *sharedSSHConn) ensureReconnectCondLocked() {
	if c.reconnectCond == nil {
		c.reconnectCond = sync.NewCond(&c.mu)
	}
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
	var old sshClientBundle
	c.mu.Lock()
	c.ensureReconnectCondLocked()
	if c.refs > 0 {
		c.refs--
	}
	if c.refs == 0 {
		old = sshClientBundle{
			sshClient: c.sshClient,
			sftpBase:  c.sftpBase,
			sftp:      c.sftp,
		}
		c.sftp = nil
		c.sftpBase = nil
		c.sshClient = nil
	}
	c.mu.Unlock()

	old.close()
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

func (c *sharedSSHConn) reconnectIfCurrent(setup fm.SSHSetup, failed *sftp.Client) error {
	return c.reconnectSpecIfCurrent(directSSHConnectionSpec(setup), failed)
}

func (c *sharedSSHConn) reconnectSpecIfCurrent(spec sshConnectionSpec, failed *sftp.Client) error {
	if c == nil {
		return errors.New("sftp session is not connected")
	}

	c.mu.Lock()
	c.ensureReconnectCondLocked()
	for c.reconnecting {
		c.reconnectCond.Wait()
		if failed != nil && c.sftp != failed {
			c.mu.Unlock()
			return nil
		}
		if failed == nil && c.sftp != nil {
			c.mu.Unlock()
			return nil
		}
	}
	if c.refs == 0 {
		c.mu.Unlock()
		return errors.New("sftp session is not connected")
	}
	if failed != nil && c.sftp != failed {
		c.mu.Unlock()
		return nil
	}
	if failed == nil && c.sftp != nil {
		c.mu.Unlock()
		return nil
	}
	old := sshClientBundle{
		sshClient: c.sshClient,
		sftpBase:  c.sftpBase,
		sftp:      c.sftp,
	}
	c.reconnecting = true
	c.mu.Unlock()

	bundle, err := openSSHConnectionSpecFunc(spec)

	var closeNow sshClientBundle
	c.mu.Lock()
	c.ensureReconnectCondLocked()
	c.reconnecting = false
	if err != nil {
		c.reconnectCond.Broadcast()
		c.mu.Unlock()
		return err
	}
	if c.refs == 0 {
		closeNow = bundle
		err = errors.New("sftp session is not connected")
	} else {
		c.sshClient = bundle.sshClient
		c.sftpBase = bundle.sftpBase
		c.sftp = bundle.sftp
		closeNow = old
	}
	c.reconnectCond.Broadcast()
	c.mu.Unlock()

	closeNow.close()
	return err
}

type paneSSHSession struct {
	setup    fm.SSHSetup
	spec     sshConnectionSpec
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
	if s == nil {
		return "/"
	}
	client := s.sftpClient()
	if client == nil {
		if err := s.reconnectSFTPClient(nil); err != nil {
			return "/"
		}
		client = s.sftpClient()
	}
	if client == nil {
		return "/"
	}
	wd, err := sftpGetwdFunc(client)
	if err != nil && shouldReconnectSSHTransport(err) {
		if reconnectErr := s.reconnectSFTPClient(client); reconnectErr == nil {
			client = s.sftpClient()
			if client != nil {
				wd, err = sftpGetwdFunc(client)
			}
		}
	}
	if err != nil || strings.TrimSpace(wd) == "" {
		return "/"
	}
	return wd
}

func (s *paneSSHSession) readDir(dir string) (filesys.Listing, error) {
	if s == nil {
		return filesys.Listing{}, errors.New("sftp session is not connected")
	}
	client := s.sftpClient()
	if client == nil {
		if err := s.reconnectSFTPClient(nil); err != nil {
			return filesys.Listing{}, err
		}
		client = s.sftpClient()
	}
	if client == nil {
		return filesys.Listing{}, errors.New("sftp session is not connected")
	}

	listing, err := readDirSFTPFunc(client, dir)
	if err == nil || !shouldReconnectSSHTransport(err) {
		return listing, err
	}
	if reconnectErr := s.reconnectSFTPClient(client); reconnectErr != nil {
		return filesys.Listing{}, reconnectErr
	}
	client = s.sftpClient()
	if client == nil {
		return filesys.Listing{}, errors.New("sftp session is not connected")
	}
	return readDirSFTPFunc(client, dir)
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

func (s *paneSSHSession) reconnectSFTPClient(failed *sftp.Client) error {
	if s == nil || s.conn == nil {
		return errors.New("sftp session is not connected")
	}
	return s.conn.reconnectSpecIfCurrent(s.connectionSpec(), failed)
}

func (s *paneSSHSession) connectionSpec() sshConnectionSpec {
	if s == nil {
		return sshConnectionSpec{}
	}
	if strings.TrimSpace(s.spec.setup.Host) != "" {
		return s.spec
	}
	return directSSHConnectionSpec(s.setup)
}

func (s *paneSSHSession) clone() *paneSSHSession {
	if s == nil || s.conn == nil {
		return nil
	}
	s.conn.retain()
	return &paneSSHSession{
		setup:    s.setup,
		spec:     s.connectionSpec(),
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
	if request := ui.sshModal.transientConnect; request != nil {
		if request.targetTab != nil && !ui.activateFilePaneStateTab(request.pane, request.targetTab) {
			return errors.New("SSH target tab is no longer available")
		}
		if ui.sshModal.selected >= 0 && ui.sshModal.selected < len(ui.sshModal.setups) {
			return ui.connectPaneSSH(request.pane, setup, request.targetDir, now)
		}
		spec := request.spec
		spec.setup.User = strings.TrimSpace(setup.User)
		spec.setup.Password = setup.Password
		spec.setup.KeyPath = strings.TrimSpace(setup.KeyPath)
		spec.setup.KeyPassphrase = setup.KeyPassphrase
		spec.passphrase = setup.KeyPassphrase
		return ui.connectPaneSSHSpec(request.pane, spec, request.targetDir, now)
	}
	return ui.connectPaneSSH(ui.activeFilePane, setup, "", now)
}

func (ui *UI) connectPaneSSH(idx int, setup fm.SSHSetup, targetDir string, now time.Time) error {
	return ui.connectPaneSSHSpec(idx, directSSHConnectionSpec(setup), targetDir, now)
}

func (ui *UI) connectPaneSSHSpec(idx int, spec sshConnectionSpec, targetDir string, now time.Time) error {
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

	setup, err := normalizeConnectSSHSetupWithAuth(spec.setup, !spec.transient)
	if err != nil {
		return err
	}
	spec.setup = setup
	var next *paneSSHSession
	showConnectedNotice := true
	if shared := ui.findReusableRemoteSessionForSpec(idx, spec); shared != nil {
		next = shared.clone()
		showConnectedNotice = false
	}
	if next == nil {
		next, err = newPaneSSHSessionForSpec(spec)
		if err != nil {
			return err
		}
		showConnectedNotice = true
	}

	return ui.attachPaneSSHSession(idx, next, targetDir, now, showConnectedNotice)
}

func (ui *UI) attachPaneSSHSession(idx int, next *paneSSHSession, targetDir string, now time.Time, showConnectedNotice bool) error {
	if ui == nil {
		if next != nil {
			next.close()
		}
		return errors.New("ui is nil")
	}
	if idx < 0 || idx >= len(ui.filePanes) {
		if next != nil {
			next.close()
		}
		return errors.New("invalid pane index")
	}
	pane := ui.filePanes[idx]
	if pane == nil {
		if next != nil {
			next.close()
		}
		return errors.New("pane is nil")
	}
	if next == nil {
		return errors.New("ssh session is not connected")
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
	if showConnectedNotice {
		pane.setNotice("connected: "+next.identity, now)
	}
	return nil
}

func (ui *UI) findReusableRemoteSessionForSpec(excludeIdx int, spec sshConnectionSpec) *paneSSHSession {
	if ui == nil {
		return nil
	}
	var excluded *filePaneState
	if excludeIdx >= 0 && excludeIdx < len(ui.filePanes) {
		excluded = ui.filePanes[excludeIdx]
	}
	for _, pane := range ui.allFilePaneTabPanes() {
		if pane == excluded || pane == nil || pane.remote == nil {
			continue
		}
		if !sameSSHRemoteTarget(pane.remote.setup, spec.setup) &&
			!(strings.EqualFold(strings.TrimSpace(pane.remote.address), strings.TrimSpace(spec.address())) &&
				strings.TrimSpace(pane.remote.setup.User) == strings.TrimSpace(spec.setup.User)) {
			continue
		}
		if pane.remote.sftpClient() == nil {
			continue
		}
		return pane.remote
	}
	return nil
}

func (ui *UI) findReusableRemoteSessionForFavorite(excludeIdx int, loc remoteFavoriteLocation) *paneSSHSession {
	if ui == nil {
		return nil
	}
	var excluded *filePaneState
	if excludeIdx >= 0 && excludeIdx < len(ui.filePanes) {
		excluded = ui.filePanes[excludeIdx]
	}
	for _, pane := range ui.allFilePaneTabPanes() {
		if pane == excluded || pane == nil || pane.remote == nil {
			continue
		}
		if !paneMatchesRemoteFavoriteTarget(pane, loc) {
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
	return strings.EqualFold(hostA, hostB) && userA == userB && portA == portB
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

func normalizeConnectSSHSetupWithAuth(raw fm.SSHSetup, requireExplicitAuth bool) (fm.SSHSetup, error) {
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
	if requireExplicitAuth && setup.Password == "" && setup.KeyPath == "" {
		return fm.SSHSetup{}, errors.New("provide password or key path")
	}
	if setup.Name == "" {
		setup.Name = sshSetupIdentity(setup)
	}
	return setup, nil
}

func shouldReconnectSSHTransport(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	if errors.Is(err, sftp.ErrSSHFxConnectionLost) || errors.Is(err, sftp.ErrSSHFxNoConnection) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "failed to send packet header: eof"),
		strings.Contains(msg, "server unexpectedly closed connection"),
		strings.Contains(msg, "use of closed network connection"),
		strings.Contains(msg, "connection reset by peer"),
		strings.Contains(msg, "broken pipe"),
		strings.Contains(msg, "ssh: disconnect"),
		strings.Contains(msg, "connection lost"),
		strings.Contains(msg, "transport is closing"):
		return true
	default:
		return false
	}
}

func sshSetupAddress(setup fm.SSHSetup) string {
	return net.JoinHostPort(setup.Host, strconv.Itoa(setup.Port))
}

func sshConnectDeadline() time.Time {
	return timeNowFunc().Add(sshConnectBudget)
}

func remainingSSHConnectTime(deadline time.Time) (time.Duration, error) {
	remaining := deadline.Sub(timeNowFunc())
	if remaining <= 0 {
		return 0, fmt.Errorf("ssh connect timed out after %s", sshConnectBudget)
	}
	return remaining, nil
}

func dialSSHClientWithDeadline(address string, cfg *ssh.ClientConfig, deadline time.Time) (*ssh.Client, net.Conn, error) {
	remaining, err := remainingSSHConnectTime(deadline)
	if err != nil {
		return nil, nil, err
	}

	conn, err := netDialTimeoutFunc("tcp", address, remaining)
	if err != nil {
		return nil, nil, err
	}
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	clientConn, chans, reqs, err := sshNewClientConnFunc(conn, address, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		_ = clientConn.Close()
		return nil, nil, err
	}
	return ssh.NewClient(clientConn, chans, reqs), conn, nil
}

func newSFTPClientWithDeadline(base *ssh.Client, rawConn net.Conn, deadline time.Time) (*sftp.Client, error) {
	if base == nil || rawConn == nil {
		return nil, errors.New("ssh sftp session: connection is not established")
	}
	if _, err := remainingSSHConnectTime(deadline); err != nil {
		return nil, err
	}
	if err := rawConn.SetDeadline(deadline); err != nil {
		return nil, err
	}
	client, err := newSFTPClientFunc(base)
	if clearErr := rawConn.SetDeadline(time.Time{}); clearErr != nil {
		if client != nil {
			closeSFTPClientFunc(client)
		}
		return nil, clearErr
	}
	return client, err
}

func openSSHClients(setup fm.SSHSetup) (sshClientBundle, error) {
	auth, err := sshAuthMethods(setup)
	if err != nil {
		return sshClientBundle{}, err
	}

	cfg := &ssh.ClientConfig{
		User:            setup.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: replace with known_hosts verification.
	}
	address := sshSetupAddress(setup)
	deadline := sshConnectDeadline()

	bundle, err := openMultiplexedSSHClients(address, cfg, deadline)
	if err != nil {
		return sshClientBundle{}, fmt.Errorf("ssh connection: %w", err)
	}
	return bundle, nil
}

func openMultiplexedSSHClients(address string, cfg *ssh.ClientConfig, deadline time.Time) (sshClientBundle, error) {
	client, rawConn, err := dialSSHClientWithDeadlineFunc(address, cfg, deadline)
	if err != nil {
		return sshClientBundle{}, err
	}
	sftpClient, err := newSFTPClientWithDeadlineFunc(client, rawConn, deadline)
	if err != nil {
		closeSSHClientFunc(client)
		return sshClientBundle{}, fmt.Errorf("sftp init: %w", err)
	}
	// SSH multiplexes the command sessions and SFTP subsystem as channels on
	// one transport. A second TCP+SSH handshake only adds latency.
	return sshClientBundle{sshClient: client, sftp: sftpClient}, nil
}

func newPaneSSHSessionForSpec(spec sshConnectionSpec) (*paneSSHSession, error) {
	setup, err := normalizeConnectSSHSetupWithAuth(spec.setup, !spec.transient)
	if err != nil {
		return nil, err
	}
	spec.setup = setup
	bundle, err := openSSHConnectionSpecFunc(spec)
	if err != nil {
		return nil, err
	}

	return &paneSSHSession{
		setup:    setup,
		spec:     spec,
		identity: sshSetupIdentity(setup),
		address:  spec.address(),
		conn:     newSharedSSHConn(bundle),
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
