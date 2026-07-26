// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_18

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func Test_UpdateOpenMilestoneCounts(t *testing.T) {
	type Issue struct {
		ID          int64
		RepoID      int64
		Index       int64
		MilestoneID int64
		IsClosed    bool
		UpdatedUnix timeutil.TimeStamp
	}

	type Milestone struct {
		ID              int64
		IsClosed        bool
		NumIssues       int
		NumClosedIssues int
		Completeness    int
		UpdatedUnix     timeutil.TimeStamp
	}

	type ExpectedMilestone Milestone

	// Prepare and load the testing database
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(Milestone), new(ExpectedMilestone), new(Issue))
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

	got := []Milestone{}
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
