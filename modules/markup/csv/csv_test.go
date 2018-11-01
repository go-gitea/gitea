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
		"a":     "<table class=\"table\"><tr><td>a</td><tr></table>",
		"1,2":   "<table class=\"table\"><tr><td>1</td><td>2</td><tr></table>",
		"<br/>": "<table class=\"table\"><tr><td>&lt;br/&gt;</td><tr></table>",
	}

	for k, v := range kases {
		res := parser.Render([]byte(k), "", nil, false)
		assert.EqualValues(t, v, string(res))
	}
}
