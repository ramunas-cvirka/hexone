// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package buildinfo

import "testing"

func TestHelpVersionTextIncludesCommitOnlyWhenNeeded(t *testing.T) {
	prevVersion, prevSemver, prevCommit := Version, SemVersion, Commit
	defer func() {
		Version = prevVersion
		SemVersion = prevSemver
		Commit = prevCommit
	}()

	Version = "v0.1.0"
	Commit = "abc1234"
	if got := HelpVersionText(); got != "Version v0.1.0 (abc1234)" {
		t.Fatalf("HelpVersionText()=%q want %q", got, "Version v0.1.0 (abc1234)")
	}

	Version = "v0.1.0-3-gabc1234"
	if got := HelpVersionText(); got != "Version v0.1.0-3-gabc1234" {
		t.Fatalf("HelpVersionText()=%q want %q", got, "Version v0.1.0-3-gabc1234")
	}
}
