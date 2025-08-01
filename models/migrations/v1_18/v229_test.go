// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_18

import (
	"testing"

	"code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func Test_UpdateOpenMilestoneCounts(t *testing.T) {
	type ExpectedMilestone issues.Milestone

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(issues.Milestone), new(ExpectedMilestone), new(issues.Issue))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	if err := UpdateOpenMilestoneCounts(x); err != nil {
		assert.NoError(t, err)
		return
	}

	expected := []ExpectedMilestone{}
	if err := x.Table("expected_milestone").Asc("id").Find(&expected); !assert.NoError(t, err) {
		return
	}

	got := []issues.Milestone{}
	if err := x.Table("milestone").Asc("id").Find(&got); !assert.NoError(t, err) {
		return
	}

	for i, e := range expected {
		got := got[i]
		assert.Equal(t, e.ID, got.ID)
		assert.Equal(t, e.NumIssues, got.NumIssues)
		assert.Equal(t, e.NumClosedIssues, got.NumClosedIssues)
	}
}
