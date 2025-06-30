// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package console

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/typesniffer"

	"github.com/stretchr/testify/assert"
)

func TestRenderConsole(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"\x1b[37m\x1b[40mnpm\x1b[0m \x1b[0m\x1b[32minfo\x1b[0m \x1b[0m\x1b[35mit worked if it ends with\x1b[0m ok", `<span class="term-fg37 term-bg40">npm</span> <span class="term-fg32">info</span> <span class="term-fg35">it worked if it ends with</span> ok`},
		{"\x1b[1;2m \x1b[123m 啊", `<span class="term-fg2">  啊</span>`},
		{"\x1b[1;2m \x1b[123m \xef", `<span class="term-fg2">  �</span>`},
		{"\x1b[1;2m \x1b[123m \xef \xef", ``},
		{"\x1b[12", ``},
		{"\x1b[1", ``},
		{"\x1b[FOO\x1b[", ``},
		{"\x1b[mFOO\x1b[m", `FOO`},
	}

	var render Renderer
	for i, c := range cases {
		var buf strings.Builder
		st := typesniffer.DetectContentType([]byte(c.input))
		canRender := render.CanRender("test", st, []byte(c.input))
		if c.expected == "" {
			assert.False(t, canRender, "case %d: expected not to render", i)
			continue
		}

		assert.True(t, canRender)
		err := render.Render(markup.NewRenderContext(t.Context()), strings.NewReader(c.input), &buf)
		assert.NoError(t, err)
		assert.Equal(t, c.expected, buf.String())
	}
}
