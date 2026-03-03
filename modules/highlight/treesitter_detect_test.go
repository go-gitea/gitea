// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import "testing"

func TestResolveTreeSitterEntryPrefersLanguageMetadataForAmbiguousExtension(t *testing.T) {
	// ".h" is ambiguous in practice; metadata should win over extension-only
	// fallback so we select the intended parser.
	cases := []struct {
		fileName string
		fileLang string
		want     string
	}{
		{fileName: "foo.h", fileLang: "Objective-C", want: "objc"},
		{fileName: "foo.h", fileLang: "C++", want: "cpp"},
	}

	for _, tc := range cases {
		t.Run(tc.fileLang, func(t *testing.T) {
			entry := resolveTreeSitterEntry(tc.fileName, tc.fileLang)
			if entry == nil {
				t.Fatalf("resolveTreeSitterEntry(%q, %q) returned nil", tc.fileName, tc.fileLang)
			}
			if entry.Name != tc.want {
				t.Fatalf("resolveTreeSitterEntry(%q, %q) = %q, want %q", tc.fileName, tc.fileLang, entry.Name, tc.want)
			}
		})
	}
}

func TestResolveTreeSitterEntryFallsBackToFilename(t *testing.T) {
	entry := resolveTreeSitterEntry("main.go", "")
	if entry == nil {
		t.Fatalf("resolveTreeSitterEntry(%q, \"\") returned nil", "main.go")
	}
	if entry.Name != "go" {
		t.Fatalf("resolveTreeSitterEntry(%q, \"\") = %q, want %q", "main.go", entry.Name, "go")
	}
}

func TestResolveTreeSitterEntryFallsBackToChromaAlias(t *testing.T) {
	// "ksh" is a chroma alias for Bash. It is a realistic metadata value from
	// external detectors, and should still resolve to a tree-sitter grammar.
	entry := resolveTreeSitterEntry("unknown.ext", "ksh")
	if entry == nil {
		t.Fatalf("resolveTreeSitterEntry(%q, %q) returned nil", "unknown.ext", "ksh")
	}
	if entry.Name != "bash" {
		t.Fatalf("resolveTreeSitterEntry(%q, %q) = %q, want %q", "unknown.ext", "ksh", entry.Name, "bash")
	}
}

func TestResolveTreeSitterEntryWithAnalyzeFallsBackToChromaAlias(t *testing.T) {
	entry := resolveTreeSitterEntryWithAnalyze("unknown.ext", "ksh", []byte("echo hi\n"))
	if entry == nil {
		t.Fatalf("resolveTreeSitterEntryWithAnalyze(%q, %q, code) returned nil", "unknown.ext", "ksh")
	}
	if entry.Name != "bash" {
		t.Fatalf("resolveTreeSitterEntryWithAnalyze(%q, %q, code) = %q, want %q", "unknown.ext", "ksh", entry.Name, "bash")
	}
}
