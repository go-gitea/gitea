// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func withForcedNilTreeSitterRenderer(t *testing.T, fileName, fileLang string, fn func()) {
	t.Helper()

	entry := resolveTreeSitterEntry(fileName, fileLang)
	if entry == nil {
		t.Skipf("tree-sitter entry unavailable for %s (%s)", fileName, fileLang)
	}

	original, hadOriginal := treeSitterRendererCache.Load(entry.Name)
	treeSitterRendererCache.Store(entry.Name, (*treeSitterRenderer)(nil))
	defer func() {
		if hadOriginal {
			treeSitterRendererCache.Store(entry.Name, original)
			return
		}
		treeSitterRendererCache.Delete(entry.Name)
	}()

	fn()
}

func TestRenderCodeFallsBackToChromaWhenTreeSitterUnavailable(t *testing.T) {
	fileName := "bench.go"
	fileLang := "Go"
	code := "package main\nfunc main() { println(1) }\n"

	want := renderCodeByChromaLexer(DetectChromaLexerByFileName(fileName, fileLang), code)

	withForcedNilTreeSitterRenderer(t, fileName, fileLang, func() {
		got := RenderCode(fileName, fileLang, code)
		assert.Equal(t, want, got)
	})
}

func TestRenderCodeFallsBackToChromaWhenTreeSitterResultIsUnusable(t *testing.T) {
	testCases := []struct {
		name     string
		fileName string
		code     string
	}{
		{
			name:     "swift malformed snippet",
			fileName: "sample.swift",
			code:     "func {\n",
		},
		{
			name:     "nginx malformed snippet",
			fileName: "nginx.conf",
			code:     "server { return 200\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			want := renderCodeByChromaLexer(DetectChromaLexerByFileName(tc.fileName, ""), tc.code)
			got := RenderCode(tc.fileName, "", tc.code)
			assert.Equal(t, want, got)
		})
	}
}

func TestRenderCodeUsesTreeSitterForParitySamples(t *testing.T) {
	testCases := []struct {
		name     string
		fileName string
		code     string
	}{
		{
			name:     "haskell bare function",
			fileName: "sample.hs",
			code:     "f x = x + 1\n",
		},
		{
			name:     "nginx single-line server block",
			fileName: "nginx.conf",
			code:     "server { listen 80; location / { return 200; } }\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tryRenderCodeByTreeSitter(tc.fileName, "", []byte(tc.code))
			assert.True(t, ok)
			rendered := string(got)
			switch tc.name {
			case "haskell bare function":
				assert.Contains(t, rendered, `<span class="kt">f</span>`)
				assert.Contains(t, rendered, `<span class="kt">x</span>`)
				assert.Contains(t, rendered, `<span class="m">1</span>`)
			case "nginx single-line server block":
				assert.Contains(t, rendered, `<span class="k">server</span>`)
				assert.Contains(t, rendered, `<span class="k">listen</span>`)
				assert.Contains(t, rendered, `<span class="ss">/</span>`)
				assert.Contains(t, rendered, `<span class="p">}</span>`)
			}
		})
	}
}

func TestRenderCodeByLexerFallsBackToChromaWhenTreeSitterUnavailable(t *testing.T) {
	fileName := "bench.go"
	fileLang := "Go"
	code := "package main\nfunc main() { println(1) }\n"
	lexer := DetectChromaLexerByFileName(fileName, fileLang)

	want := renderCodeByChromaLexer(lexer, code)

	withForcedNilTreeSitterRenderer(t, fileName, fileLang, func() {
		got := RenderCodeByLexer(lexer, code)
		assert.Equal(t, want, got)
	})
}

func TestRenderFullFileFallsBackToChromaWhenTreeSitterUnavailable(t *testing.T) {
	fileName := "bench.go"
	fileLang := "Go"
	code := []byte("package main\nfunc main() { println(1) }\n")

	wantLines, wantLexer, err := renderFullFileByChroma(fileName, fileLang, code)
	assert.NoError(t, err)

	withForcedNilTreeSitterRenderer(t, fileName, fileLang, func() {
		gotLines, gotLexer, err := RenderFullFile(fileName, fileLang, code)
		assert.NoError(t, err)
		assert.Equal(t, wantLines, gotLines)
		assert.Equal(t, wantLexer, gotLexer)
	})
}

func TestRenderFullFileByTreeSitterHighlightsMarkdownFenceInjection(t *testing.T) {
	code := []byte("```golang\npackage main\nfunc main() {}\n```\n")

	lines, lexerName, err := renderFullFileByTreeSitter("README.md", "Markdown", code)
	assert.NoError(t, err)
	assert.Equal(t, "Markdown", lexerName)

	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		rendered = append(rendered, string(line))
	}
	out := strings.Join(rendered, "")
	assert.Contains(t, out, `<span class="k">package</span>`)
	assert.Contains(t, out, `<span class="k">func</span>`)
}
