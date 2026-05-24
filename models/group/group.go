// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"
	"fmt"
	"net/url"
	"slices"

	"gitea.dev/models/db"
	org_model "gitea.dev/models/organization"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	giturl "gitea.dev/modules/git/url"
	"gitea.dev/modules/log"
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
	return setting.AppSubURL + "/" + url.PathEscape(g.OwnerName) + "/groups/" + g.FullPath()
}

func (g *Group) OrgGroupLink() string {
	return setting.AppSubURL + "/org/" + url.PathEscape(g.OwnerName) + "/groups/" + g.FullPath()
}

func (g *Group) UserGroupLink() string {
	return setting.AppSubURL + "/" + url.PathEscape(g.OwnerName) + "/-/groups/" + g.FullPath()
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

func (g *Group) FullPath(ctxs ...context.Context) string {
	path, _ := PathByID(g.ID, ctxs...)
	return path
}

func (g *Group) CanAccess(ctx context.Context, user *user_model.User) (bool, error) {
	return g.CanAccessAtLevel(ctx, user, perm.AccessModeRead)
}

func (g *Group) CanAccessAtLevel(ctx context.Context, user *user_model.User, level perm.AccessMode) (bool, error) {
	return g.CanAccessUnitAtLevel(ctx, user, unit.TypeInvalid, level)
}

func (g *Group) CanAccessUnitAtLevel(ctx context.Context, user *user_model.User, u unit.Type, level perm.AccessMode) (bool, error) {
	if user != nil {
		if user.IsAdmin {
			return true, nil
		}
		ownedBy, err := g.IsOwnedBy(ctx, user.ID)
		if err != nil {
			return false, err
		}
		if ownedBy {
			return true, nil
		}
		if level >= perm.AccessModeAdmin {
			return g.IsAdminOf(ctx, user.ID)
		}
		if level >= perm.AccessModeWrite {
			return g.CanCreateIn(ctx, user.ID)
		}
	}
	orCond := builder.Or(AccessibleGroupCondition(user))
	isMember, err := g.IsMemberOf(ctx, user)
	if err != nil {
		return false, err
	}
	if level == perm.AccessModeRead && !isMember {
		orCond = orCond.And(builder.Eq{"`repo_group`.visibility": structs.VisibleTypePublic})
	}
	return db.GetEngine(ctx).Table(g.TableName()).Where(builder.And(builder.Eq{"`repo_group`.id": g.ID}, orCond)).Exist()
}

func (g *Group) IsOwnedBy(ctx context.Context, userID int64) (bool, error) {
	owner, err := user_model.GetUserByID(ctx, g.OwnerID)
	if err != nil {
		return false, err
	}
	if owner.Type == user_model.UserTypeIndividual {
		return owner.ID == userID, nil
	}
	org, err := org_model.GetOrgByID(ctx, g.OwnerID)
	if err != nil {
		return false, err
	}
	return org.IsOwnedBy(ctx, userID)
}

func (g *Group) IsMemberOf(ctx context.Context, user *user_model.User) (bool, error) {
	if user == nil {
		return false, nil
	}
	owner, err := user_model.GetUserByID(ctx, g.OwnerID)
	if err != nil {
		return false, err
	}
	if owner.Type == user_model.UserTypeIndividual {
		return owner.ID == user.ID, nil
	}
	org, err := org_model.GetOrgByID(ctx, g.OwnerID)
	if err != nil {
		return false, err
	}
	return org.IsOrgMember(ctx, user.ID)
}

func (g *Group) CanCreateIn(ctx context.Context, userID int64) (bool, error) {
	can, err := org_model.CanCreateOrgRepo(ctx, g.OwnerID, userID)
	if err != nil {
		return false, err
	}
	return can || g.OwnerID == userID, nil
}

func (g *Group) IsAdminOf(ctx context.Context, userID int64) (bool, error) {
	owner, err := user_model.GetUserByID(ctx, g.OwnerID)
	if err != nil {
		return false, err
	}
	if owner.Type == user_model.UserTypeIndividual {
		return owner.ID == userID, nil
	}
	org, err := org_model.GetOrgByID(ctx, g.OwnerID)
	if err != nil {
		return false, err
	}
	return org.IsOrgAdmin(ctx, userID)
}

func (g *Group) ShortName(length int) string {
	return util.EllipsisDisplayString(g.Name, length)
}

func (g *Group) IsPrivateBecauseOfParentPermissions(ctx context.Context, user *user_model.User) (bool, error) {
	cond := AccessibleParentGroupCond(ctx, "`repo_group`.`id`", user)
	has, err := db.GetEngine(ctx).Where(cond.And(builder.Eq{
		"`repo_group`.id": g.ID,
	})).Table(g.TableName()).Exist()
	return !has, err
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

func PathByID(gid int64, ctxs ...context.Context) (string, error) {
	if gid <= 0 {
		return "", nil
	}
	ctx := util.OptionalArg(ctxs, context.TODO())
	var strs []string
	err := db.GetEngine(ctx).SQL(fmt.Sprintf(`%s
select path from repo_groups where id = ?`, groupPathCTEBuilder()), gid).Find(&strs)
	if err != nil {
		log.Error("unable to find group path: %w", err)
		return "", err
	}
	if len(strs) < 1 {
		return "", nil
	}
	return strs[0], nil
}

func groupPathCTEBuilder() string {
	var recursiveKeyword string
	if !setting.Database.Type.IsMSSQL() {
		recursiveKeyword = " RECURSIVE"
	}
	return fmt.Sprintf(`WITH%s repo_groups AS (
    SELECT
        repo_group.*,
       lower_name AS path
    FROM repo_group
    WHERE parent_group_id = 0

    UNION ALL

    SELECT
        g.*,
        concat(p.path, '/', g.lower_name) as path
    FROM repo_group g
    INNER JOIN repo_groups p ON g.parent_group_id = p.id
)`, recursiveKeyword)
}

func GetGroupByPathname(ctx context.Context, owner, pathname string) (*Group, error) {
	pathname = giturl.NormalizeGroupPath(pathname)
	rawSQL := groupPathCTEBuilder() + `
SELECT *
FROM repo_groups
WHERE owner_name = ? and path = ?;`
	g := new(Group)
	has, err := db.GetEngine(ctx).SQL(rawSQL, owner, pathname).Get(g)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrGroupNotExist{Path: pathname}
	}

	return g, nil
}

func IDByPathname(ctx context.Context, ownerID int64, pathname string) (int64, error) {
	pathname = giturl.NormalizeGroupPath(pathname)
	if pathname == "" {
		return 0, nil
	}
	owner, err := user_model.GetUserByID(ctx, ownerID)
	if err != nil {
		return 0, nil
	}

	rg, err := GetGroupByPathname(ctx, owner.LowerName, pathname)
	if err != nil {
		return 0, err
	}
	if rg == nil {
		return 0, nil
	}
	return rg.ID, nil
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

func ParentGroupCondByRepoID(ctx context.Context, repoID int64, idStr string) builder.Cond {
	g, err := GetGroupByRepoID(ctx, repoID)
	if err != nil {
		return builder.In(idStr)
	}
	return ParentGroupCond(ctx, idStr, g.ID)
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

// GetParentGroupChain returns a slice containing a group and its ancestors
func GetParentGroupChain(ctx context.Context, groupID int64) (RepoGroupList, error) {
	groupList := make([]*Group, 0, NestingLimit)
	currentGroupID := groupID
	for {
		if currentGroupID < 1 {
			break
		}
		if len(groupList) >= NestingLimit {
			return nil, ErrGroupTooDeep{currentGroupID}
		}
		currentGroup, err := GetGroupByID(ctx, currentGroupID)
		if err != nil {
			return nil, err
		}
		groupList = append(groupList, currentGroup)
		currentGroupID = currentGroup.ParentGroupID
	}
	slices.Reverse(groupList)
	return groupList, nil
}

func GetParentGroupIDChain(ctx context.Context, groupID int64) ([]int64, error) {
	var ids []int64
	groupList, err := GetParentGroupChain(ctx, groupID)
	if err != nil {
		return nil, err
	}
	ids = util.SliceMap(groupList, func(g *Group) int64 {
		return g.ID
	})
	return ids, err
}

func groupHierarchyCTEBuilder(cond builder.Cond) string {
	firstPart := builder.Dialect(db.BuilderDialect()).Select("repo_group.*", "1 as depth", "id as ancestor_id").
		From("repo_group")
	if cond != nil {
		firstPart = firstPart.Where(cond)
	}
	secondPart := builder.Dialect(db.BuilderDialect()).Select("r.*", "h.depth + 1", "h.ancestor_id").
		From("repo_group", "r").
		Join("INNER", "group_hierarchy h", "r.parent_group_id = h.id")

	firstSQL, _ := firstPart.ToBoundSQL()
	secondSQL, _ := secondPart.ToBoundSQL()
	return firstSQL + " UNION ALL " + secondSQL
}

func AccessibleParentGroupCond(ctx context.Context, idStr string, user *user_model.User) builder.Cond {
	accessibleCond := AccessibleGroupCondition(user)
	unionSQL := groupHierarchyCTEBuilder(
		accessibleCond.And(builder.Eq{
			"parent_group_id": 0,
		}))
	var recursiveKeyword string

	if !setting.Database.Type.IsMSSQL() {
		recursiveKeyword = "RECURSIVE "
	}

	e := db.GetEngine(ctx)
	sql := "WITH " + recursiveKeyword + "group_hierarchy AS (" + unionSQL + ")"
	var ids []int64
	err := e.SQL(sql + " select id from group_hierarchy").Find(&ids)
	if err != nil {
		return builder.NotIn(idStr)
	}
	return builder.In(idStr, ids)
}

// ParentGroupCond returns a condition matching a group and its ancestors
func ParentGroupCond(ctx context.Context, idStr string, groupID int64) builder.Cond {
	groupList, err := GetParentGroupIDChain(ctx, groupID)
	if err != nil {
		log.Info("Error building group cond: %w", err)
		return builder.NotIn(idStr)
	}
	return builder.In(idStr, groupList)
}

// ChildGroupCond returns a condition recursively matching a group and its descendants
func ChildGroupCond(ctx context.Context, firstParent int64, cond builder.Cond) ([]int64, error) {
	if firstParent < 0 {
		firstParent = 0
	}
	var (
		filter string
		err    error
	)

	if cond != nil {
		var boundFilter string
		boundFilter, err = builder.ToBoundSQL(cond)
		if err == nil {
			filter = "AND (" + boundFilter + ")"
		}
	}

	var ids []int64

	var recursiveKeyword string
	if !setting.Database.Type.IsMSSQL() {
		recursiveKeyword = "RECURSIVE "
	}

	err = db.GetEngine(ctx).SQL(fmt.Sprintf(`WITH %srepo_groups AS (
		SELECT * from repo_group
		WHERE parent_group_id = %d %s

		UNION ALL

		SELECT subgroup.*
		FROM repo_group subgroup
		JOIN repo_groups g ON g.id = subgroup.parent_group_id
	) SELECT g.id FROM repo_groups g ORDER BY id ASC`, recursiveKeyword, firstParent, filter)).Find(&ids)
	return ids, err
}

func UpdateGroup(ctx context.Context, group *Group) error {
	sess := db.GetEngine(ctx)
	_, err := sess.Table(group.TableName()).ID(group.ID).Update(group)
	return err
}

func MoveGroup(ctx context.Context, group *Group, newParent int64, newSortOrder int) error {
	sess := db.GetEngine(ctx)
	if newParent == group.ID {
		return util.NewInvalidArgumentErrorf("cannot move group %d under itself", group.ID)
	}

	descendantIDs, err := ChildGroupCond(ctx, group.ID, nil)
	if err != nil {
		return err
	}
	if _, has := slices.BinarySearch(descendantIDs, newParent); has {
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
