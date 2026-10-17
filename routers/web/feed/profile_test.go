// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package feed_test

import (
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/routers/web/feed"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestCheckGetOrgFeedsAsOrgMember(t *testing.T) {
	unittest.PrepareTestEnv(t)
	t.Run("OrgMember", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "org3.atom")
		ctx.ContextUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
		contexttest.LoadUser(t, ctx, 2)
		feed.ShowUserFeedAtom(ctx)
		assert.Contains(t, resp.Body.String(), "<entry>") // Should contain 1 private entry
	})
	t.Run("NonOrgMember", func(t *testing.T) {
		ctx, resp := contexttest.MockContext(t, "org3.atom")
		ctx.ContextUser = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
		contexttest.LoadUser(t, ctx, 5)
		feed.ShowUserFeedAtom(ctx)
		assert.NotContains(t, resp.Body.String(), "<entry>") // Should not contain any entries
	})
}
