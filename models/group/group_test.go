package group_test

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/util"
	"github.com/stretchr/testify/assert"
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

func assertGroupOrder(t *testing.T, pgid int64, expectedIds []int64) {
	e := db.GetEngine(t.Context())
	groups := make(group_model.RepoGroupList, 0)
	err := e.Where(builder.Eq{"parent_group_id": pgid}).Asc("sort_order").Find(&groups)
	mappedIDs := util.SliceMap(groups, getID)
	assert.NoError(t, err)
	for i, group := range mappedIDs {
		assert.Equal(t, expectedIds[i], group)
		assert.Equal(t, i, groups[i].SortOrder)
	}
}

func getID(it *group_model.Group) int64 {
	return it.ID
}

func combineSlices[E any](sl ...[]E) []E {
	final := make([]E, 0)
	for _, subslice := range sl {
		final = append(final, subslice...)
	}
	return final
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
		assertGroupOrder(t, parentGroup.ID, combineSlices(first, middle, end))
	})
	t.Run("NewPositionAfterOldPosition", func(t *testing.T) {
		parentGroup, groups := createParentGroup(t)
		err := group_model.MoveGroup(t.Context(), groups[3], parentGroup.ID, 4)
		assert.NoError(t, err)
		first := util.SliceMap(groups[0:3], getID)
		middle := util.SliceMap(groups[4:5], getID)
		middle = append(middle, groups[3].ID)
		end := util.SliceMap(groups[5:], getID)
		assertGroupOrder(t, parentGroup.ID, combineSlices(first, middle, end))
	})
	t.Run("ToFirst", func(t *testing.T) {
		parentGroup, groups := createParentGroup(t)
		err := group_model.MoveGroup(t.Context(), groups[3], parentGroup.ID, 0)
		assert.NoError(t, err)
		first := util.SliceMap(groups[0:3], getID)
		end := util.SliceMap(groups[4:], getID)
		onlyItem := []int64{groups[3].ID}
		assertGroupOrder(t, parentGroup.ID, combineSlices(onlyItem, first, end))
	})
	t.Run("ToLast", func(t *testing.T) {
		parentGroup, groups := createParentGroup(t)
		err := group_model.MoveGroup(t.Context(), groups[3], parentGroup.ID, 7)
		assert.NoError(t, err)
		first := util.SliceMap(groups[0:3], getID)
		end := util.SliceMap(groups[4:], getID)
		onlyItem := []int64{groups[3].ID}
		assertGroupOrder(t, parentGroup.ID, combineSlices(first, end, onlyItem))
	})
}
