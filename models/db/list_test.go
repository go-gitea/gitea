// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
)

type mockListOptions struct {
	db.ListOptions
}

func (opts *mockListOptions) IsListAll() bool {
	return true
}

func (opts *mockListOptions) ToConds() builder.Cond {
	return builder.NewCond()
}

func TestFind(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	xe := unittest.GetXORMEngine()
	assert.NoError(t, xe.Sync(&repo_model.RepoUnit{}))

	var repoUnitCount int
	_, err := db.GetEngine(db.DefaultContext).SQL("SELECT COUNT(*) FROM repo_unit").Get(&repoUnitCount)
	assert.NoError(t, err)
	assert.NotEmpty(t, repoUnitCount)

	opts := mockListOptions{}
	var repoUnits []repo_model.RepoUnit
	err = db.Find(db.DefaultContext, &opts, &repoUnits)
	assert.NoError(t, err)
	assert.Len(t, repoUnits, repoUnitCount)

	cnt, err := db.Count(db.DefaultContext, &opts, new(repo_model.RepoUnit))
	assert.NoError(t, err)
	assert.EqualValues(t, repoUnitCount, cnt)

	repoUnits = make([]repo_model.RepoUnit, 0, 10)
	newCnt, err := db.FindAndCount(db.DefaultContext, &opts, &repoUnits)
	assert.NoError(t, err)
	assert.EqualValues(t, cnt, newCnt)
}
