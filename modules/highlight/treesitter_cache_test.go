// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"strings"
	"testing"

	"github.com/odvcencio/gotreesitter"
)

var treeSitterCacheTestGoSnippet = []byte(`package main

import "fmt"

func main() {
	for i := 0; i < 100; i++ {
		fmt.Println(i)
	}
}
`)

func TestTreeSitterRendererRenderLinesCacheCopiesOutput(t *testing.T) {
	entry := resolveTreeSitterEntry("bench.go", "Go")
	if entry == nil {
		t.Skip("tree-sitter entry unavailable for Go")
	}
	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		t.Skip("tree-sitter renderer unavailable for Go")
	}

	code := treeSitterCacheTestGoSnippet
	lines1, ok := renderer.renderLines(code)
	if !ok || len(lines1) == 0 {
		t.Fatal("expected initial renderLines success with non-empty output")
	}

	originalFirst := lines1[0]
	lines1[0] = "tampered"

	lines2, ok := renderer.renderLines(code)
	if !ok || len(lines2) == 0 {
		t.Fatal("expected cached renderLines success with non-empty output")
	}
	if lines2[0] != originalFirst {
		t.Fatalf("cached output was mutated by caller: got %q want %q", lines2[0], originalFirst)
	}
	if lines2[0] == "tampered" {
		t.Fatalf("cached output returned caller-mutated value %q", lines2[0])
	}
}

func TestTreeSitterRendererRenderLinesCacheInvalidatesOnSourceChange(t *testing.T) {
	entry := resolveTreeSitterEntry("bench.go", "Go")
	if entry == nil {
		t.Skip("tree-sitter entry unavailable for Go")
	}
	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		t.Skip("tree-sitter renderer unavailable for Go")
	}

	srcA := []byte("package main\nfunc a() int { return 1 }\n")
	srcB := []byte("package main\nfunc b() int { return 2 }\n")

	linesA1, ok := renderer.renderLines(srcA)
	if !ok {
		t.Fatal("expected renderLines success for srcA")
	}
	linesB, ok := renderer.renderLines(srcB)
	if !ok {
		t.Fatal("expected renderLines success for srcB")
	}
	linesA2, ok := renderer.renderLines(srcA)
	if !ok {
		t.Fatal("expected renderLines success for srcA after invalidation")
	}

	if len(linesA1) == 0 || len(linesA2) == 0 || len(linesB) == 0 {
		t.Fatal("expected non-empty line output for both sources")
	}
	if len(linesA1) != len(linesA2) {
		t.Fatalf("srcA line count changed across cache cycle: %d vs %d", len(linesA1), len(linesA2))
	}
	if linesA1[0] != linesA2[0] {
		t.Fatalf("srcA first line changed across cache cycle: %q vs %q", linesA1[0], linesA2[0])
	}
}

func TestTreeSitterRendererRenderTrimModeCachesSeparately(t *testing.T) {
	entry := resolveTreeSitterEntry("bench.go", "Go")
	if entry == nil {
		t.Skip("tree-sitter entry unavailable for Go")
	}
	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		t.Skip("tree-sitter renderer unavailable for Go")
	}

	code := []byte("package main\n")

	full, ok := renderer.render(code, false)
	if !ok {
		t.Fatal("expected render success without trim")
	}
	trimmed, ok := renderer.render(code, true)
	if !ok {
		t.Fatal("expected render success with trim")
	}
	if full == trimmed {
		t.Fatalf("expected different HTML for trimmed vs non-trimmed output, got %q", full)
	}

	trimmed2, ok := renderer.render(code, true)
	if !ok {
		t.Fatal("expected cached render success with trim")
	}
	if trimmed2 != trimmed {
		t.Fatalf("trimmed cache mismatch: %q vs %q", trimmed2, trimmed)
	}
}

func TestTreeSitterRendererCompatibilityFallbackAfterIncrementalEdit(t *testing.T) {
	entry := resolveTreeSitterEntry("nginx.conf", "")
	if entry == nil {
		t.Skip("tree-sitter entry unavailable for nginx")
	}
	renderer := getTreeSitterRenderer(entry)
	if renderer == nil {
		t.Skip("tree-sitter renderer unavailable for nginx")
	}

	multiline := []byte("server {\n  listen 80;\n  location / {\n    return 200;\n  }\n}\n")
	singleLine := []byte("server { listen 80; location / { return 200; } }\n")

	if rendered, ok := renderer.render(multiline, true); !ok || rendered == "" {
		t.Fatal("expected initial multiline render success for nginx")
	}

	rendered, ok := renderer.render(singleLine, true)
	if !ok {
		t.Fatal("expected single-line nginx render success after incremental edit")
	}
	if !strings.Contains(string(rendered), `<span class="k">server</span>`) {
		t.Fatalf("expected compatibility-highlighted nginx output, got %q", rendered)
	}
}

func TestComputeSingleInputEditReplaceMiddle(t *testing.T) {
	oldSrc := []byte("a\nbc\ndef\n")
	newSrc := []byte("a\nZZ\ndef\n")

	edit, ok := computeSingleInputEdit(oldSrc, newSrc)
	if !ok {
		t.Fatal("expected edit for changed source")
	}

	if edit.StartByte != 2 || edit.OldEndByte != 4 || edit.NewEndByte != 4 {
		t.Fatalf("unexpected byte edit: %+v", edit)
	}
	if edit.StartPoint != (gotreesitter.Point{Row: 1, Column: 0}) {
		t.Fatalf("unexpected StartPoint: %+v", edit.StartPoint)
	}
	if edit.OldEndPoint != (gotreesitter.Point{Row: 1, Column: 2}) {
		t.Fatalf("unexpected OldEndPoint: %+v", edit.OldEndPoint)
	}
	if edit.NewEndPoint != (gotreesitter.Point{Row: 1, Column: 2}) {
		t.Fatalf("unexpected NewEndPoint: %+v", edit.NewEndPoint)
	}
}

func TestComputeSingleInputEditInsertTail(t *testing.T) {
	oldSrc := []byte("line1\nline2\n")
	newSrc := []byte("line1\nline2\nline3\n")

	edit, ok := computeSingleInputEdit(oldSrc, newSrc)
	if !ok {
		t.Fatal("expected edit for appended source")
	}
	if edit.StartByte != uint32(len(oldSrc)) {
		t.Fatalf("unexpected StartByte: %d", edit.StartByte)
	}
	if edit.OldEndByte != uint32(len(oldSrc)) || edit.NewEndByte != uint32(len(newSrc)) {
		t.Fatalf("unexpected end bytes: old=%d new=%d", edit.OldEndByte, edit.NewEndByte)
	}
	if edit.StartPoint != (gotreesitter.Point{Row: 2, Column: 0}) {
		t.Fatalf("unexpected StartPoint: %+v", edit.StartPoint)
	}
	if edit.OldEndPoint != (gotreesitter.Point{Row: 2, Column: 0}) {
		t.Fatalf("unexpected OldEndPoint: %+v", edit.OldEndPoint)
	}
	if edit.NewEndPoint != (gotreesitter.Point{Row: 3, Column: 0}) {
		t.Fatalf("unexpected NewEndPoint: %+v", edit.NewEndPoint)
	}
}
