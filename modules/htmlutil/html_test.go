// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package htmlutil

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testStringer struct{}

func (t testStringer) String() string {
	return "&StringMethod"
}

func TestHTMLFormat(t *testing.T) {
	assert.Equal(t, template.HTML("<a>&lt; < 1</a>"), HTMLFormat("<a>%s %s %d</a>", "<", template.HTML("<"), 1))
	assert.Equal(t, template.HTML("%!s(<nil>)"), HTMLFormat("%s", nil))
	assert.Equal(t, template.HTML("&lt;&gt;"), HTMLFormat("%s", template.URL("<>")))
	assert.Equal(t, template.HTML("&amp;StringMethod &amp;StringMethod"), HTMLFormat("%s %s", testStringer{}, &testStringer{}))
}
