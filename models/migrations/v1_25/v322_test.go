// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

type UserBadgeBefore struct {
	ID      int64 `xorm:"pk autoincr"`
	BadgeID int64
	UserID  int64 `xorm:"INDEX"`
}

func (UserBadgeBefore) TableName() string {
	return "user_badge"
}

func Test_AddUniqueIndexForUserBadge(t *testing.T) {
	x, deferable := base.PrepareTestEnv(t, 0, new(UserBadgeBefore))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	testData := []*UserBadgeBefore{
		{UserID: 1, BadgeID: 1},
		{UserID: 1, BadgeID: 1}, // duplicate
		{UserID: 2, BadgeID: 1},
		{UserID: 1, BadgeID: 2},
		{UserID: 3, BadgeID: 3},
		{UserID: 3, BadgeID: 3}, // duplicate
	}

	for _, data := range testData {
		_, err := x.Insert(data)
		assert.NoError(t, err)
	}

	// check that we have duplicates
	count, err := x.Where("user_id = ? AND badge_id = ?", 1, 1).Count(&UserBadgeBefore{})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count, err = x.Where("user_id = ? AND badge_id = ?", 3, 3).Count(&UserBadgeBefore{})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	totalCount, err := x.Count(&UserBadgeBefore{})
	assert.NoError(t, err)
	assert.Equal(t, int64(6), totalCount)

	// run the migration
	if err := AddUniqueIndexForUserBadge(x); err != nil {
		assert.NoError(t, err)
		return
	}

	// verify the duplicates were removed
	count, err = x.Where("user_id = ? AND badge_id = ?", 1, 1).Count(&UserBadgeBefore{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	count, err = x.Where("user_id = ? AND badge_id = ?", 3, 3).Count(&UserBadgeBefore{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// check total count
	totalCount, err = x.Count(&UserBadgeBefore{})
	assert.NoError(t, err)
	assert.Equal(t, int64(4), totalCount)

	// fail to insert a duplicate
	_, err = x.Insert(&UserBadge{UserID: 1, BadgeID: 1})
	assert.Error(t, err)

	// succeed adding a non-duplicate
	_, err = x.Insert(&UserBadge{UserID: 4, BadgeID: 1})
	assert.NoError(t, err)
}
