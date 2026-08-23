// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package external

import (
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestPrepareExternalCommand(t *testing.T) {
	r := &Renderer{MarkupRenderer: &setting.MarkupRenderer{Command: ""}}
	_, _, err := r.prepareExternalCommand(map[string]string{"KEY": "val"})
	assert.ErrorContains(t, err, "no command")

	r = &Renderer{MarkupRenderer: &setting.MarkupRenderer{Command: `"/foo bar/bin" --opt $KEY "$KEY" %KEY% other`}}
	prog, args, err := r.prepareExternalCommand(map[string]string{"KEY": `a"b`})
	assert.NoError(t, err)
	assert.Equal(t, "/foo bar/bin", prog)
	assert.Equal(t, []string{"--opt", `a"b`, `a"b`, `a"b`, "other"}, args)
}
