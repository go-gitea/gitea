// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"testing"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/session"
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
		impersonated       bool
		wantWritten        bool
		wantStatus         int
	}{
		{name: "individual", mustChangePassword: true, isActive: true, wantWritten: true, wantStatus: http.StatusUnauthorized},
		{name: "individual without password change", isActive: true, wantStatus: http.StatusOK},
		{name: "impersonated individual", mustChangePassword: true, isActive: true, impersonated: true, wantStatus: http.StatusOK},
		{name: "bot", userType: user_model.UserTypeBot, mustChangePassword: true, isActive: true, wantStatus: http.StatusOK},
		{name: "bot without password change", userType: user_model.UserTypeBot, isActive: true, wantStatus: http.StatusOK},
		{name: "inactive bot", userType: user_model.UserTypeBot, mustChangePassword: true, wantWritten: true, wantStatus: http.StatusOK},
		{name: "prohibited bot", userType: user_model.UserTypeBot, mustChangePassword: true, isActive: true, prohibitLogin: true, wantWritten: true, wantStatus: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := session.NewMockMemStore("password-change")
			if tc.impersonated {
				require.NoError(t, store.Set(session.KeyImpersonatorData, "admin"))
			}
			ctx, resp := contexttest.MockContext(t, "/bot/repo.git/info/refs?service=git-upload-pack", contexttest.MockContextOption{SessionStore: store})
			ctx.Req.Header.Set("User-Agent", "git/2.50.0")
			ctx.Doer = &user_model.User{
				Type:               tc.userType,
				MustChangePassword: tc.mustChangePassword,
				IsActive:           tc.isActive,
				ProhibitLogin:      tc.prohibitLogin,
			}
			ctx.IsSigned = true

			verifyAuthWithOptions(&common.VerifyOptions{SignInRequired: true})(ctx)

			assert.Equal(t, tc.wantStatus, resp.Code)
			assert.Equal(t, tc.wantWritten, ctx.Written())
			if !tc.wantWritten {
				assert.Empty(t, resp.Body.String())
			}
		})
	}
}
