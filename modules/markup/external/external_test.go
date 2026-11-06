// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package external

import (
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestPrepareExternalCommand(t *testing.T) {
	t.Run("NoCommand", func(t *testing.T) {
		r := &Renderer{MarkupRenderer: &setting.MarkupRenderer{Command: ""}}
		_, _, err := r.prepareExternalCommand(map[string]string{"KEY": "val"})
		assert.ErrorContains(t, err, "no command")
	})
	t.Run("ReplaceArgs", func(t *testing.T) {
		r := &Renderer{MarkupRenderer: &setting.MarkupRenderer{Command: `"/foo bar/bin" --opt $KEY "$KEY" %KEY% other`}}
		prog, args, err := r.prepareExternalCommand(map[string]string{"KEY": `a"b`})
		assert.NoError(t, err)
		assert.Equal(t, "/foo bar/bin", prog)
		assert.Equal(t, []string{"--opt", `a"b`, `a"b`, `a"b`, "other"}, args)
	})
}

func TestPrepare(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		r := &Renderer{MarkupRenderer: &setting.MarkupRenderer{Command: "foo $GITEA_PREFIX_SRC $GITEA_PREFIX_RAW"}}
		ret, err := r.prepare("https://example.com/src", "https://example.com/raw")
		assert.NoError(t, err)
		assert.Equal(t, []string{"https://example.com/src", "https://example.com/raw"}, ret.args)
	})
	t.Run("Sanitized", func(t *testing.T) {
		r := &Renderer{MarkupRenderer: &setting.MarkupRenderer{Command: "foo $GITEA_PREFIX_SRC $GITEA_PREFIX_RAW"}}
		ret, err := r.prepare("https://[::1%25foo]:3000/a%2fb/x$y", "https://$foo.com")
		assert.NoError(t, err)
		assert.Equal(t, []string{"https://[::1%25foo]:3000/a%2fb/x%24y", ""}, ret.args)
	})
}
