// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestResolveLinkRelative(t *testing.T) {
	ctx := t.Context()
	setting.AppURL = "http://localhost:3000"
	assert.Equal(t, "/a", resolveLinkRelative(ctx, "/a", "", "", false))
	assert.Equal(t, "/a/b", resolveLinkRelative(ctx, "/a", "b", "", false))
	assert.Equal(t, "/a/b/c", resolveLinkRelative(ctx, "/a", "b", "c", false))
	assert.Equal(t, "/a/c", resolveLinkRelative(ctx, "/a", "b", "/c", false))
	assert.Equal(t, "http://localhost:3000/a", resolveLinkRelative(ctx, "/a", "", "", true))

	// some users might have used absolute paths a lot, so if the prefix overlaps and has enough slashes, we should tolerate it
	assert.Equal(t, "/owner/repo/foo/owner/repo/foo/bar/xxx", resolveLinkRelative(ctx, "/owner/repo/foo", "", "/owner/repo/foo/bar/xxx", false))
	assert.Equal(t, "/owner/repo/foo/bar/xxx", resolveLinkRelative(ctx, "/owner/repo/foo/bar", "", "/owner/repo/foo/bar/xxx", false))
}
