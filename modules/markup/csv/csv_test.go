// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/markup"

	"github.com/stretchr/testify/assert"
)

func TestRenderCSV(t *testing.T) {
	var render Renderer
	kases := map[string]string{
		"a":        "<table class=\"data-table\"><tr><th class=\"line-num\">1</th><th>a</th></tr></table>",
		"1,2":      "<table class=\"data-table\"><tr><th class=\"line-num\">1</th><th>1</th><th>2</th></tr></table>",
		"1;2\n3;4": "<table class=\"data-table\"><tr><th class=\"line-num\">1</th><th>1</th><th>2</th></tr><tr><td class=\"line-num\">2</td><td>3</td><td>4</td></tr></table>",
		"<br/>":    "<table class=\"data-table\"><tr><th class=\"line-num\">1</th><th>&lt;br/&gt;</th></tr></table>",
	}

	for k, v := range kases {
		var buf strings.Builder
		err := render.Render(markup.NewRenderContext(t.Context()), strings.NewReader(k), &buf)
		assert.NoError(t, err)
		assert.EqualValues(t, v, buf.String())
	}
}
