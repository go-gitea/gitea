// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/optional"

	"github.com/stretchr/testify/assert"
)

func Test_Suggestion(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	testCases := []struct {
		keyword         string
		isPull          optional.Option[bool]
		expectedIndexes []int64
	}{
		{
			keyword:         "",
			expectedIndexes: []int64{5, 1, 4, 2, 3},
		},
		{
			keyword:         "1",
			expectedIndexes: []int64{1},
		},
		{
			keyword:         "issue",
			expectedIndexes: []int64{4, 1, 2, 3},
		},
		{
			keyword:         "pull",
			expectedIndexes: []int64{5},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.keyword, func(t *testing.T) {
			issues, err := GetSuggestion(db.DefaultContext, repo1, testCase.isPull, testCase.keyword)
			assert.NoError(t, err)

			issueIndexes := make([]int64, 0, len(issues))
			for _, issue := range issues {
				issueIndexes = append(issueIndexes, issue.Index)
			}
			assert.EqualValues(t, testCase.expectedIndexes, issueIndexes)
		})
	}
}
