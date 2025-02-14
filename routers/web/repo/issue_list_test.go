// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/contexttest"
	"code.gitea.io/gitea/services/mailer/token"

	"github.com/stretchr/testify/assert"
)

func TestRenderMailToIssue(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx, _ := contexttest.MockContext(t, "user2/repo1")

	ctx.IsSigned = true
	ctx.Doer = unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	ctx.Repo = &context.Repository{
		Repository: unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}),
	}

	setting.IncomingEmail.Enabled = true
	setting.IncomingEmail.ReplyToAddress = "test%{token}@gitea.io"
	setting.IncomingEmail.TokenPlaceholder = "%{token}"

	err := renderMailToIssue(ctx)
	assert.NoError(t, err)

	key, ok := ctx.Data["MailToIssueToken"].(string)
	assert.True(t, ok)

	handlerType, user, _, err := token.ExtractToken(ctx, key)
	assert.NoError(t, err)
	assert.EqualValues(t, token.NewIssueHandlerType, handlerType)
	assert.EqualValues(t, ctx.Doer.ID, user.ID)
}
