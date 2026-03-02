// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"strings"
	"testing"
)

var benchmarkGoSnippet = []byte(strings.Repeat(`package main

import "fmt"

func main() {
	for i := 0; i < 100; i++ {
		fmt.Println(i)
	}
}
`, 500))

func BenchmarkRenderCodeTreeSitterGo(b *testing.B) {
	b.ReportAllocs()
	code := benchmarkGoSnippet

	if _, _, ok := tryRenderCodeByTreeSitter("bench.go", "Go", code, false); !ok {
		b.Skip("tree-sitter renderer is unavailable for Go")
	}

	for b.Loop() {
		if _, _, ok := tryRenderCodeByTreeSitter("bench.go", "Go", code, false); !ok {
			b.Fatal("tree-sitter renderer became unavailable for Go")
		}
	}
}

func BenchmarkRenderCodeTreeSitterGoCold(b *testing.B) {
	b.ReportAllocs()
	base := benchmarkGoSnippet

	if _, _, ok := tryRenderCodeByTreeSitter("bench.go", "Go", base, false); !ok {
		b.Skip("tree-sitter renderer is unavailable for Go")
	}

	codeA := make([]byte, 0, len(base)+32)
	codeA = append(codeA, base...)
	codeA = append(codeA, []byte("// cold-a\n")...)
	codeB := make([]byte, 0, len(base)+32)
	codeB = append(codeB, base...)
	codeB = append(codeB, []byte("// cold-b\n")...)

	for i := 0; i < b.N; i++ {
		buf := codeA
		if i&1 == 1 {
			buf = codeB
		}
		if _, _, ok := tryRenderCodeByTreeSitter("bench.go", "Go", buf, false); !ok {
			b.Fatal("tree-sitter renderer became unavailable for Go")
		}
	}
}

func BenchmarkRenderCodeChromaGo(b *testing.B) {
	b.ReportAllocs()
	code := string(benchmarkGoSnippet)
	lexer := DetectChromaLexerByFileName("bench.go", "Go")

	for b.Loop() {
		_ = renderCodeByChromaLexer(lexer, code)
	}
}

func BenchmarkRenderFullFileTreeSitterGo(b *testing.B) {
	b.ReportAllocs()
	code := benchmarkGoSnippet

	if _, _, err := renderFullFileByTreeSitter("bench.go", "Go", code); err != nil {
		b.Skipf("tree-sitter renderer is unavailable for Go: %v", err)
	}

	for b.Loop() {
		if _, _, err := renderFullFileByTreeSitter("bench.go", "Go", code); err != nil {
			b.Fatalf("tree-sitter renderer became unavailable for Go: %v", err)
		}
	}
}

func BenchmarkRenderFullFileTreeSitterGoCold(b *testing.B) {
	b.ReportAllocs()
	base := benchmarkGoSnippet

	if _, _, err := renderFullFileByTreeSitter("bench.go", "Go", base); err != nil {
		b.Skipf("tree-sitter renderer is unavailable for Go: %v", err)
	}

	codeA := make([]byte, 0, len(base)+32)
	codeA = append(codeA, base...)
	codeA = append(codeA, []byte("// cold-a\n")...)
	codeB := make([]byte, 0, len(base)+32)
	codeB = append(codeB, base...)
	codeB = append(codeB, []byte("// cold-b\n")...)

	for i := 0; i < b.N; i++ {
		buf := codeA
		if i&1 == 1 {
			buf = codeB
		}
		if _, _, err := renderFullFileByTreeSitter("bench.go", "Go", buf); err != nil {
			b.Fatalf("tree-sitter renderer became unavailable for Go: %v", err)
		}
	}
}

func BenchmarkRenderFullFileChromaGo(b *testing.B) {
	b.ReportAllocs()
	code := benchmarkGoSnippet

	for b.Loop() {
		_, _, err := renderFullFileByChroma("bench.go", "Go", code)
		if err != nil {
			b.Fatal(err)
		}
	}
}
