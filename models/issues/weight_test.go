// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestGetIssueTotalWeight(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	tw, twc, err := issues_model.GetIssueTotalWeight(db.DefaultContext, &issues_model.IssuesOptions{MilestoneIDs: []int64{1}})
	assert.NoError(t, err)
	assert.EqualValues(t, 12, tw)
	assert.EqualValues(t, 12, twc)

	tw, twc, err = issues_model.GetIssueTotalWeight(db.DefaultContext, &issues_model.IssuesOptions{MilestoneIDs: []int64{3}})
	assert.NoError(t, err)
	assert.EqualValues(t, 5, tw)
	assert.EqualValues(t, 5, twc)

	tw, twc, err = issues_model.GetIssueTotalWeight(db.DefaultContext, &issues_model.IssuesOptions{RepoIDs: []int64{2}})
	assert.NoError(t, err)
	assert.EqualValues(t, 20, tw)
	assert.EqualValues(t, 10, twc)
}
