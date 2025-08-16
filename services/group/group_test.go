// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

// group 12 is private
// team 23 are owners

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestNewGroup(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	const groupName = "group x"
	group := &group_model.Group{
		Name:    groupName,
		OwnerID: 3,
	}
	assert.NoError(t, NewGroup(db.DefaultContext, group))
	unittest.AssertExistsAndLoadBean(t, &group_model.Group{Name: groupName})
}

func TestMoveGroup(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{
		ID: 28,
	})
	testfn := func(gid int64) {
		cond := &group_model.FindGroupsOptions{
			ParentGroupID: 123,
			OwnerID:       3,
		}
		origCount := unittest.GetCount(t, new(group_model.Group), cond.ToConds())

		assert.NoError(t, MoveGroupItem(t.Context(), MoveGroupOptions{
			NewParent: 123,
			ItemID:    gid,
			IsGroup:   true,
			NewPos:    -1,
		}, doer))
		unittest.AssertCountByCond(t, "repo_group", cond.ToConds(), origCount+1)
	}
	testfn(124)
	testfn(132)
	testfn(150)
}

func TestMoveRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{
		ID: 28,
	})
	cond := repo_model.SearchRepositoryCondition(repo_model.SearchRepoOptions{
		GroupID: 123,
	})
	origCount := unittest.GetCount(t, new(repo_model.Repository), cond)

	assert.NoError(t, MoveGroupItem(db.DefaultContext, MoveGroupOptions{
		NewParent: 123,
		ItemID:    32,
		IsGroup:   false,
		NewPos:    -1,
	}, doer))
	unittest.AssertCountByCond(t, "repository", cond, origCount+1)
}
