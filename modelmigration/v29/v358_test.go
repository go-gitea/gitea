// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/migrationtest"

	"github.com/stretchr/testify/assert"
)

type TeamBefore struct {
	ID        int64 `xorm:"pk autoincr"`
	OrgID     int64 `xorm:"INDEX"`
	LowerName string
}

func (TeamBefore) TableName() string {
	return "team"
}

func Test_AddUniqueIndexForTeam(t *testing.T) {
	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(TeamBefore))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	testData := []*TeamBefore{
		{OrgID: 1, LowerName: "team1"},
		{OrgID: 1, LowerName: "team1"}, // duplicate
		{OrgID: 2, LowerName: "team1"},
		{OrgID: 1, LowerName: "team2"},
		{OrgID: 3, LowerName: "team3"},
		{OrgID: 3, LowerName: "team3"}, // duplicate
	}

	for _, data := range testData {
		_, err := x.Insert(data)
		assert.NoError(t, err)
	}

	// check that we have duplicates
	count, err := x.Where("org_id = ? AND lower_name = ?", 1, "team1").Count(&TeamBefore{})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count, err = x.Where("org_id = ? AND lower_name = ?", 3, "team3").Count(&TeamBefore{})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	totalCount, err := x.Count(&TeamBefore{})
	assert.NoError(t, err)
	assert.Equal(t, int64(6), totalCount)

	// run the migration
	if err := AddUniqueIndexForTeam(t.Context(), x); err != nil {
		assert.NoError(t, err)
		return
	}

	// verify the duplicates were removed
	count, err = x.Where("org_id = ? AND lower_name = ?", 1, "team1").Count(&TeamBefore{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	count, err = x.Where("org_id = ? AND lower_name = ?", 3, "team3").Count(&TeamBefore{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// check total count
	totalCount, err = x.Count(&TeamBefore{})
	assert.NoError(t, err)
	assert.Equal(t, int64(4), totalCount)

	// fail to insert a duplicate
	_, err = x.Insert(&Team{OrgID: 1, LowerName: "team1"})
	assert.Error(t, err)

	// succeed adding a non-duplicate
	_, err = x.Insert(&Team{OrgID: 1, LowerName: "team9"})
	assert.NoError(t, err)
}
