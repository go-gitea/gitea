// Copyright 2024 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/commitstatus"

	"github.com/stretchr/testify/assert"
)

func TestMergeRequiredContextsCommitStatus(t *testing.T) {
	cases := []struct {
		commitStatuses   []*git_model.CommitStatus
		requiredContexts []string
		expected         commitstatus.CommitStatusState
	}{
		{
			commitStatuses:   []*git_model.CommitStatus{},
			requiredContexts: []string{},
			expected:         commitstatus.CommitStatusPending,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build xxx", State: commitstatus.CommitStatusSkipped},
			},
			requiredContexts: []string{"Build*"},
			expected:         commitstatus.CommitStatusSuccess,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: commitstatus.CommitStatusSkipped},
				{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 3", State: commitstatus.CommitStatusSuccess},
			},
			requiredContexts: []string{"Build*"},
			expected:         commitstatus.CommitStatusSuccess,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2t", State: commitstatus.CommitStatusPending},
			},
			requiredContexts: []string{"Build*", "Build 2t*"},
			expected:         commitstatus.CommitStatusPending,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2t", State: commitstatus.CommitStatusFailure},
			},
			requiredContexts: []string{"Build*", "Build 2t*"},
			expected:         commitstatus.CommitStatusFailure,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2t", State: commitstatus.CommitStatusFailure},
			},
			requiredContexts: []string{"Build*"},
			expected:         commitstatus.CommitStatusFailure,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2t", State: commitstatus.CommitStatusSuccess},
			},
			requiredContexts: []string{"Build*", "Build 2t*", "Build 3*"},
			expected:         commitstatus.CommitStatusPending,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
				{Context: "Build 2t", State: commitstatus.CommitStatusSuccess},
			},
			requiredContexts: []string{"Build*", "Build *", "Build 2t*", "Build 1*"},
			expected:         commitstatus.CommitStatusSuccess,
		},
	}
	for i, c := range cases {
		assert.Equal(t, c.expected, MergeRequiredContextsCommitStatus(c.commitStatuses, c.requiredContexts), "case %d", i)
	}
}
