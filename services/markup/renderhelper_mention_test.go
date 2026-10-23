// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"codenest.dev/models/unittest"
	"codenest.dev/models/user"
	codenest_context "codenest.dev/services/context"
	"codenest.dev/services/contexttest"

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

	// when using general context, use user's visibility to check
	assert.True(t, FormalRenderHelperFuncs().IsUsernameMentionable(t.Context(), userPublic))
	assert.False(t, FormalRenderHelperFuncs().IsUsernameMentionable(t.Context(), userLimited))
	assert.False(t, FormalRenderHelperFuncs().IsUsernameMentionable(t.Context(), userPrivate))
	assert.False(t, FormalRenderHelperFuncs().IsUsernameMentionable(t.Context(), userNoSuch))

	// when using web context, use user.IsUserVisibleToViewer to check
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	assert.NoError(t, err)
	base := codenest_context.NewBaseContextForTest(t, httptest.NewRecorder(), req)
	codenestCtx := codenest_context.NewWebContext(base, &contexttest.MockRender{}, nil)

	assert.True(t, FormalRenderHelperFuncs().IsUsernameMentionable(codenestCtx, userPublic))
	assert.False(t, FormalRenderHelperFuncs().IsUsernameMentionable(codenestCtx, userPrivate))

	codenestCtx.Doer, err = user.GetUserByName(t.Context(), userPrivate)
	assert.NoError(t, err)
	assert.True(t, FormalRenderHelperFuncs().IsUsernameMentionable(codenestCtx, userPublic))
	assert.True(t, FormalRenderHelperFuncs().IsUsernameMentionable(codenestCtx, userPrivate))
}
