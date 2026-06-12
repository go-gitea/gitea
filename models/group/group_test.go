// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package group_test

import (
	"fmt"
	"slices"
	"testing"

	"gitea.dev/models/db"
	group_model "gitea.dev/models/group"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

func createTestGroup(t *testing.T, name string, pgid int64) *group_model.Group {
	newGroup := &group_model.Group{
		Name:          name,
		OwnerID:       3,
		ParentGroupID: pgid,
	}
	e := db.GetEngine(t.Context())
	curCount, err := e.Where(builder.Eq{"parent_group_id": pgid}).Table(newGroup.TableName()).Count()
	assert.NoError(t, err)
	newGroup.SortOrder = int(curCount)
	_, err = e.Insert(newGroup)
	assert.NoError(t, err)
	return newGroup
}

func createParentGroup(t *testing.T) (*group_model.Group, group_model.RepoGroupList) {
	parentGroup := createTestGroup(t, t.Name(), 0)
	var groups group_model.RepoGroupList
	for i := range 7 {
		groups = append(groups, createTestGroup(t, fmt.Sprintf("group %d", i+1), parentGroup.ID))
	}
	return parentGroup, groups
}

func assertGroupOrder(t *testing.T, pgid int64, expectedIDs []int64) {
	e := db.GetEngine(t.Context())
	groups := make(group_model.RepoGroupList, 0)
	err := e.Where(builder.Eq{"parent_group_id": pgid}).Asc("sort_order").Find(&groups)
	mappedIDs := util.SliceMap(groups, getID)
	assert.NoError(t, err)
	assert.Len(t, groups, len(expectedIDs))
	for i, group := range mappedIDs {
		assert.Equal(t, expectedIDs[i], group)
		assert.Equal(t, i, groups[i].SortOrder)
	}
}

func getID(it *group_model.Group) int64 {
	return it.ID
}

func TestIsPrivateBecauseOfParentPermissions(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	orgMember := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	setVisibility := func(t *testing.T, group *group_model.Group, visibility structs.VisibleType) {
		t.Helper()
		_, err := db.GetEngine(ctx).ID(group.ID).Cols("visibility").Update(&group_model.Group{
			Visibility: visibility,
		})
		require.NoError(t, err)
	}

	t.Run("PublicHierarchy", func(t *testing.T) {
		root := createTestGroup(t, "public root", 0)
		child := createTestGroup(t, "public child", root.ID)

		private, err := child.IsPrivateBecauseOfParentPermissions(ctx, nil)
		require.NoError(t, err)
		assert.False(t, private)
	})

	t.Run("PrivateRoot", func(t *testing.T) {
		root := createTestGroup(t, "private root", 0)
		setVisibility(t, root, structs.VisibleTypePrivate)
		child := createTestGroup(t, "public child", root.ID)

		private, err := child.IsPrivateBecauseOfParentPermissions(ctx, nil)
		require.NoError(t, err)
		assert.True(t, private)

		private, err = child.IsPrivateBecauseOfParentPermissions(ctx, orgMember)
		require.NoError(t, err)
		assert.False(t, private)

		private, err = child.IsPrivateBecauseOfParentPermissions(ctx, admin)
		require.NoError(t, err)
		assert.False(t, private)
	})

	t.Run("PrivateIntermediateAncestor", func(t *testing.T) {
		root := createTestGroup(t, "public root", 0)
		parent := createTestGroup(t, "private parent", root.ID)
		setVisibility(t, parent, structs.VisibleTypePrivate)
		child := createTestGroup(t, "public child", parent.ID)

		private, err := child.IsPrivateBecauseOfParentPermissions(ctx, nil)
		require.NoError(t, err)
		assert.True(t, private)
	})
}

func TestMoveGroup(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	t.Run("NewPositionBeforeOldPosition", func(t *testing.T) {
		parentGroup, groups := createParentGroup(t)
		err := group_model.MoveGroup(t.Context(), groups[3], parentGroup.ID, 1)
		assert.NoError(t, err)
		first := util.SliceMap(groups[0:1], getID)
		first = append(first, groups[3].ID)
		middle := util.SliceMap(groups[1:3], getID)
		end := util.SliceMap(groups[4:], getID)
		assertGroupOrder(t, parentGroup.ID, slices.Concat(first, middle, end))
	})
	t.Run("NewPositionAfterOldPosition", func(t *testing.T) {
		parentGroup, groups := createParentGroup(t)
		err := group_model.MoveGroup(t.Context(), groups[3], parentGroup.ID, 4)
		assert.NoError(t, err)
		first := util.SliceMap(groups[0:3], getID)
		middle := util.SliceMap(groups[4:5], getID)
		middle = append(middle, groups[3].ID)
		end := util.SliceMap(groups[5:], getID)
		assertGroupOrder(t, parentGroup.ID, slices.Concat(first, middle, end))
	})
	t.Run("ToFirst", func(t *testing.T) {
		parentGroup, groups := createParentGroup(t)
		err := group_model.MoveGroup(t.Context(), groups[3], parentGroup.ID, 0)
		assert.NoError(t, err)
		first := util.SliceMap(groups[0:3], getID)
		end := util.SliceMap(groups[4:], getID)
		onlyItem := []int64{groups[3].ID}
		assertGroupOrder(t, parentGroup.ID, slices.Concat(onlyItem, first, end))
	})
	t.Run("ToLast", func(t *testing.T) {
		parentGroup, groups := createParentGroup(t)
		err := group_model.MoveGroup(t.Context(), groups[3], parentGroup.ID, 7)
		assert.NoError(t, err)
		first := util.SliceMap(groups[0:3], getID)
		end := util.SliceMap(groups[4:], getID)
		onlyItem := []int64{groups[3].ID}
		assertGroupOrder(t, parentGroup.ID, slices.Concat(first, end, onlyItem))
	})
	t.Run("ToEmptyParent", func(t *testing.T) {
		oldParent, groups := createParentGroup(t)
		newParent := createTestGroup(t, "empty parent", 0)
		err := group_model.MoveGroup(t.Context(), groups[3], newParent.ID, 0)
		assert.NoError(t, err)
		first := util.SliceMap(groups[0:3], getID)
		end := util.SliceMap(groups[4:], getID)
		assertGroupOrder(t, oldParent.ID, slices.Concat(first, end))
		assertGroupOrder(t, newParent.ID, []int64{groups[3].ID})
	})
	t.Run("ToDifferentParentWithSiblings", func(t *testing.T) {
		oldParent, groups := createParentGroup(t)
		newParent, newSiblings := createParentGroup(t)
		err := group_model.MoveGroup(t.Context(), groups[3], newParent.ID, 1)
		assert.NoError(t, err)
		first := util.SliceMap(groups[0:3], getID)
		end := util.SliceMap(groups[4:], getID)
		assertGroupOrder(t, oldParent.ID, slices.Concat(first, end))
		assertGroupOrder(t, newParent.ID, slices.Concat(
			util.SliceMap(newSiblings[0:1], getID),
			[]int64{groups[3].ID},
			util.SliceMap(newSiblings[1:], getID),
		))
	})
	t.Run("RejectsMovingUnderSelf", func(t *testing.T) {
		parentGroup, _ := createParentGroup(t)
		err := group_model.MoveGroup(t.Context(), parentGroup, parentGroup.ID, 0)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
		reloaded, err := group_model.GetGroupByID(t.Context(), parentGroup.ID)
		assert.NoError(t, err)
		assert.EqualValues(t, 0, reloaded.ParentGroupID)
	})
	t.Run("RejectsMovingUnderDescendant", func(t *testing.T) {
		parentGroup, groups := createParentGroup(t)
		child := createTestGroup(t, "child group", groups[3].ID)
		err := group_model.MoveGroup(t.Context(), groups[3], child.ID, 0)
		assert.ErrorIs(t, err, util.ErrInvalidArgument)
		assertGroupOrder(t, parentGroup.ID, util.SliceMap(groups, getID))
		assertGroupOrder(t, groups[3].ID, []int64{child.ID})
	})
}
