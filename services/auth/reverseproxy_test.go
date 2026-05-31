// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/reqctx"
	session_module "gitea.dev/modules/session"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReverseProxyVerifyUpdatesLastLogin(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user.LastLoginUnix = timeutil.TimeStamp(0)
	require.NoError(t, user_model.UpdateUserCols(t.Context(), user, "last_login_unix"))

	sess := session_module.NewMockMemStore("reverse-proxy-last-login")
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(
		context.WithValue(t.Context(), session_module.MockStoreContextKey, sess),
	)
	req.Header.Set(setting.ReverseProxyAuthUser, user.Name)

	verifiedUser, err := (&ReverseProxy{CreateSession: true}).Verify(req, httptest.NewRecorder(), reqctx.ContextData{}, sess)
	require.NoError(t, err)
	require.NotNil(t, verifiedUser)

	updatedUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: user.ID})
	assert.NotZero(t, updatedUser.LastLoginUnix)
}
