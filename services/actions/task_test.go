// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func Test_findTaskNeeds(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	task := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionTask{ID: 51})
	job := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionRunJob{ID: task.JobID})

	ret, err := findTaskNeeds(context.Background(), job)
	assert.NoError(t, err)
	assert.Len(t, ret, 1)
	assert.Contains(t, ret, "job1")
	assert.Len(t, ret["job1"].Outputs, 2)
	assert.Equal(t, "abc", ret["job1"].Outputs["output_a"])
	assert.Equal(t, "bbb", ret["job1"].Outputs["output_b"])
}
