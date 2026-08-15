// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"hexone/fm"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

const openSSHConfigResolveTimeout = 3 * time.Second

type sshConnectionSpec struct {
	setup                fm.SSHSetup
	dialHost             string
	identityFiles        []string
	identityAgent        string
	knownHostsFiles      []string
	knownHostsConfigured bool
	hostKeyAlias         string
	passphrase           string
	transient            bool
}

func directSSHConnectionSpec(setup fm.SSHSetup) sshConnectionSpec {
	return sshConnectionSpec{setup: setup}
}

func (s sshConnectionSpec) address() string {
	host := strings.TrimSpace(s.dialHost)
	if host == "" {
		host = strings.TrimSpace(s.setup.Host)
	}
	port := s.setup.Port
	if port <= 0 {
		port = 22
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

type sshPassphraseRequiredError struct {
	spec    sshConnectionSpec
	keyPath string
	cause   error
}

type sshTransientConnectRequest struct {
	pane      int
	targetDir string
	spec      sshConnectionSpec
}

func (ui *UI) openSSHPassphraseRetry(pane int, targetDir string, connectErr error) bool {
	if ui == nil {
		return false
	}
	var required *sshPassphraseRequiredError
	if !errors.As(connectErr, &required) || required == nil {
		return false
	}
	ui.openSSHModal()
	st := ui.sshModal
	if st == nil {
		return false
	}
	st.selected = -1
	st.clearEditors()
	setup := required.spec.setup
	st.nameEdit.SetText(setup.Name)
	st.hostEdit.SetText(setup.Host)
	st.portEdit.SetText(strconv.Itoa(setup.Port))
	st.userEdit.SetText(setup.User)
	st.passEdit.SetText(setup.Password)
	st.keyPathEdit.SetText(required.keyPath)
	st.keyPassEdit.SetText("")
	st.transientConnect = &sshTransientConnectRequest{
		pane:      pane,
		targetDir: targetDir,
		spec:      required.spec,
	}
	st.errText = required.Error()
	st.focus = sshModalFocusPassphrase
	st.actionFocus = sshModalActionConnect
	st.focusPassphrase = true
	return true
}

func (e *sshPassphraseRequiredError) Error() string {
	if e == nil {
		return "SSH key passphrase is required"
	}
	if e.keyPath == "" {
		return "SSH key passphrase is required"
	}
	return "passphrase required for " + e.keyPath
}

func (e *sshPassphraseRequiredError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

var runOpenSSHConfigCommandFunc = runOpenSSHConfigCommand

var resolveOpenSSHConnectionSpecFunc = resolveOpenSSHConnectionSpec

func resolveOpenSSHConnectionSpec(target terminalSSHTarget) (sshConnectionSpec, error) {
	host := strings.TrimSpace(target.Host)
	if host == "" {
		return sshConnectionSpec{}, errors.New("terminal SSH host is unavailable")
	}

	output, err := runOpenSSHConfigCommandFunc(target)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			if target.OpenSSHArgs != "" {
				return sshConnectionSpec{}, errors.New("OpenSSH executable is unavailable; terminal SSH options cannot be honored")
			}
			return fallbackOpenSSHConnectionSpec(target), nil
		}
		return sshConnectionSpec{}, err
	}
	values := parseOpenSSHConfigOutput(output)
	resolvedHost := firstOpenSSHConfigValue(values, "hostname")
	if resolvedHost == "" {
		resolvedHost = host
	}
	user := firstOpenSSHConfigValue(values, "user")
	if user == "" {
		user = strings.TrimSpace(target.User)
	}
	if user == "" {
		return sshConnectionSpec{}, errors.New("terminal SSH user is unavailable")
	}
	port := target.Port
	if raw := firstOpenSSHConfigValue(values, "port"); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 && parsed <= 65535 {
			port = parsed
		}
	}
	if port <= 0 {
		port = 22
	}
	tokens := newOpenSSHPathTokens(host, resolvedHost, user, port)

	if proxy := firstOpenSSHConfigValue(values, "proxyjump"); proxy != "" && !strings.EqualFold(proxy, "none") {
		return sshConnectionSpec{}, fmt.Errorf("SSH ProxyJump is not supported yet: %s", proxy)
	}
	if proxy := firstOpenSSHConfigValue(values, "proxycommand"); proxy != "" && !strings.EqualFold(proxy, "none") {
		return sshConnectionSpec{}, errors.New("SSH ProxyCommand is not supported yet")
	}

	return sshConnectionSpec{
		setup: fm.SSHSetup{
			Name: sshSetupIdentity(fm.SSHSetup{User: user, Host: host, Port: port}),
			Host: host,
			Port: port,
			User: user,
		},
		dialHost:             resolvedHost,
		identityFiles:        expandOpenSSHPaths(values["identityfile"], tokens),
		identityAgent:        expandOpenSSHIdentityAgent(firstOpenSSHConfigValue(values, "identityagent"), tokens),
		knownHostsFiles:      expandOpenSSHPathList(append(append([]string(nil), values["globalknownhostsfile"]...), values["userknownhostsfile"]...), tokens),
		knownHostsConfigured: len(values["globalknownhostsfile"]) > 0 || len(values["userknownhostsfile"]) > 0,
		hostKeyAlias:         firstOpenSSHConfigValue(values, "hostkeyalias"),
		transient:            true,
	}, nil
}

func fallbackOpenSSHConnectionSpec(target terminalSSHTarget) sshConnectionSpec {
	port := target.Port
	if port <= 0 {
		port = 22
	}
	setup := fm.SSHSetup{
		Host: strings.TrimSpace(target.Host),
		Port: port,
		User: strings.TrimSpace(target.User),
	}
	setup.Name = sshSetupIdentity(setup)
	home, _ := os.UserHomeDir()
	identities := make([]string, 0, 4)
	for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		if home != "" {
			identities = append(identities, filepath.Join(home, ".ssh", name))
		}
	}
	return sshConnectionSpec{
		setup:           setup,
		dialHost:        setup.Host,
		identityFiles:   identities,
		knownHostsFiles: defaultOpenSSHKnownHostsFiles(),
		transient:       true,
	}
}

func runOpenSSHConfigCommand(target terminalSSHTarget) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), openSSHConfigResolveTimeout)
	defer cancel()
	args := openSSHConfigCommandArgs(target)
	out, err := exec.CommandContext(ctx, "ssh", args...).CombinedOutput()
	if err != nil {
		if detail := strings.TrimSpace(string(out)); detail != "" {
			return "", fmt.Errorf("resolve OpenSSH config: %s: %w", detail, err)
		}
		return "", fmt.Errorf("resolve OpenSSH config: %w", err)
	}
	return string(out), nil
}

func openSSHConfigCommandArgs(target terminalSSHTarget) []string {
	args := []string{"-G"}
	args = append(args, decodeTerminalOpenSSHArgs(target.OpenSSHArgs)...)
	if user := strings.TrimSpace(target.User); user != "" {
		args = append(args, "-l", user)
	}
	if target.HasPort && target.Port > 0 {
		args = append(args, "-p", strconv.Itoa(target.Port))
	}
	args = append(args, "--", strings.TrimSpace(target.Host))
	return args
}

func parseOpenSSHConfigOutput(output string) map[string][]string {
	values := make(map[string][]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		values[key] = append(values[key], value)
	}
	return values
}

func firstOpenSSHConfigValue(values map[string][]string, key string) string {
	if len(values[key]) == 0 {
		return ""
	}
	return strings.TrimSpace(values[key][0])
}

type openSSHPathTokens struct {
	values map[byte]string
}

func newOpenSSHPathTokens(originalHost, resolvedHost, remoteUser string, port int) openSSHPathTokens {
	home, _ := os.UserHomeDir()
	localHost, _ := os.Hostname()
	localShort := strings.SplitN(localHost, ".", 2)[0]
	localUser := ""
	localUID := ""
	if current, err := user.Current(); err == nil && current != nil {
		localUser = current.Username
		localUID = current.Uid
		if home == "" {
			home = current.HomeDir
		}
	}
	hash := sha1.Sum([]byte(localHost + resolvedHost + strconv.Itoa(port) + remoteUser))
	return openSSHPathTokens{values: map[byte]string{
		'%': "%",
		'C': fmt.Sprintf("%x", hash[:]),
		'd': home,
		'h': resolvedHost,
		'i': localUID,
		'L': localShort,
		'l': localHost,
		'n': originalHost,
		'p': strconv.Itoa(port),
		'r': remoteUser,
		'u': localUser,
	}}
}

func expandOpenSSHPaths(paths []string, tokens openSSHPathTokens) []string {
	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		path := expandOpenSSHPath(raw, tokens)
		if path == "" || strings.EqualFold(path, "none") {
			continue
		}
		out = appendUniqueStringFold(out, path)
	}
	return out
}

func expandOpenSSHPathList(lines []string, tokens openSSHPathTokens) []string {
	var paths []string
	for _, line := range lines {
		for _, raw := range splitOpenSSHConfigWords(line) {
			path := expandOpenSSHPath(raw, tokens)
			if path == "" || strings.EqualFold(path, "none") {
				continue
			}
			paths = appendUniqueStringFold(paths, path)
		}
	}
	return paths
}

func expandOpenSSHIdentityAgent(raw string, tokens openSSHPathTokens) string {
	if strings.EqualFold(strings.TrimSpace(raw), "SSH_AUTH_SOCK") {
		return strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
	}
	return expandOpenSSHPath(raw, tokens)
}

func expandOpenSSHPath(raw string, tokens openSSHPathTokens) string {
	raw = strings.TrimSpace(strings.Trim(raw, `"`))
	if raw == "" {
		return ""
	}
	raw = expandOpenSSHPathTokens(raw, tokens)
	if programData := strings.TrimSpace(os.Getenv("PROGRAMDATA")); programData != "" {
		const marker = "__PROGRAMDATA__"
		if len(raw) >= len(marker) && strings.EqualFold(raw[:len(marker)], marker) {
			raw = programData + raw[len(marker):]
		}
	}
	raw = os.ExpandEnv(raw)
	if raw == "~" || strings.HasPrefix(raw, "~/") || strings.HasPrefix(raw, `~\`) {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if raw == "~" {
				return home
			}
			return filepath.Join(home, raw[2:])
		}
	}
	return filepath.Clean(raw)
}

func expandOpenSSHPathTokens(raw string, tokens openSSHPathTokens) string {
	var out strings.Builder
	out.Grow(len(raw))
	for i := 0; i < len(raw); i++ {
		if raw[i] != '%' || i+1 >= len(raw) {
			out.WriteByte(raw[i])
			continue
		}
		next := raw[i+1]
		value, ok := tokens.values[next]
		if !ok {
			out.WriteByte(raw[i])
			continue
		}
		out.WriteString(value)
		i++
	}
	return out.String()
}

func splitOpenSSHConfigWords(line string) []string {
	var words []string
	var word strings.Builder
	quote := rune(0)
	flush := func() {
		if word.Len() > 0 {
			words = append(words, word.String())
			word.Reset()
		}
	}
	for _, r := range line {
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' {
			flush()
			continue
		}
		word.WriteRune(r)
	}
	flush()
	return words
}

func appendUniqueStringFold(values []string, value string) []string {
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}

func defaultOpenSSHKnownHostsFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".ssh", "known_hosts"),
		filepath.Join(home, ".ssh", "known_hosts2"),
	}
}

func openSSHConnectionSpec(spec sshConnectionSpec) (sshClientBundle, error) {
	if !spec.transient {
		return openSSHClientsFunc(spec.setup)
	}
	hostKeyCallback, err := openSSHHostKeyCallback(spec)
	if err != nil {
		return sshClientBundle{}, err
	}
	auth, encryptedKeys, closer, err := sshAuthMethodsForConnectionSpec(spec)
	if closer != nil {
		defer closer.Close()
	}
	if err != nil {
		return sshClientBundle{}, err
	}

	cfg := &ssh.ClientConfig{
		User:            spec.setup.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
	}
	address := spec.address()
	deadline := sshConnectDeadline()
	cmdClient, _, err := dialSSHClientWithDeadline(address, cfg, deadline)
	if err != nil {
		if len(encryptedKeys) > 0 && sshAuthenticationFailed(err) {
			return sshClientBundle{}, &sshPassphraseRequiredError{spec: spec, keyPath: encryptedKeys[0], cause: err}
		}
		return sshClientBundle{}, fmt.Errorf("ssh command session: %w", err)
	}
	sftpBase, sftpConn, err := dialSSHClientWithDeadline(address, cfg, deadline)
	if err != nil {
		closeSSHClientFunc(cmdClient)
		return sshClientBundle{}, fmt.Errorf("ssh sftp session: %w", err)
	}
	sftpClient, err := newSFTPClientWithDeadline(sftpBase, sftpConn, deadline)
	if err != nil {
		closeSSHClientFunc(sftpBase)
		closeSSHClientFunc(cmdClient)
		return sshClientBundle{}, fmt.Errorf("sftp init: %w", err)
	}
	return sshClientBundle{sshClient: cmdClient, sftpBase: sftpBase, sftp: sftpClient}, nil
}

func sshAuthMethodsForConnectionSpec(spec sshConnectionSpec) ([]ssh.AuthMethod, []string, io.Closer, error) {
	paths := make([]string, 0, len(spec.identityFiles)+1)
	if path := strings.TrimSpace(spec.setup.KeyPath); path != "" {
		paths = append(paths, path)
	}
	for _, path := range spec.identityFiles {
		paths = appendUniqueStringFold(paths, strings.TrimSpace(path))
	}

	var signers []ssh.Signer
	var encrypted []string
	for _, keyPath := range paths {
		data, err := os.ReadFile(keyPath)
		if err != nil {
			if strings.EqualFold(keyPath, strings.TrimSpace(spec.setup.KeyPath)) {
				return nil, nil, nil, fmt.Errorf("read key %q: %w", keyPath, err)
			}
			continue
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			var missing *ssh.PassphraseMissingError
			if errors.As(err, &missing) {
				passphrase := ""
				if strings.EqualFold(keyPath, strings.TrimSpace(spec.setup.KeyPath)) ||
					(strings.TrimSpace(spec.setup.KeyPath) == "" && len(paths) == 1) {
					passphrase = spec.passphrase
					if passphrase == "" {
						passphrase = spec.setup.KeyPassphrase
					}
				}
				if passphrase == "" {
					encrypted = append(encrypted, keyPath)
					continue
				}
				signer, err = ssh.ParsePrivateKeyWithPassphrase(data, []byte(passphrase))
				if err == nil {
					signers = append(signers, signer)
					continue
				}
			}
			if strings.EqualFold(keyPath, strings.TrimSpace(spec.setup.KeyPath)) {
				return nil, nil, nil, fmt.Errorf("parse key %q: %w", keyPath, err)
			}
			continue
		}
		signers = append(signers, signer)
	}

	methods := make([]ssh.AuthMethod, 0, 3)
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}

	var agentCloser io.Closer
	if !strings.EqualFold(strings.TrimSpace(spec.identityAgent), "none") {
		if conn, err := dialSSHAgent(spec.identityAgent); err == nil {
			client := agent.NewClient(conn)
			if agentSigners, signerErr := client.Signers(); signerErr == nil && len(agentSigners) > 0 {
				methods = append(methods, ssh.PublicKeys(agentSigners...))
				agentCloser = conn
			} else {
				_ = conn.Close()
			}
		}
	}
	if spec.setup.Password != "" {
		methods = append(methods, ssh.Password(spec.setup.Password))
	}
	if len(methods) == 0 {
		if len(encrypted) > 0 {
			return nil, encrypted, agentCloser, &sshPassphraseRequiredError{spec: spec, keyPath: encrypted[0]}
		}
		return nil, nil, agentCloser, errors.New("no SSH authentication method is available; load a key into ssh-agent or configure an IdentityFile")
	}
	return methods, encrypted, agentCloser, nil
}

func openSSHHostKeyCallback(spec sshConnectionSpec) (ssh.HostKeyCallback, error) {
	files := spec.knownHostsFiles
	if len(files) == 0 && !spec.knownHostsConfigured {
		files = defaultOpenSSHKnownHostsFiles()
	}
	existing := make([]string, 0, len(files))
	for _, file := range files {
		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			existing = append(existing, file)
		}
	}
	if len(existing) == 0 {
		return nil, errors.New("no OpenSSH known_hosts file is available; connect with ssh once to verify the host")
	}
	callback, err := knownhosts.New(existing...)
	if err != nil {
		return nil, fmt.Errorf("read OpenSSH known_hosts: %w", err)
	}
	checkAddress := spec.address()
	if alias := strings.TrimSpace(spec.hostKeyAlias); alias != "" {
		checkAddress = net.JoinHostPort(alias, strconv.Itoa(spec.setup.Port))
	}
	return func(_ string, remote net.Addr, key ssh.PublicKey) error {
		if err := callback(checkAddress, remote, key); err != nil {
			return fmt.Errorf("verify SSH host key for %s: %w", checkAddress, err)
		}
		return nil
	}, nil
}

func sshAuthenticationFailed(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "unable to authenticate") || strings.Contains(text, "no supported methods remain")
}
