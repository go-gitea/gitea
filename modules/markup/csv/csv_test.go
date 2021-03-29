// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderCSV(t *testing.T) {
	var parser Parser
	var kases = map[string]string{
		"a":        "<table class=\"data-table\"><tr><th class=\"line-num\">1</th><th>a</th></tr></table>",
		"1,2":      "<table class=\"data-table\"><tr><th class=\"line-num\">1</th><th>1</th><th>2</th></tr></table>",
		"1;2\n3;4": "<table class=\"data-table\"><tr><th class=\"line-num\">1</th><th>1</th><th>2</th></tr><tr><td class=\"line-num\">2</td><td>3</td><td>4</td></tr></table>",
		"<br/>":    "<table class=\"data-table\"><tr><th class=\"line-num\">1</th><th>&lt;br/&gt;</th></tr></table>",
	}

	for k, v := range kases {
		res := parser.Render([]byte(k), "", nil, false)
		assert.EqualValues(t, v, string(res))
	}
}
