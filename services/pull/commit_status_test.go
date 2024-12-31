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
	testCases := [][]*git_model.CommitStatus{
		{
			{Context: "Build 1", State: structs.CommitStatusSuccess},
			{Context: "Build 2", State: structs.CommitStatusSuccess},
			{Context: "Build 3", State: structs.CommitStatusSuccess},
		},
		{
			{Context: "Build 1", State: structs.CommitStatusSuccess},
			{Context: "Build 2", State: structs.CommitStatusSuccess},
			{Context: "Build 2t", State: structs.CommitStatusPending},
		},
		{
			{Context: "Build 1", State: structs.CommitStatusSuccess},
			{Context: "Build 2", State: structs.CommitStatusSuccess},
			{Context: "Build 2t", State: structs.CommitStatusFailure},
		},
		{
			{Context: "Build 1", State: structs.CommitStatusSuccess},
			{Context: "Build 2", State: structs.CommitStatusSuccess},
			{Context: "Build 2t", State: structs.CommitStatusSuccess},
		},
		{
			{Context: "Build 1", State: structs.CommitStatusSuccess},
			{Context: "Build 2", State: structs.CommitStatusSuccess},
			{Context: "Build 2t", State: structs.CommitStatusSuccess},
		},
	}
	testCasesRequiredContexts := [][]string{
		{"Build*"},
		{"Build*", "Build 2t*"},
		{"Build*", "Build 2t*"},
		{"Build*", "Build 2t*", "Build 3*"},
		{"Build*", "Build *", "Build 2t*", "Build 1*"},
	}

	testCasesExpected := []structs.CommitStatusState{
		structs.CommitStatusSuccess,
		structs.CommitStatusPending,
		structs.CommitStatusFailure,
		structs.CommitStatusPending,
		structs.CommitStatusSuccess,
	}

	for i, commitStatuses := range testCases {
		if MergeRequiredContextsCommitStatus(commitStatuses, testCasesRequiredContexts[i]) != testCasesExpected[i] {
			assert.Fail(t, "Test case failed", "Test case %d failed", i+1)
		}
	}
}
