// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectLanguage(t *testing.T) {
	assert.Equal(t, "Go", DetectLanguage("main.go", "", nil))
	assert.Equal(t, "Python", DetectLanguage("main.go", "Python?foo=bar", nil))
	assert.Equal(t, "Shell", DetectLanguage("run", "", []byte("#!/bin/sh\necho\n")))
	assert.Empty(t, DetectLanguage("unknown", "", nil))
}

func TestRenderPlainLines(t *testing.T) {
	assert.Equal(t, []template.HTML{"a&lt;b\r\n", "\n", "c"}, RenderPlainLines("a<b\r\n\nc"))
	assert.Empty(t, RenderPlainLines(""))
}

func TestCodeBlockAttributes(t *testing.T) {
	test := func(lang string, pre, code template.HTML) {
		t.Helper()
		preActual, codeActual := CodeBlockAttributes(lang)
		assert.Equal(t, pre, preActual)
		assert.Equal(t, code, codeActual)
	}
	test("", `class="code-block"`, `class="code-highlight language-text" data-code-language="text"`)
	test("math", `class="code-block is-loading"`, `class="code-highlight language-math" data-code-language="math"`)
	test("Visual Basic.NET", `class="code-block"`, `class="code-highlight language-visual_basic_net" data-code-language="Visual Basic.NET"`)
	test("c++-x", `class="code-block"`, `class="code-highlight language-c___x" data-code-language="c++-x"`)
}
