// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestCompareHeadRef(t *testing.T) {
	defer test.MockVariableValue(&setting.Repository.AllowForkIntoSameOwner, false)()
	baseRepo := &repo_model.Repository{ID: 1, OwnerID: 100, OwnerName: "base-owner", Name: "base-repo"}
	sameRepo := baseRepo
	sameOwner := &repo_model.Repository{ID: 2, OwnerID: 100, OwnerName: "head-owner", Name: "head-repo"}
	diffOwner := &repo_model.Repository{ID: 2, OwnerID: 101, OwnerName: "head-owner", Name: "head-repo"}

	assert.Equal(t, "my-branch", CompareHeadRef(baseRepo, sameRepo, "my-branch"))
	assert.Equal(t, "head-owner/head-repo:my-branch", CompareHeadRef(baseRepo, sameOwner, "my-branch"))
	assert.Equal(t, "head-owner:my-branch", CompareHeadRef(baseRepo, diffOwner, "my-branch"))
	setting.Repository.AllowForkIntoSameOwner = true
	assert.Equal(t, "head-owner/head-repo:my-branch", CompareHeadRef(baseRepo, diffOwner, "my-branch"))
}

func TestCannotCommitReasons(t *testing.T) {
	tests := []struct {
		name                    string
		submitToForkedRepo      bool
		isMirror                bool
		canPushWithProtection   bool
		protectionRequireSigned bool
		willSign                bool
		canCommitToBranch       bool
		want                    []string
	}{
		{
			name:                  "fork without upstream write access",
			submitToForkedRepo:    true,
			canPushWithProtection: true,
			canCommitToBranch:     false,
			want:                  []string{"repo.editor.no_write_access_to_upstream_branch"},
		},
		{
			name:                  "protected branch without push",
			canPushWithProtection: false,
			canCommitToBranch:     false,
			want:                  []string{"repo.editor.user_no_push_to_branch"},
		},
		{
			name:                    "protected branch requires signed commit",
			canPushWithProtection:   true,
			protectionRequireSigned: true,
			willSign:                false,
			canCommitToBranch:       false,
			want:                    []string{"repo.editor.require_signed_commit"},
		},
		{
			name:              "direct commit allowed",
			canCommitToBranch: true,
			want:              nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &repo_model.Repository{IsMirror: tt.isMirror}
			got := cannotCommitReasons(tt.submitToForkedRepo, repo, tt.canPushWithProtection, tt.protectionRequireSigned, tt.willSign, tt.canCommitToBranch)
			assert.Equal(t, tt.want, got)
		})
	}
}
