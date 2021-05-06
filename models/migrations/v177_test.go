// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"testing"

	"code.gitea.io/gitea/modules/timeutil"
	"github.com/stretchr/testify/assert"
)

func Test_deleteOrphanedIssueLabels(t *testing.T) {
	// Create the models used in the migration
	type IssueLabel struct {
		ID      int64 `xorm:"pk autoincr"`
		IssueID int64 `xorm:"UNIQUE(s)"`
		LabelID int64 `xorm:"UNIQUE(s)"`
	}

	type Label struct {
		ID              int64 `xorm:"pk autoincr"`
		RepoID          int64 `xorm:"INDEX"`
		OrgID           int64 `xorm:"INDEX"`
		Name            string
		Description     string
		Color           string `xorm:"VARCHAR(7)"`
		NumIssues       int
		NumClosedIssues int
		CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix     timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	// Prepare and load the testing database
	x, deferable := prepareTestEnv(t, 0, new(IssueLabel), new(Label))
	if x == nil || t.Failed() {
		defer deferable()
		return
	}
	defer deferable()

	var issueLabels []*IssueLabel
	preMigration := map[int64]*IssueLabel{}
	postMigration := map[int64]*IssueLabel{}

	// Load issue labels that exist in the database pre-migration
	if err := x.Find(&issueLabels); err != nil {
		assert.NoError(t, err)
		return
	}
	for _, issueLabel := range issueLabels {
		preMigration[issueLabel.ID] = issueLabel
	}

	// Run the migration
	if err := deleteOrphanedIssueLabels(x); err != nil {
		assert.NoError(t, err)
		return
	}

	// Load the remaining issue-labels
	issueLabels = issueLabels[:0]
	if err := x.Find(&issueLabels); err != nil {
		assert.NoError(t, err)
		return
	}
	for _, issueLabel := range issueLabels {
		postMigration[issueLabel.ID] = issueLabel
	}

	// Now test what is left
	if _, ok := postMigration[2]; ok {
		t.Errorf("Orphaned Label[2] survived the migration")
		return
	}

	if _, ok := postMigration[5]; ok {
		t.Errorf("Orphaned Label[5] survived the migration")
		return
	}

	for id, post := range postMigration {
		pre := preMigration[id]
		assert.Equal(t, pre, post, "migration changed issueLabel %d", id)
	}

}
