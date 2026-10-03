// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/session"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestCheckSignedInUser(t *testing.T) {
	defer test.MockVariableValue(&setting.Service.RegisterEmailConfirm)()
	sessNormal := session.NewMockMemStore("session-a")
	sessImpersonated := session.NewMockMemStore("session-b")
	_ = sessImpersonated.Set(session.KeyImpersonatorData, "any-value")

	setting.Service.RegisterEmailConfirm = false
	ret := CheckSignedInUser(&user_model.User{IsActive: false}, nil)
	assert.False(t, ret.NeedActivateAccount)
	assert.True(t, ret.LoginIsProhibited)

	setting.Service.RegisterEmailConfirm = true
	ret = CheckSignedInUser(&user_model.User{IsActive: false}, nil)
	assert.True(t, ret.NeedActivateAccount)
	assert.True(t, ret.LoginIsProhibited)

	ret = CheckSignedInUser(&user_model.User{IsActive: true}, nil)
	assert.False(t, ret.NeedActivateAccount)
	assert.False(t, ret.LoginIsProhibited)

	ret = CheckSignedInUser(&user_model.User{IsActive: true, ProhibitLogin: true}, nil)
	assert.False(t, ret.NeedActivateAccount)
	assert.True(t, ret.LoginIsProhibited)

	ret = CheckSignedInUser(&user_model.User{MustChangePassword: false}, nil)
	assert.False(t, ret.NeedChangePassword)

	ret = CheckSignedInUser(&user_model.User{MustChangePassword: true}, nil)
	assert.True(t, ret.NeedChangePassword)

	ret = CheckSignedInUser(&user_model.User{MustChangePassword: true}, sessNormal)
	assert.True(t, ret.NeedChangePassword)

	ret = CheckSignedInUser(&user_model.User{MustChangePassword: true, Type: user_model.UserTypeBot}, sessNormal)
	assert.False(t, ret.NeedChangePassword)

	ret = CheckSignedInUser(&user_model.User{MustChangePassword: true}, sessImpersonated)
	assert.False(t, ret.NeedChangePassword)
}
