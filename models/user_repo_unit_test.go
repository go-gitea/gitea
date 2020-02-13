// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserRepoUnit(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.NoError(t, UserRepoUnitTestDo(x))
}

func TestUserRepoUnit_Repo5(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// To monitor this test, change the following line in unit_tests.go:
	// x, err = xorm.NewEngine("sqlite3", "file::memory:?cache=shared")
	// into:
	//		x, err = xorm.NewEngine("sqlite3", "file:/tmp/sqlite3-test.db?cache=shared&mode=rwc&_busy_timeout=30")

	// Also, uncomment the following block
	/*
		workTable = "user_repo_unit_test5"
		workTableCreate = "CREATE TABLE user_repo_unit_test5 AS SELECT * FROM user_repo_unit WHERE 0 = 1"
		workTableDrop = "DROP TABLE user_repo_unit_test5"
		_, _ = x.Exec(workTableDrop)
		_, err := x.Exec(workTableCreate)
		assert.NoError(t, err)
	*/

	repo := AssertExistsAndLoadBean(t, &Repository{ID: 5}).(*Repository)
	assert.NoError(t, batchBuildRepoUnits(x, repo, -1))

	// Finally, uncomment this:
	// _, _ = x.Exec(workTableDrop)

	// Use the following command to inspect the results:
	// sqlite3 -column -header -nullvalue '<null>' sqlite3-test.db
}
