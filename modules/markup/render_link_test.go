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
	assert.Equal(t, "/a/c#id", resolveLinkRelative(ctx, "/a", "b", "/c#id", false))
	assert.Equal(t, "/a/%2f?k=/", resolveLinkRelative(ctx, "/a", "b", "/%2f/?k=/", false))
	assert.Equal(t, "/a/b/c?k=v#id", resolveLinkRelative(ctx, "/a", "b", "c/?k=v#id", false))
	assert.Equal(t, "%invalid", resolveLinkRelative(ctx, "/a", "b", "%invalid", false))
	assert.Equal(t, "http://localhost:3000/a", resolveLinkRelative(ctx, "/a", "", "", true))

	// absolute link is returned as is
	assert.Equal(t, "mailto:user@domain.com", resolveLinkRelative(ctx, "/a", "", "mailto:user@domain.com", false))
	assert.Equal(t, "http://other/path/", resolveLinkRelative(ctx, "/a", "", "http://other/path/", false))

	// some users might have used absolute paths a lot, so if the prefix overlaps and has enough slashes, we should tolerate it
	assert.Equal(t, "/owner/repo/foo/owner/repo/foo/bar/xxx", resolveLinkRelative(ctx, "/owner/repo/foo", "", "/owner/repo/foo/bar/xxx", false))
	assert.Equal(t, "/owner/repo/foo/bar/xxx", resolveLinkRelative(ctx, "/owner/repo/foo/bar", "", "/owner/repo/foo/bar/xxx", false))
}
