// Copyright 2026 Ramunas Cvirka
// SPDX-License-Identifier: Apache-2.0

package resources

import "testing"

func TestBundledNerdFontFamilies(t *testing.T) {
	families := BundledFontFamilies()
	if got, want := len(families), 4; got != want {
		t.Fatalf("family count=%d want %d", got, want)
	}
	wantNames := []string{
		BundledFontFamilyFiraCodeNerdFontMono,
		BundledFontFamilyJetBrainsMonoNerdFontMono,
		BundledFontFamilyHackNerdFontMono,
		BundledFontFamilyIosevkaNerdFontMono,
	}
	wantLabels := []string{"FiraCode", "JetBrains", "Hack", "Iosevka"}
	for i, name := range wantNames {
		if got := families[i].Name; got != name {
			t.Fatalf("family[%d]=%q want %q", i, got, name)
		}
		if got := families[i].Label; got != wantLabels[i] {
			t.Fatalf("family[%d] label=%q want %q", i, got, wantLabels[i])
		}
		if !IsBundledFontFamily(name) {
			t.Fatalf("%q should be a bundled font family", name)
		}
		for _, path := range []string{families[i].RegularPath, families[i].MediumPath, families[i].BoldPath} {
			data, ok := BundledFont(path)
			if !ok {
				t.Fatalf("%q should resolve to embedded bytes", path)
			}
			if len(data) < 1024 {
				t.Fatalf("%q embedded bytes too small: %d", path, len(data))
			}
		}
	}
	if !IsBundledMonospaceFontFamily(BundledFontFamilyFiraCodeNerdFontMono) {
		t.Fatal("FiraCode Nerd Font Mono should be marked monospace")
	}
	if !IsBundledMonospaceFontFamily(BundledFontFamilyJetBrainsMonoNerdFontMono) {
		t.Fatal("JetBrainsMono Nerd Font Mono should be marked monospace")
	}
	if !IsBundledMonospaceFontFamily(BundledFontFamilyHackNerdFontMono) {
		t.Fatal("Hack Nerd Font Mono should be marked monospace")
	}
	if !IsBundledMonospaceFontFamily(BundledFontFamilyIosevkaNerdFontMono) {
		t.Fatal("Iosevka Nerd Font Mono should be marked monospace")
	}
}
