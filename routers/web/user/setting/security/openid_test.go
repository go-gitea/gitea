// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package security

import (
	"net/http"
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestDeleteOpenIDReturnsNotFoundForOtherUsersAddress(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "POST /user/settings/security")
	contexttest.LoadUser(t, ctx, 2)
	ctx.SetFormString("id", "1")

	DeleteOpenID(ctx)

	assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())
}

func TestToggleOpenIDVisibilityReturnsNotFoundForOtherUsersAddress(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "POST /user/settings/security")
	contexttest.LoadUser(t, ctx, 2)
	ctx.SetFormString("id", "1")

	ToggleOpenIDVisibility(ctx)

	assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())
}
