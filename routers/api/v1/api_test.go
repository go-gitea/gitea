// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1

import (
	"net/http"
	"testing"

	codespace_model "gitea.dev/models/codespace"
	user_model "gitea.dev/models/user"
	"gitea.dev/routers/common"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestSudoRejectsCodespaceToken(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		set  func(header http.Header)
	}{
		{
			name: "query",
			path: "GET /api/v1/user?sudo=user2",
		},
		{
			name: "header",
			path: "GET /api/v1/user",
			set: func(header http.Header) {
				header.Set("Sudo", "user2")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := contexttest.MockAPIContext(t, tc.path)
			if tc.set != nil {
				tc.set(ctx.Req.Header)
			}
			ctx.IsSigned = true
			ctx.Doer = &user_model.User{ID: 1, Name: "admin", IsAdmin: true}
			ctx.GetData()[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 1}

			sudo()(ctx)

			assert.Equal(t, http.StatusForbidden, ctx.Resp.WrittenStatus())
			assert.Equal(t, int64(1), ctx.Doer.ID)
		})
	}
}

func TestCodespaceTokenRouteGuard(t *testing.T) {
	for _, tc := range []struct {
		name       string
		policy     string
		wantStatus int
	}{
		{
			name:   "self",
			policy: codespace_model.GiteaTokenRoutePolicySelf,
		},
		{
			name:   "public-info",
			policy: codespace_model.GiteaTokenRoutePolicyPublicInfo,
		},
		{
			name:   "repository-group",
			policy: codespace_model.GiteaTokenRoutePolicyRepositoryGroup,
		},
		{
			name:   "signed-artifact",
			policy: codespace_model.GiteaTokenRoutePolicySignedArtifact,
		},
		{
			name:       "unmarked-route",
			wantStatus: http.StatusForbidden,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := contexttest.MockAPIContext(t, "GET /api/v1/repos/user2/repo1")
			ctx.GetData()[codespace_model.GiteaTokenAuthDataKey] = testCodespaceTokenSnapshot{repoID: 1}
			if tc.policy != "" {
				ctx.GetData()[codespace_model.GiteaTokenRoutePolicyDataKey] = tc.policy
			}

			codespaceTokenRouteGuard(ctx)

			assert.Equal(t, tc.wantStatus, ctx.Resp.WrittenStatus())
		})
	}
}

func TestSignedArtifactRouteSkipsRegularAPIAuth(t *testing.T) {
	ctx, _ := contexttest.MockAPIContext(t, "GET /api/v1/repos/user2/repo1/actions/artifacts/1/zip/raw")
	ctx.Req.Header.Set("Authorization", "Bearer gcs_invalid")
	ctx.GetData()[codespace_model.GiteaTokenRoutePolicyDataKey] = codespace_model.GiteaTokenRoutePolicySignedArtifact

	apiAuth(buildAuthGroup())(ctx)
	assert.Equal(t, 0, ctx.Resp.WrittenStatus())
	assert.Nil(t, ctx.Doer)

	verifyAuthWithOptions(&common.VerifyOptions{SignInRequired: true})(ctx)
	assert.Equal(t, 0, ctx.Resp.WrittenStatus())
}

type testCodespaceTokenSnapshot struct {
	repoID int64
}

func (s testCodespaceTokenSnapshot) CodespaceTokenRepoID() int64 {
	return s.repoID
}
