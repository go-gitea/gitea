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
	testCases := [][]*git_model.CommitStatus{
		{
			{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
			{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
			{Context: "Build 3", State: commitstatus.CommitStatusSuccess},
		},
		{
			{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
			{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
			{Context: "Build 2t", State: commitstatus.CommitStatusPending},
		},
		{
			{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
			{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
			{Context: "Build 2t", State: commitstatus.CommitStatusFailure},
		},
		{
			{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
			{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
			{Context: "Build 2t", State: commitstatus.CommitStatusSuccess},
		},
		{
			{Context: "Build 1", State: commitstatus.CommitStatusSuccess},
			{Context: "Build 2", State: commitstatus.CommitStatusSuccess},
			{Context: "Build 2t", State: commitstatus.CommitStatusSuccess},
		},
	}
	testCasesRequiredContexts := [][]string{
		{"Build*"},
		{"Build*", "Build 2t*"},
		{"Build*", "Build 2t*"},
		{"Build*", "Build 2t*", "Build 3*"},
		{"Build*", "Build *", "Build 2t*", "Build 1*"},
	}

	testCasesExpected := []commitstatus.CombinedStatusState{
		commitstatus.CombinedStatusSuccess,
		commitstatus.CombinedStatusPending,
		commitstatus.CombinedStatusFailure,
		commitstatus.CombinedStatusPending,
		commitstatus.CombinedStatusSuccess,
	}

	for i, commitStatuses := range testCases {
		status := MergeRequiredContextsCommitStatus(commitStatuses, testCasesRequiredContexts[i])
		if status != testCasesExpected[i] {
			assert.Fail(t, "Test case failed", "Test case %d failed: expect %s, got %s", i+1, testCasesExpected[i], status)
		}
	}
}
