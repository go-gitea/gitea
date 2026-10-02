// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1

import (
	"net/http"
	"testing"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/routers/common"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyAuthWithOptionsMustChangePassword(t *testing.T) {
	for _, tc := range []struct {
		name               string
		userType           user_model.UserType
		mustChangePassword bool
		isActive           bool
		prohibitLogin      bool
		wantStatus         int
		wantMessage        string
	}{
		{name: "individual", userType: user_model.UserTypeIndividual, mustChangePassword: true, isActive: true, wantStatus: http.StatusForbidden, wantMessage: "You must change your password."},
		{name: "individual without password change", userType: user_model.UserTypeIndividual, isActive: true, wantStatus: http.StatusOK},
		{name: "bot", userType: user_model.UserTypeBot, mustChangePassword: true, isActive: true, wantStatus: http.StatusOK},
		{name: "bot without password change", userType: user_model.UserTypeBot, isActive: true, wantStatus: http.StatusOK},
		{name: "inactive bot", userType: user_model.UserTypeBot, mustChangePassword: true, wantStatus: http.StatusForbidden},
		{name: "prohibited bot", userType: user_model.UserTypeBot, mustChangePassword: true, isActive: true, prohibitLogin: true, wantStatus: http.StatusForbidden, wantMessage: "This account is prohibited from signing in"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, resp := contexttest.MockAPIContext(t, "/api/v1/user")
			ctx.Doer = &user_model.User{
				Type:               tc.userType,
				MustChangePassword: tc.mustChangePassword,
				IsActive:           tc.isActive,
				ProhibitLogin:      tc.prohibitLogin,
			}
			ctx.IsSigned = true

			verifyAuthWithOptions(&common.VerifyOptions{SignInRequired: true})(ctx)

			assert.Equal(t, tc.wantStatus, resp.Code)
			assert.Equal(t, tc.wantStatus != http.StatusOK, ctx.Written())
			if tc.wantStatus == http.StatusOK {
				assert.Empty(t, resp.Body.String())
			} else if tc.wantMessage != "" {
				assert.Contains(t, resp.Body.String(), tc.wantMessage)
			}
		})
	}
}

func TestDoerNeedTwoFactorAuth(t *testing.T) {
	defer test.MockVariableValue(&setting.TwoFactorAuthEnforced, true)()

	for _, doer := range []*user_model.User{nil, user_model.NewActionsUser(), user_model.NewDeployKeyUser()} {
		need, err := doerNeedTwoFactorAuth(t.Context(), doer)
		require.NoError(t, err)
		assert.False(t, need)
	}
}
