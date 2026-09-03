// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package filesys

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pkg/sftp"
)

// sftpStatInfo mimics what the sftp client hands to the listing code: an
// os.FileInfo whose Sys() returns a *sftp.FileStat. Only Sys() is exercised, so
// the embedded interface stays nil.
//
// That shape is an assumption about a dependency, and this test cannot catch it
// changing: the test supplies the value it then asserts on. If pkg/sftp made
// Sys() return a value type or renamed FileStat, statOwnerIDs would stop
// matching and the SFTP owner field would go silently empty in production while
// this test kept passing. So the contract is verified by hand instead, against
// pkg/sftp v1.13.11: fileInfo.Sys() returns fi.stat (attrs.go:43), that field is
// typed *FileStat (attrs.go:25), and FileStat.UID/GID are uint32
// (attrs.go:54-55). Re-check on upgrade.
//
// One consequence worth recording: pkg/sftp fills UID/GID only when the server
// sent SSH_FILEXFER_ATTR_UIDGID, and FileStat keeps no flag field to say
// whether it did (unmarshalFileStat, packet.go:195). A server that reports no
// owner is therefore indistinguishable from root, and both render as "0:0".
// That ambiguity is accepted by design — suppressing 0:0 would hide genuine
// root ownership, which is the worse error of the two.
type sftpStatInfo struct {
	os.FileInfo
	stat *sftp.FileStat
}

func (i sftpStatInfo) Sys() any { return i.stat }

func TestRemoteOwnerTextStaysNumeric(t *testing.T) {
	info := sftpStatInfo{stat: &sftp.FileStat{UID: 1000, GID: 2000}}
	if got := remoteOwnerText(info); got != "1000:2000" {
		t.Fatalf("remoteOwnerText = %q, want %q", got, "1000:2000")
	}
}

func TestOwnerTextEmptyWithoutOwnerIDs(t *testing.T) {
	if got := remoteOwnerText(nil); got != "" {
		t.Fatalf("remoteOwnerText(nil) = %q, want empty", got)
	}
	if got := localOwnerText(nil); got != "" {
		t.Fatalf("localOwnerText(nil) = %q, want empty", got)
	}
}

func TestFormatOwnerNumeric(t *testing.T) {
	if got := formatOwnerNumeric(501, 20); got != "501:20" {
		t.Fatalf("formatOwnerNumeric(501, 20) = %q, want %q", got, "501:20")
	}
}

func TestFormatOwnerNames(t *testing.T) {
	if got := formatOwnerNames("ramunas", "staff"); got != "ramunas:staff" {
		t.Fatalf("got %q, want %q", got, "ramunas:staff")
	}
}

// resetOwnerCaches empties the process-global lookup caches. The lookup tests
// below seed them with uids that resolve to their numeric fallback, and those
// entries would otherwise outlive the test and surprise any later test that
// asserts a real resolution. Takes the write lock, so it is safe under -race.
func resetOwnerCaches() {
	ownerCacheMu.Lock()
	defer ownerCacheMu.Unlock()
	clear(userNameCache)
	clear(groupNameCache)
}

func TestLookupOwnerNameFallsBackToNumeric(t *testing.T) {
	t.Cleanup(resetOwnerCaches)
	// A uid this high is not in any passwd database, so the lookup fails and
	// the numeric form is used instead.
	const missing = 4294967000
	got := lookupUserName(missing)
	if got != "4294967000" {
		t.Fatalf("lookupUserName(%d) = %q, want the numeric fallback", missing, got)
	}
}

func TestLookupOwnerNameIsCached(t *testing.T) {
	t.Cleanup(resetOwnerCaches)
	const missing = 4294966999
	first := lookupUserName(missing)
	second := lookupUserName(missing)
	if first != second {
		t.Fatalf("cached lookup returned %q then %q", first, second)
	}
	ownerCacheMu.RLock()
	_, cached := userNameCache[missing]
	ownerCacheMu.RUnlock()
	if !cached {
		t.Fatalf("lookupUserName did not populate the cache")
	}
}

func TestLocalListingPopulatesOwnerText(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows local files carry no uid/gid")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	listing, err := ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range listing.Entries {
		if entry.Name != "file.txt" {
			continue
		}
		if entry.OwnerText == "" {
			t.Fatalf("OwnerText was not populated for a local file")
		}
		return
	}
	t.Fatalf("file.txt not found in listing")
}

func TestParentEntryHasNoOwner(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "child")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	listing, err := ReadDir(sub)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range listing.Entries {
		if entry.Kind == EntryParent && entry.OwnerText != "" {
			t.Fatalf("parent entry OwnerText = %q, want empty", entry.OwnerText)
		}
	}
}
