// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/models/user"
	gitea_context "gitea.dev/services/context"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestRenderHelperMention(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	userPublic := "user1"
	userPrivate := "user31"
	userLimited := "user33"
	userNoSuch := "no-such-user"

	unittest.AssertCount(t, &user.User{Name: userPublic}, 1)
	unittest.AssertCount(t, &user.User{Name: userPrivate}, 1)
	unittest.AssertCount(t, &user.User{Name: userLimited}, 1)
	unittest.AssertCount(t, &user.User{Name: userNoSuch}, 0)

	mentionable := func(ctx context.Context, username string) bool {
		ok, _ := FormalRenderHelperFuncs().ResolveUserMention(ctx, username)
		return ok
	}

	// when using general context, use user's visibility to check
	assert.True(t, mentionable(t.Context(), userPublic))
	assert.False(t, mentionable(t.Context(), userLimited))
	assert.False(t, mentionable(t.Context(), userPrivate))
	assert.False(t, mentionable(t.Context(), userNoSuch))

	ok, avatarLink := FormalRenderHelperFuncs().ResolveUserMention(t.Context(), userPublic)
	assert.True(t, ok)
	assert.NotEmpty(t, avatarLink)

	// when using web context, use user.IsUserVisibleToViewer to check
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	assert.NoError(t, err)
	base := gitea_context.NewBaseContextForTest(t, httptest.NewRecorder(), req)
	giteaCtx := gitea_context.NewWebContext(base, &contexttest.MockRender{}, nil)

	assert.True(t, mentionable(giteaCtx, userPublic))
	assert.False(t, mentionable(giteaCtx, userPrivate))

	giteaCtx.Doer, err = user.GetUserByName(t.Context(), userPrivate)
	assert.NoError(t, err)
	assert.True(t, mentionable(giteaCtx, userPublic))
	assert.True(t, mentionable(giteaCtx, userPrivate))
}
