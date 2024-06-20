// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
)

func TestBuildCanSeeUserCondition(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	getIDs := func(cond builder.Cond) (ids []int64) {
		db.GetEngine(db.DefaultContext).Select("id").Table(`user`).
			Where(builder.Eq{"is_active": true}.And(cond)).Asc("id").Find(&ids)
		return ids
	}

	getUser := func(t *testing.T, id int64) *user.User {
		user, err := user.GetUserByID(db.DefaultContext, id)
		assert.NoError(t, err)
		if !assert.NotNil(t, user) {
			t.FailNow()
		}
		return user
	}

	// no login
	cond := user.BuildCanSeeUserCondition(nil)
	assert.NotNil(t, cond)
	ids := getIDs(cond)
	assert.Len(t, ids, 24)
	assert.EqualValues(t, []int64{1, 2, 4, 5, 8, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 24, 27, 28, 29, 30, 32, 34}, ids)

	// normal user
	cond = user.BuildCanSeeUserCondition(getUser(t, 5))
	ids = getIDs(cond)
	assert.Len(t, ids, 28)
	assert.EqualValues(t, []int64{1, 2, 4, 5, 8, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 27, 28, 29, 30, 32, 33, 34, 36}, ids)

	// admin
	cond = user.BuildCanSeeUserCondition(getUser(t, 1))
	assert.Nil(t, cond)
	ids = getIDs(cond)
	assert.Len(t, ids, 30)
	assert.EqualValues(t, []int64{1, 2, 4, 5, 8, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36}, ids)

	// private user
	cond = user.BuildCanSeeUserCondition(getUser(t, 31))
	ids = getIDs(cond)
	assert.Len(t, ids, 28)
	assert.EqualValues(t, []int64{1, 2, 4, 5, 8, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 24, 27, 28, 29, 30, 31, 32, 33, 34, 36}, ids)

	// limited user who is followed by private user
	cond = user.BuildCanSeeUserCondition(getUser(t, 33))
	ids = getIDs(cond)
	assert.Len(t, ids, 28)
	assert.EqualValues(t, []int64{1, 2, 4, 5, 8, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 24, 27, 28, 29, 30, 31, 32, 33, 34, 36}, ids)

	// restricted user
	cond = user.BuildCanSeeUserCondition(getUser(t, 29))
	ids = getIDs(cond)
	assert.Len(t, ids, 2)
	assert.EqualValues(t, []int64{17, 29}, ids)
}
