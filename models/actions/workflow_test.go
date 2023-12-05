// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestWorkFlowLabels(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// test update
	err := UpdateWorkFlowLabels(db.DefaultContext, 2, "test-workflow-2.yml", "test1", []string{"aa", "bb"})
	assert.NoError(t, err)

	w := unittest.AssertExistsAndLoadBean(t, &ActionWorkflow{RepoID: 2, Name: "test-workflow-2.yml"})
	assert.EqualValues(t, []string{"aa", "bb"}, w.LoadLabels())

	err = UpdateWorkFlowLabels(db.DefaultContext, 2, "test-workflow-2.yml", "test2", []string{"aa", "bb", "cccc"})
	assert.NoError(t, err)

	w = unittest.AssertExistsAndLoadBean(t, &ActionWorkflow{RepoID: 2, Name: "test-workflow-2.yml"})
	assert.EqualValues(t, []string{"aa", "bb", "cccc"}, w.LoadLabels())

	err = UpdateWorkFlowLabels(db.DefaultContext, 2, "test-workflow-2.yml", "test2", []string{"aa", "bb", "dddd"})
	assert.NoError(t, err)

	w = unittest.AssertExistsAndLoadBean(t, &ActionWorkflow{RepoID: 2, Name: "test-workflow-2.yml"})
	assert.EqualValues(t, []string{"aa", "bb", "dddd"}, w.LoadLabels())

	// test delete
	err = DeleteWorkFlowBranch(db.DefaultContext, 2, "test-workflow-2.yml", "test2")
	assert.NoError(t, err)
	w = unittest.AssertExistsAndLoadBean(t, &ActionWorkflow{RepoID: 2, Name: "test-workflow-2.yml"})
	assert.EqualValues(t, []string{"aa", "bb"}, w.LoadLabels())

	err = DeleteWorkFlowBranch(db.DefaultContext, 2, "test-workflow-2.yml", "test1")
	assert.NoError(t, err)
	unittest.AssertNotExistsBean(t, &ActionWorkflow{RepoID: 2, Name: "test-workflow-2.yml"})
}
