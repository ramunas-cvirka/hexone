// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"hexone/filesys"
	"hexone/fm"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func TestResolveOpenSSHConnectionSpecUsesEffectiveConfig(t *testing.T) {
	oldRun := runOpenSSHConfigCommandFunc
	t.Cleanup(func() { runOpenSSHConfigCommandFunc = oldRun })
	runOpenSSHConfigCommandFunc = func(target terminalSSHTarget) (string, error) {
		if target.Host != "production" {
			t.Fatalf("target host=%q want production", target.Host)
		}
		return "hostname 192.0.2.10\n" +
			"user deploy\n" +
			"port 2222\n" +
			"identityfile ~/.ssh/prod_ed25519\n" +
			"userknownhostsfile ~/.ssh/known_hosts ~/.ssh/known_hosts2\n", nil
	}

	spec, err := resolveOpenSSHConnectionSpec(terminalSSHTarget{Host: "production"})
	if err != nil {
		t.Fatalf("resolveOpenSSHConnectionSpec: %v", err)
	}
	if spec.setup.Host != "production" || spec.setup.User != "deploy" || spec.setup.Port != 2222 {
		t.Fatalf("setup=%+v want deploy@production:2222", spec.setup)
	}
	if spec.dialHost != "192.0.2.10" {
		t.Fatalf("dialHost=%q want 192.0.2.10", spec.dialHost)
	}
	home, _ := os.UserHomeDir()
	if got, want := spec.identityFiles, []string{filepath.Join(home, ".ssh", "prod_ed25519")}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("identityFiles=%v want %v", got, want)
	}
	if len(spec.knownHostsFiles) != 2 {
		t.Fatalf("knownHostsFiles=%v want two files", spec.knownHostsFiles)
	}
}

func TestResolveOpenSSHConnectionSpecRejectsProxyJump(t *testing.T) {
	oldRun := runOpenSSHConfigCommandFunc
	t.Cleanup(func() { runOpenSSHConfigCommandFunc = oldRun })
	runOpenSSHConfigCommandFunc = func(terminalSSHTarget) (string, error) {
		return "hostname internal.test\nuser deploy\nport 22\nproxyjump bastion.test\n", nil
	}

	_, err := resolveOpenSSHConnectionSpec(terminalSSHTarget{Host: "internal"})
	if err == nil {
		t.Fatal("ProxyJump config should be rejected explicitly")
	}
}

func TestResolveOpenSSHConnectionSpecPropagatesConfigFailure(t *testing.T) {
	oldRun := runOpenSSHConfigCommandFunc
	t.Cleanup(func() { runOpenSSHConfigCommandFunc = oldRun })
	runOpenSSHConfigCommandFunc = func(terminalSSHTarget) (string, error) {
		return "", errors.New("malformed ssh config")
	}

	_, err := resolveOpenSSHConnectionSpec(terminalSSHTarget{Host: "production", User: "deploy"})
	if err == nil || err.Error() != "malformed ssh config" {
		t.Fatalf("config error=%v want malformed ssh config", err)
	}
}

func TestResolveOpenSSHConnectionSpecFallsBackOnlyWhenSSHIsMissing(t *testing.T) {
	oldRun := runOpenSSHConfigCommandFunc
	t.Cleanup(func() { runOpenSSHConfigCommandFunc = oldRun })
	runOpenSSHConfigCommandFunc = func(terminalSSHTarget) (string, error) {
		return "", errors.Join(errors.New("ssh unavailable"), exec.ErrNotFound)
	}

	spec, err := resolveOpenSSHConnectionSpec(terminalSSHTarget{Host: "srv.test", User: "deploy", Port: 2200, HasPort: true})
	if err != nil {
		t.Fatalf("missing ssh fallback: %v", err)
	}
	if !spec.transient || spec.dialHost != "srv.test" || spec.setup.User != "deploy" || spec.setup.Port != 2200 {
		t.Fatalf("fallback spec=%+v", spec)
	}
}

func TestResolveOpenSSHConnectionSpecDoesNotDropOptionsWhenSSHIsMissing(t *testing.T) {
	oldRun := runOpenSSHConfigCommandFunc
	t.Cleanup(func() { runOpenSSHConfigCommandFunc = oldRun })
	runOpenSSHConfigCommandFunc = func(terminalSSHTarget) (string, error) {
		return "", exec.ErrNotFound
	}

	_, err := resolveOpenSSHConnectionSpec(terminalSSHTarget{
		Host:        "production",
		User:        "deploy",
		OpenSSHArgs: encodeTerminalOpenSSHArgs([]string{"-F", "custom.conf"}),
	})
	if err == nil || !strings.Contains(err.Error(), "options cannot be honored") {
		t.Fatalf("missing ssh with custom options error=%v", err)
	}
}

func TestResolveOpenSSHConnectionSpecExpandsTokensAndGlobalKnownHosts(t *testing.T) {
	oldRun := runOpenSSHConfigCommandFunc
	t.Cleanup(func() { runOpenSSHConfigCommandFunc = oldRun })
	runOpenSSHConfigCommandFunc = func(terminalSSHTarget) (string, error) {
		return "hostname srv.test\n" +
			"user deploy\n" +
			"port 2222\n" +
			"identityfile ~/.ssh/%r@%h-%p-%n\n" +
			"globalknownhostsfile /etc/ssh/ssh_known_hosts\n" +
			"userknownhostsfile \"~/.ssh/known hosts\"\n", nil
	}

	spec, err := resolveOpenSSHConnectionSpec(terminalSSHTarget{Host: "production"})
	if err != nil {
		t.Fatalf("resolveOpenSSHConnectionSpec: %v", err)
	}
	home, _ := os.UserHomeDir()
	wantIdentity := filepath.Join(home, ".ssh", "deploy@srv.test-2222-production")
	if len(spec.identityFiles) != 1 || spec.identityFiles[0] != wantIdentity {
		t.Fatalf("identityFiles=%v want %q", spec.identityFiles, wantIdentity)
	}
	if !spec.knownHostsConfigured || len(spec.knownHostsFiles) != 2 {
		t.Fatalf("knownHostsConfigured=%v files=%v want global and user files", spec.knownHostsConfigured, spec.knownHostsFiles)
	}
	if filepath.Base(spec.knownHostsFiles[0]) != "ssh_known_hosts" || filepath.Base(spec.knownHostsFiles[1]) != "known hosts" {
		t.Fatalf("knownHostsFiles=%v", spec.knownHostsFiles)
	}
}

func TestSplitOpenSSHConfigWordsPreservesWindowsPaths(t *testing.T) {
	line := `C:\ProgramData\ssh\ssh_known_hosts "C:\Users\Test User\.ssh\known_hosts"`
	got := splitOpenSSHConfigWords(line)
	want := []string{`C:\ProgramData\ssh\ssh_known_hosts`, `C:\Users\Test User\.ssh\known_hosts`}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("words=%q want %q", got, want)
	}
}

func TestSSHAuthMethodsForConnectionSpecRequestsEncryptedKeyPassphrase(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKeyWithPassphrase(privateKey, "test", []byte("correct horse"))
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}

	spec := sshConnectionSpec{
		setup:         fm.SSHSetup{Host: "example.test", Port: 22, User: "alice"},
		identityFiles: []string{keyPath},
		identityAgent: "none",
		transient:     true,
	}
	_, _, _, err = sshAuthMethodsForConnectionSpec(spec)
	var required *sshPassphraseRequiredError
	if !errors.As(err, &required) || required.keyPath != keyPath {
		t.Fatalf("auth error=%v want passphrase request for %q", err, keyPath)
	}

	spec.passphrase = "correct horse"
	methods, encrypted, closer, err := sshAuthMethodsForConnectionSpec(spec)
	if closer != nil {
		defer closer.Close()
	}
	if err != nil {
		t.Fatalf("auth with passphrase: %v", err)
	}
	if len(methods) != 1 || len(encrypted) != 0 {
		t.Fatalf("methods=%d encrypted=%v want one method and no encrypted keys", len(methods), encrypted)
	}
}

func TestSSHAuthMethodsPassphraseIsScopedToRequestedKey(t *testing.T) {
	_, encryptedKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encryptedBlock, err := ssh.MarshalPrivateKeyWithPassphrase(encryptedKey, "encrypted", []byte("correct horse"))
	if err != nil {
		t.Fatal(err)
	}
	_, plainKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	plainBlock, err := ssh.MarshalPrivateKey(plainKey, "plain")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	encryptedPath := filepath.Join(dir, "id_ed25519_encrypted")
	plainPath := filepath.Join(dir, "id_ed25519_plain")
	if err := os.WriteFile(encryptedPath, pem.EncodeToMemory(encryptedBlock), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plainPath, pem.EncodeToMemory(plainBlock), 0o600); err != nil {
		t.Fatal(err)
	}

	spec := sshConnectionSpec{
		setup:         fm.SSHSetup{Host: "example.test", Port: 22, User: "alice", KeyPath: encryptedPath},
		identityFiles: []string{encryptedPath, plainPath},
		identityAgent: "none",
		passphrase:    "correct horse",
		transient:     true,
	}
	methods, encrypted, closer, err := sshAuthMethodsForConnectionSpec(spec)
	if closer != nil {
		defer closer.Close()
	}
	if err != nil {
		t.Fatalf("mixed encrypted and plain identities: %v", err)
	}
	if len(methods) != 1 || len(encrypted) != 0 {
		t.Fatalf("methods=%d encrypted=%v want both signers in one method", len(methods), encrypted)
	}
}

func TestOpenSSHPassphraseRetryPrefillsOneTimeConnection(t *testing.T) {
	cfg := fm.DefaultConfig()
	ui := &UI{fmCfg: cfg, filePanes: []*filePaneState{newFilePaneState(t.TempDir(), cfg)}}
	spec := sshConnectionSpec{
		setup:     fm.SSHSetup{Host: "production", Port: 2222, User: "deploy"},
		dialHost:  "192.0.2.10",
		transient: true,
	}
	err := &sshPassphraseRequiredError{spec: spec, keyPath: "/keys/prod"}

	if !ui.openSSHPassphraseRetry(0, "/var/log", err) {
		t.Fatal("passphrase retry modal was not opened")
	}
	st := ui.sshModal
	if st == nil || st.transientConnect == nil {
		t.Fatal("missing transient SSH modal state")
	}
	if st.hostEdit.Text() != "production" || st.keyPathEdit.Text() != "/keys/prod" || st.transientConnect.targetDir != "/var/log" {
		t.Fatalf("unexpected transient modal state: host=%q key=%q request=%+v", st.hostEdit.Text(), st.keyPathEdit.Text(), st.transientConnect)
	}
	if st.defaultAction() != sshModalActionConnect {
		t.Fatal("one-time passphrase retry should default to Connect")
	}
	for _, target := range st.focusOrder() {
		if target == sshModalFocusHost || target == sshModalFocusKeyPath {
			t.Fatalf("read-only connection field %v remains in transient tab order", target)
		}
	}
}

func TestOpenSSHPassphraseRetryConnectsRequestedPaneAndDirectory(t *testing.T) {
	oldOpenSpec := openSSHConnectionSpecFunc
	oldReadDir := readDirSFTPFunc
	oldCloseSFTP := closeSFTPClientFunc
	oldCloseSSH := closeSSHClientFunc
	t.Cleanup(func() {
		openSSHConnectionSpecFunc = oldOpenSpec
		readDirSFTPFunc = oldReadDir
		closeSFTPClientFunc = oldCloseSFTP
		closeSSHClientFunc = oldCloseSSH
	})
	closeSFTPClientFunc = func(*sftp.Client) {}
	closeSSHClientFunc = func(*ssh.Client) {}

	cfg := fm.DefaultConfig()
	pane := newFilePaneState(t.TempDir(), cfg)
	ui := &UI{fmCfg: cfg, filePanes: []*filePaneState{pane}}
	spec := sshConnectionSpec{
		setup:     fm.SSHSetup{Host: "production", Port: 22, User: "deploy"},
		dialHost:  "srv.test",
		transient: true,
	}
	if !ui.openSSHPassphraseRetry(0, "/var/log", &sshPassphraseRequiredError{spec: spec, keyPath: "/keys/prod"}) {
		t.Fatal("passphrase retry modal was not opened")
	}
	ui.sshModal.keyPassEdit.SetText("secret")
	client := new(sftp.Client)
	openSSHConnectionSpecFunc = func(got sshConnectionSpec) (sshClientBundle, error) {
		if got.passphrase != "secret" || got.setup.KeyPath != "/keys/prod" {
			t.Fatalf("retry spec passphrase=%q key=%q", got.passphrase, got.setup.KeyPath)
		}
		return sshClientBundle{sshClient: new(ssh.Client), sftpBase: new(ssh.Client), sftp: client}, nil
	}
	readDirSFTPFunc = func(got *sftp.Client, dir string) (filesys.Listing, error) {
		if got != client || dir != "/var/log" {
			t.Fatalf("readDir client=%p dir=%q", got, dir)
		}
		return filesys.Listing{Dir: dir}, nil
	}

	if err := ui.connectSSHModalToActivePane(time.Now()); err != nil {
		t.Fatalf("connectSSHModalToActivePane: %v", err)
	}
	if pane.remote == nil || pane.dir != "/var/log" {
		t.Fatalf("retry did not connect requested pane: %+v", pane)
	}
}
