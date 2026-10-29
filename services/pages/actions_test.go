// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"testing"

	pages_model "gitea.dev/models/pages"
	"gitea.dev/modules/json"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsDeploymentRejectsIncompleteRequest(t *testing.T) {
	_, err := CommitActionsArtifact(t.Context(), 1, 1, ActionsDeploymentRequest{})
	require.ErrorIs(t, err, ErrActionsDeploymentRequest)

	_, err = CommitActionsArtifact(t.Context(), 1, 1, ActionsDeploymentRequest{
		ArtifactID: 1, PagesBuildVersion: "sha", OIDCToken: "token", Preview: true,
	})
	require.ErrorIs(t, err, ErrActionsPreview)
}

func TestActionsRestrictedBranch(t *testing.T) {
	site := &pages_model.Site{SourceBranch: "main"}
	assert.True(t, actionsRefAllowed(site, &actionsOIDCClaims{Ref: "refs/heads/main", RefType: "branch"}))
	assert.False(t, actionsRefAllowed(site, &actionsOIDCClaims{Ref: "refs/tags/main", RefType: "tag"}))
	assert.False(t, actionsRefAllowed(site, &actionsOIDCClaims{Ref: "refs/heads/other", RefType: "branch"}))
	site.SourceBranch = ""
	assert.True(t, actionsRefAllowed(site, &actionsOIDCClaims{Ref: "refs/tags/main", RefType: "tag"}))
}

func TestPagesDeploymentRequestJSONContract(t *testing.T) {
	request := api.PagesDeploymentOption{
		ArtifactID:        42,
		PagesBuildVersion: "commit-sha",
		OIDCToken:         "oidc-token",
		Preview:           true,
	}
	payload, err := json.Marshal(request)
	require.NoError(t, err)
	assert.JSONEq(t, `{"artifact_id":42,"pages_build_version":"commit-sha","oidc_token":"oidc-token","preview":true}`, string(payload))
}
