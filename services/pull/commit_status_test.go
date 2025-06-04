// Copyright 2024 The Gitea Authors.
// All rights reserved.
// SPDX-License-Identifier: MIT

package pull

import (
	"testing"

	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestMergeRequiredContextsCommitStatus(t *testing.T) {
	cases := []struct {
		commitStatuses   []*git_model.CommitStatus
		requiredContexts []string
		expected         structs.CommitStatusState
	}{
		{
			commitStatuses:   []*git_model.CommitStatus{},
			requiredContexts: []string{},
			expected:         structs.CommitStatusPending,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build xxx", State: structs.CommitStatusSkipped},
			},
			requiredContexts: []string{"Build*"},
			expected:         structs.CommitStatusSuccess,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: structs.CommitStatusSkipped},
				{Context: "Build 2", State: structs.CommitStatusSuccess},
				{Context: "Build 3", State: structs.CommitStatusSuccess},
			},
			requiredContexts: []string{"Build*"},
			expected:         structs.CommitStatusSuccess,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: structs.CommitStatusSuccess},
				{Context: "Build 2", State: structs.CommitStatusSuccess},
				{Context: "Build 2t", State: structs.CommitStatusPending},
			},
			requiredContexts: []string{"Build*", "Build 2t*"},
			expected:         structs.CommitStatusPending,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: structs.CommitStatusSuccess},
				{Context: "Build 2", State: structs.CommitStatusSuccess},
				{Context: "Build 2t", State: structs.CommitStatusFailure},
			},
			requiredContexts: []string{"Build*", "Build 2t*"},
			expected:         structs.CommitStatusFailure,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: structs.CommitStatusSuccess},
				{Context: "Build 2", State: structs.CommitStatusSuccess},
				{Context: "Build 2t", State: structs.CommitStatusSuccess},
			},
			requiredContexts: []string{"Build*", "Build 2t*", "Build 3*"},
			expected:         structs.CommitStatusPending,
		},
		{
			commitStatuses: []*git_model.CommitStatus{
				{Context: "Build 1", State: structs.CommitStatusSuccess},
				{Context: "Build 2", State: structs.CommitStatusSuccess},
				{Context: "Build 2t", State: structs.CommitStatusSuccess},
			},
			requiredContexts: []string{"Build*", "Build *", "Build 2t*", "Build 1*"},
			expected:         structs.CommitStatusSuccess,
		},
	}
	for i, c := range cases {
		assert.Equal(t, c.expected, MergeRequiredContextsCommitStatus(c.commitStatuses, c.requiredContexts), "case %d", i)
	}
}
