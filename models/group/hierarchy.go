// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"
	"fmt"
	"slices"

	"gitea.dev/models/db"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

func groupHierarchyCTEBuilder(cond builder.Cond, maybeInitialCond ...builder.Cond) string {
	var descendantCTE,
		finalCTE string

	icond := builder.Cond(builder.Eq{"parent_group_id": 0})
	if len(maybeInitialCond) > 0 {
		if maybeInitialCond[0] != nil {
			icond = maybeInitialCond[0]
		}
	}

	firstPart := builder.Dialect(db.BuilderDialect()).Select("repo_group.*", "1 as depth", "name as path").
		From("repo_group").Where(icond)

	secondPart := builder.Dialect(db.BuilderDialect()).Select("r.*", "h.depth + 1", "concat(h.path, '/', r.name) as path").
		From("repo_group", "r").
		Join("INNER", "group_descendants h", "r.parent_group_id = h.id")

	firstSQL, _ := firstPart.ToBoundSQL()
	secondSQL, _ := secondPart.ToBoundSQL()
	descendantCTE = firstSQL + " UNION ALL " + secondSQL

	firstPart = builder.Dialect(db.BuilderDialect()).Select("group_descendants.*").
		From("group_descendants")

	if cond != nil {
		firstPart = firstPart.Where(cond)
	}
	secondPart = builder.Dialect(db.BuilderDialect()).Select("parent.*").
		From("group_descendants", "parent").
		Join("INNER", "group_hierarchy child", "child.parent_group_id = parent.id")

	firstSQL, _ = firstPart.ToBoundSQL()
	secondSQL, _ = secondPart.ToBoundSQL()
	finalCTE = "group_descendants AS (" + descendantCTE + "), group_hierarchy AS (" + firstSQL + " UNION ALL " + secondSQL + ")"
	return finalCTE
}

// GetParentGroupChain returns a slice containing a group and its ancestors
func GetParentGroupChain(ctx context.Context, groupID int64) (RepoGroupList, error) {
	groupList := make([]*Group, 0, NestingLimit)

	var (
		err              error
		recursiveKeyword string
	)
	if !setting.Database.Type.IsMSSQL() {
		recursiveKeyword = " recursive"
	}

	foundGroups := make([]*Group, 0)
	err = db.GetEngine(ctx).SQL(fmt.Sprintf(`WITH%s %s
SELECT group_hierarchy.* FROM group_hierarchy
ORDER BY group_hierarchy.depth ASC`,
		recursiveKeyword, groupHierarchyCTEBuilder(builder.Eq{"id": groupID}))).Find(&foundGroups)
	if err != nil {
		return nil, err
	}

	for i := range min(NestingLimit, len(foundGroups)) {
		groupList = append(groupList, foundGroups[i])
	}
	return groupList, nil
}

func GetParentGroupIDChain(ctx context.Context, groupID int64) ([]int64, error) {
	var (
		ids []int64
		err error
	)
	var recursiveKeyword string
	if !setting.Database.Type.IsMSSQL() {
		recursiveKeyword = " recursive"
	}
	err = db.GetEngine(ctx).SQL(fmt.Sprintf(`WITH%s %s
SELECT group_hierarchy.id FROM group_hierarchy
ORDER BY group_hierarchy.depth ASC`,
		recursiveKeyword, groupHierarchyCTEBuilder(builder.Eq{"id": groupID}))).Find(&ids)
	return ids, err
}

// ChildGroupCond returns a condition recursively matching a group and its descendants
func ChildGroupCond(ctx context.Context, firstParent int64, cond builder.Cond) ([]int64, error) {
	if firstParent < 0 {
		firstParent = 0
	}
	var (
		err error
		ids []int64
	)

	var recursiveKeyword string
	if !setting.Database.Type.IsMSSQL() {
		recursiveKeyword = "RECURSIVE "
	}

	err = db.GetEngine(ctx).SQL(fmt.Sprintf(`WITH %s%s SELECT g.id FROM group_hierarchy g ORDER BY id ASC`,
		recursiveKeyword,
		groupHierarchyCTEBuilder(nil,
			builder.And(builder.Eq{"parent_group_id": firstParent}, cond)))).
		Find(&ids)
	return ids, err
}

func CheckCycle(ctx context.Context, current, newParent int64) (bool, error) {
	if newParent == current {
		return true, nil
	}
	descendantIDs, err := ChildGroupCond(ctx, current, nil)
	if err != nil {
		return false, err
	}
	if _, has := slices.BinarySearch(descendantIDs, newParent); has {
		return true, nil
	}
	return false, nil
}

func MoveGroup(ctx context.Context, group *Group, newParent int64, newSortOrder int) error {
	sess := db.GetEngine(ctx)
	if newParent == group.ID {
		return util.NewInvalidArgumentErrorf("cannot move group %d under itself", group.ID)
	}

	hasCycle, err := CheckCycle(ctx, group.ID, newParent)
	if err != nil {
		return err
	}
	if hasCycle {
		return util.NewInvalidArgumentErrorf("cannot move group %d under one of its descendants", group.ID)
	}

	parentGroupChain, err := GetParentGroupChain(ctx, newParent)
	if err != nil {
		return err
	}

	if len(parentGroupChain) >= NestingLimit {
		return ErrGroupTooDeep{
			ID: group.ID,
		}
	}

	ng, err := GetGroupByID(ctx, newParent)
	if err != nil && !IsErrGroupNotExist(err) {
		return err
	}

	var siblings RepoGroupList
	var tmpSiblings RepoGroupList
	if ng != nil {
		if ng.OwnerID != group.OwnerID {
			return fmt.Errorf("group[%d]'s ownerID is not equal to new parent group[%d]'s owner ID", group.ID, ng.ID)
		}
		if err = ng.LoadSubgroups(ctx, false); err != nil {
			return err
		}
		tmpSiblings = ng.Subgroups
	} else if newParent <= 0 {
		tmpSiblings, err = FindGroups(ctx, &FindGroupsOptions{
			OwnerID:       group.OwnerID,
			ParentGroupID: 0,
		})
		if err != nil {
			return err
		}
	}

	for _, sibling := range tmpSiblings {
		if sibling.ID != group.ID {
			siblings = append(siblings, sibling)
		}
	}
	siblings = util.SliceInsert(siblings, group, newSortOrder)

	err = group.LoadOwner(ctx)
	if err != nil {
		return err
	}

	oldParentID := group.ParentGroupID

	group.OwnerName = group.Owner.Name
	group.ParentGroupID = newParent
	group.SortOrder = newSortOrder
	for i, gg := range siblings {
		gg.SortOrder = i
		if _, err = sess.Table(group.TableName()).
			ID(gg.ID).
			AllCols().
			Update(gg); err != nil {
			return err
		}
	}

	group.ParentGroup = nil
	if newParent != 0 {
		if err = group.LoadParentGroup(ctx); err != nil {
			return err
		}
	}

	// re-index items in old parent if different
	if newParent != oldParentID {
		oldItems, err := FindGroups(ctx, &FindGroupsOptions{
			ParentGroupID: oldParentID,
		})
		if err != nil {
			return err
		}
		for i, item := range oldItems {
			item.SortOrder = i
			if _, err = sess.Table(group.TableName()).
				ID(item.ID).
				Cols("sort_order").
				Update(item); err != nil {
				return err
			}
		}
	}

	return nil
}
