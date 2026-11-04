// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"testing"

	git_model "gitea.dev/models/git"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/web"
	"gitea.dev/services/contexttest"
	"gitea.dev/services/forms"

	"github.com/stretchr/testify/assert"
)

func TestSettingsProtectedBranchPostBindingError(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx, _ := contexttest.MockContext(t, "POST /user2/repo1/settings/branches/edit")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 1)
	web.SetForm(ctx, &forms.ProtectBranchForm{RuleName: "main"})
	ctx.Data["HasError"] = true
	ctx.Data["ErrorMsg"] = "RequiredApprovals is invalid"

	SettingsProtectedBranchPost(ctx)

	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	assert.Equal(t, "RequiredApprovals is invalid", ctx.Flash.ErrorMsg)
	unittest.AssertNotExistsBean(t, &git_model.ProtectedBranch{RepoID: 1, RuleName: "main"})
}
