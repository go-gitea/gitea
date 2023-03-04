// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"testing"

	"code.gitea.io/gitea/models/issues"

	"github.com/stretchr/testify/assert"
)

func Test_updateOpenMilestoneCounts(t *testing.T) {
	type ExpectedMilestone issues.Milestone

	// Prepare and load the testing database
	x, deferable := prepareTestEnv(t, 0, new(issues.Milestone), new(ExpectedMilestone), new(issues.Issue))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	if err := updateOpenMilestoneCounts(x); err != nil {
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
