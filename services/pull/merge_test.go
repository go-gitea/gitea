// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func Test_expandDefaultMergeMessage(t *testing.T) {
	type args struct {
		template string
		vars     map[string]string
	}
	tests := []struct {
		name     string
		args     args
		want     string
		wantBody string
	}{
		{
			name: "single line",
			args: args{
				template: "Merge ${PullRequestTitle}",
				vars: map[string]string{
					"PullRequestTitle":       "PullRequestTitle",
					"PullRequestDescription": "Pull\nRequest\nDescription\n",
				},
			},
			want:     "Merge PullRequestTitle",
			wantBody: "",
		},
		{
			name: "multiple lines",
			args: args{
				template: "Merge ${PullRequestTitle}\nDescription:\n\n${PullRequestDescription}\n",
				vars: map[string]string{
					"PullRequestTitle":       "PullRequestTitle",
					"PullRequestDescription": "Pull\nRequest\nDescription\n",
				},
			},
			want:     "Merge PullRequestTitle",
			wantBody: "Description:\n\nPull\nRequest\nDescription\n",
		},
		{
			name: "leading newlines",
			args: args{
				template: "\n\n\nMerge ${PullRequestTitle}\n\n\nDescription:\n\n${PullRequestDescription}\n",
				vars: map[string]string{
					"PullRequestTitle":       "PullRequestTitle",
					"PullRequestDescription": "Pull\nRequest\nDescription\n",
				},
			},
			want:     "Merge PullRequestTitle",
			wantBody: "Description:\n\nPull\nRequest\nDescription\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := expandDefaultMergeMessage(tt.args.template, tt.args.vars)
			assert.Equalf(t, tt.want, got, "expandDefaultMergeMessage(%v, %v)", tt.args.template, tt.args.vars)
			assert.Equalf(t, tt.wantBody, got1, "expandDefaultMergeMessage(%v, %v)", tt.args.template, tt.args.vars)
		})
	}
}

func Test_IsUserAllowedToMerge(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: pr.BaseRepoID})

	perm, err := access.GetUserRepoPermission(db.DefaultContext, repo, user.NewActionsUser())
	assert.NoError(t, err)

	allowed, err := IsUserAllowedToMerge(db.DefaultContext, pr, perm, user.NewActionsUser())
	assert.NoError(t, err)
	assert.False(t, allowed)
}
