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
	_, _, err := r.prepareExternalCommand()
	assert.ErrorContains(t, err, "no command")

	r = &Renderer{MarkupRenderer: &setting.MarkupRenderer{Command: `"/foo bar/bin" --opt $GITEA_PREFIX_SRC other`}}
	prog, args, err := r.prepareExternalCommand()
	assert.NoError(t, err)
	assert.Equal(t, "/foo bar/bin", prog)
	assert.Equal(t, []string{"--opt", "$GITEA_PREFIX_SRC", "other"}, args)
}
