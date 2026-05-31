// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	git_model "gitea.dev/models/git"
	"gitea.dev/modules/commitstatus"

	"github.com/stretchr/testify/assert"
)

func TestPullCommitStatusCheckDataFaviconVariant(t *testing.T) {
	tests := []struct {
		name string
		data *pullMergeBoxData
		want string
	}{
		{
			name: "hidden without visible status checks",
			data: &pullMergeBoxData{
				ShowStatusCheck: false,
				StatusCheckData: &pullCommitStatusCheckData{
					pullCommitStatusState: commitstatus.CommitStatusSuccess,
				},
			},
			want: "",
		},
		{
			name: "success",
			data: &pullMergeBoxData{
				ShowStatusCheck: true,
				StatusCheckData: &pullCommitStatusCheckData{
					pullCommitStatusState: commitstatus.CommitStatusSuccess,
					PullCommitStatuses: []*git_model.CommitStatus{
						{State: commitstatus.CommitStatusSuccess},
					},
				},
			},
			want: "success",
		},
		{
			name: "pending",
			data: &pullMergeBoxData{
				ShowStatusCheck: true,
				StatusCheckData: &pullCommitStatusCheckData{
					pullCommitStatusState: commitstatus.CommitStatusPending,
					PullCommitStatuses: []*git_model.CommitStatus{
						{State: commitstatus.CommitStatusPending},
					},
				},
			},
			want: "pending",
		},
		{
			name: "failure",
			data: &pullMergeBoxData{
				ShowStatusCheck: true,
				StatusCheckData: &pullCommitStatusCheckData{
					pullCommitStatusState: commitstatus.CommitStatusFailure,
					PullCommitStatuses: []*git_model.CommitStatus{
						{State: commitstatus.CommitStatusFailure},
					},
				},
			},
			want: "failure",
		},
		{
			name: "missing required checks",
			data: &pullMergeBoxData{
				ShowStatusCheck: true,
				StatusCheckData: &pullCommitStatusCheckData{
					MissingRequiredChecks: []string{"ci/test"},
					RequiredChecksState:   commitstatus.CommitStatusPending,
				},
			},
			want: "pending",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, test.data.FaviconVariant())
		})
	}
}
