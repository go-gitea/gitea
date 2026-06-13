// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

const NestingLimit = 20

// Group represents a group of repositories for a user or organization
type Group struct {
	ID          int64 `xorm:"pk autoincr"`
	OwnerID     int64 `xorm:"INDEX NOT NULL"`
	OwnerName   string
	Owner       *user_model.User    `xorm:"-"`
	LowerName   string              `xorm:"TEXT NOT NULL"`
	Name        string              `xorm:"TEXT NOT NULL"`
	Description string              `xorm:"TEXT"`
	Visibility  structs.VisibleType `xorm:"NOT NULL DEFAULT 0"`
	Avatar      string              `xorm:"VARCHAR(64)"`

	ParentGroupID int64         `xorm:"INDEX DEFAULT NULL"`
	ParentGroup   *Group        `xorm:"-"`
	Subgroups     RepoGroupList `xorm:"-"`

	SortOrder int `xorm:"INDEX"`
}

// GroupLink returns the link to this group
func (g *Group) GroupLink() string {
	return setting.AppSubURL + "/" + url.PathEscape(g.OwnerName) + "/groups/" + strconv.FormatInt(g.ID, 10)
}

func (g *Group) OrgGroupLink() string {
	return setting.AppSubURL + "/org/" + url.PathEscape(g.OwnerName) + "/groups/" + strconv.FormatInt(g.ID, 10)
}

func (g *Group) UserGroupLink() string {
	return setting.AppSubURL + "/" + url.PathEscape(g.OwnerName) + "/-/groups/" + strconv.FormatInt(g.ID, 10)
}

func (Group) TableName() string { return "repo_group" }

func init() {
	db.RegisterModel(new(Group))
}

func (g *Group) doLoadSubgroups(ctx context.Context, recursive bool, cond builder.Cond, currentLevel int) error {
	if currentLevel >= NestingLimit {
		return ErrGroupTooDeep{
			g.ID,
		}
	}
	if g.Subgroups != nil {
		return nil
	}
	var err error
	g.Subgroups, err = FindGroupsByCond(ctx, &FindGroupsOptions{
		ParentGroupID: g.ID,
	}, cond)
	if err != nil {
		return err
	}
	slices.SortStableFunc(g.Subgroups, func(a, b *Group) int {
		return a.SortOrder - b.SortOrder
	})
	if recursive {
		for _, group := range g.Subgroups {
			err = group.doLoadSubgroups(ctx, recursive, cond, currentLevel+1)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *Group) LoadSubgroups(ctx context.Context, recursive bool) error {
	fgo := &FindGroupsOptions{
		ParentGroupID: g.ID,
	}
	return g.doLoadSubgroups(ctx, recursive, fgo.ToConds(), 0)
}

func (g *Group) LoadAccessibleSubgroups(ctx context.Context, recursive bool, doer *user_model.User, requireMember bool) error {
	cond := AccessibleGroupCondition(doer)
	if requireMember {
		cond = builder.And(MemberCond("`repo_group`.parent_group_id", g.ID, doer), cond)
	}
	return g.doLoadSubgroups(ctx, recursive, cond, 0)
}

func (g *Group) LoadAttributes(ctx context.Context) error {
	err := g.LoadOwner(ctx)
	if err != nil {
		return err
	}
	return g.LoadParentGroup(ctx)
}

func (g *Group) LoadParentGroup(ctx context.Context) error {
	if g.ParentGroup != nil {
		return nil
	}
	if g.ParentGroupID == 0 {
		return nil
	}
	parentGroup, err := GetGroupByID(ctx, g.ParentGroupID)
	if err != nil {
		return err
	}
	g.ParentGroup = parentGroup
	return nil
}

func (g *Group) LoadOwner(ctx context.Context) error {
	if g.Owner != nil {
		return nil
	}
	var err error
	g.Owner, err = user_model.GetUserByID(ctx, g.OwnerID)
	return err
}

func (g *Group) ShortName(length int) string {
	return util.EllipsisDisplayString(g.Name, length)
}

func GetGroupByIDAndCond(ctx context.Context, id int64, cond builder.Cond) (*Group, error) {
	group := new(Group)

	has, err := db.GetEngine(ctx).
		Where(cond.And(builder.Eq{"`repo_group`.id": id})).Get(group)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrGroupNotExist{ID: id}
	}
	return group, nil
}

func GetGroupByID(ctx context.Context, id int64) (*Group, error) {
	return GetGroupByIDAndCond(ctx, id, builder.Expr("1 = 1"))
}

func GetGroupByRepoID(ctx context.Context, repoID int64) (*Group, error) {
	group := new(Group)
	_, err := db.GetEngine(ctx).
		Join("INNER", "repository", "repository.group_id = repo_group.id").
		Where(builder.Eq{"repository.`id`": repoID}).
		Get(group)
	return group, err
}

type FindGroupsOptions struct {
	db.ListOptions
	OwnerID       int64
	ParentGroupID int64
	ActorID       int64
	Name          string
}

func (opts FindGroupsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OwnerID != 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if opts.ParentGroupID >= 0 {
		cond = cond.And(builder.Eq{"parent_group_id": opts.ParentGroupID})
	}
	if opts.Name != "" {
		cond = cond.And(builder.Eq{"lower_name": opts.Name})
	}
	return cond
}

func FindGroups(ctx context.Context, opts *FindGroupsOptions) (RepoGroupList, error) {
	sess := db.GetEngine(ctx)
	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, opts)
	}
	sess = sess.Where(opts.ToConds())

	groups := make([]*Group, 0, 10)
	return groups, sess.
		Asc("repo_group.sort_order").
		Find(&groups)
}

func findGroupsByCond(ctx context.Context, opts *FindGroupsOptions, cond builder.Cond) db.Engine {
	if opts.Page <= 0 {
		opts.Page = 1
	}

	sess := db.GetEngine(ctx).Where(cond.And(opts.ToConds()))
	if opts.PageSize > 0 {
		sess = sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	return sess.Asc("sort_order")
}

func FindGroupsByCond(ctx context.Context, opts *FindGroupsOptions, cond builder.Cond) (RepoGroupList, error) {
	defaultSize := 50
	if opts.PageSize > 0 {
		defaultSize = opts.PageSize
	}
	sess := findGroupsByCond(ctx, opts, cond)
	groups := make([]*Group, 0, defaultSize)
	if err := sess.Find(&groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func CountGroups(ctx context.Context, opts *FindGroupsOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.ToConds()).Count(new(Group))
}

func UpdateGroupOwnerName(ctx context.Context, oldUser, newUser string) error {
	if _, err := db.GetEngine(ctx).Exec("UPDATE `repo_group` SET owner_name=? WHERE owner_name=?", newUser, oldUser); err != nil {
		return fmt.Errorf("change group owner name: %w", err)
	}
	return nil
}

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
		recursiveKeyword, groupHierarchyCTEBuilder(builder.Eq{"id": groupID}, nil))).Find(&ids)
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

func UpdateGroup(ctx context.Context, group *Group) error {
	sess := db.GetEngine(ctx)
	_, err := sess.Table(group.TableName()).ID(group.ID).Update(group)
	return err
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

func GetOwnerByGroupID(ctx context.Context, groupID int64) (*user_model.User, error) {
	e := db.GetEngine(ctx)
	tableName := "repo_group"
	user := new(user_model.User)
	has, err := e.Join("INNER", tableName, fmt.Sprintf("`%s`.owner_id = `user`.`id`", tableName)).
		Where(builder.Eq{fmt.Sprintf("`%s`.id", tableName): groupID}).Get(user)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, user_model.ErrUserNotExist{}
	}
	return user, err
}
