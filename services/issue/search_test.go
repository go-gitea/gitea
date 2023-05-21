// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"testing"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestSearch(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	total, ids, err := Search(context.TODO(), &issues_model.IssuesOptions{
		RepoIDs: []int64{1},
		Keyword: "issue2",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{2}, ids)
	assert.EqualValues(t, 1, total)

	total, ids, err = Search(context.TODO(), &issues_model.IssuesOptions{
		RepoIDs: []int64{1},
		Keyword: "first",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)
	assert.EqualValues(t, 1, total)

	total, ids, err = Search(context.TODO(), &issues_model.IssuesOptions{
		RepoIDs: []int64{1},
		Keyword: "for",
	})
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int64{1, 2, 3, 5, 11}, ids)
	assert.EqualValues(t, 5, total)

	total, ids, err = Search(context.TODO(), &issues_model.IssuesOptions{
		RepoIDs: []int64{1},
		Keyword: "good",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, []int64{1}, ids)
	assert.EqualValues(t, 1, total)
}
