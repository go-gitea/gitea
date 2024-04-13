// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"

	"github.com/stretchr/testify/assert"
)

func TestGetAllRerunJobs(t *testing.T) {
	job1 := &actions_model.ActionRunJob{JobID: "job1"}
	job2 := &actions_model.ActionRunJob{JobID: "job2", Needs: []string{"job1"}}
	job3 := &actions_model.ActionRunJob{JobID: "job3", Needs: []string{"job2"}}
	job4 := &actions_model.ActionRunJob{JobID: "job4", Needs: []string{"job2", "job3"}}

	jobs := []*actions_model.ActionRunJob{job1, job2, job3, job4}

	testCases := []struct {
		job       *actions_model.ActionRunJob
		rerunJobs []*actions_model.ActionRunJob
	}{
		{
			job1,
			[]*actions_model.ActionRunJob{job1, job2, job3, job4},
		},
		{
			job2,
			[]*actions_model.ActionRunJob{job2, job3, job4},
		},
		{
			job3,
			[]*actions_model.ActionRunJob{job3, job4},
		},
		{
			job4,
			[]*actions_model.ActionRunJob{job4},
		},
	}

	for _, tc := range testCases {
		rerunJobs := GetAllRerunJobs(tc.job, jobs)
		assert.ElementsMatch(t, tc.rerunJobs, rerunJobs)
	}
}
