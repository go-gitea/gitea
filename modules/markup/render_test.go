// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderIFrame(t *testing.T) {
	render := func(ctx *RenderContext, opts ExternalRendererOptions) string {
		sb := &strings.Builder{}
		require.NoError(t, RenderIFrame(ctx, &opts, sb))
		return sb.String()
	}

	ctx := NewRenderContext(t.Context()).
		WithRelativePath("tree-path").
		WithMetas(map[string]string{"user": "test-owner", "repo": "test-repo", "RefTypeNameSubURL": "src/branch/master"})

	// iframe doesn't need sandbox, the sandbox is set in render's response header
	ret := render(ctx, ExternalRendererOptions{ContentSandbox: "any"})
	assert.Equal(t, `<iframe data-src="/test-owner/test-repo/render/src/branch/master/tree-path" data-global-init="initExternalRenderIframe" class="external-render-iframe"></iframe>`, ret)
}
