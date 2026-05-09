// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

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

func (Group) TableName() string { return "repo_group" }

func init() {
	db.RegisterModel(new(Group))
	db.RegisterModel(new(RepoGroupTeam))
	db.RegisterModel(new(RepoGroupUnit))
}

func (g *Group) doLoadSubgroups(ctx context.Context, recursive bool, cond builder.Cond, currentLevel int) error {
	if currentLevel >= 20 {
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

func (g *Group) LoadAccessibleSubgroups(ctx context.Context, recursive bool, doer *user_model.User) error {
	return g.doLoadSubgroups(ctx, recursive, AccessibleGroupCondition(doer, g.OwnerID, unit.TypeInvalid, perm.AccessModeRead), 0)
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

func (g *Group) CanAccess(ctx context.Context, user *user_model.User) (bool, error) {
	return g.CanAccessAtLevel(ctx, user, perm.AccessModeRead)
}

func (g *Group) CanAccessAtLevel(ctx context.Context, user *user_model.User, level perm.AccessMode) (bool, error) {
	return g.CanAccessUnitAtLevel(ctx, user, unit.TypeInvalid, level)
}

func (g *Group) CanAccessUnitAtLevel(ctx context.Context, user *user_model.User, u unit.Type, level perm.AccessMode) (bool, error) {
	if user != nil {
		ownedBy, err := g.IsOwnedBy(ctx, user.ID)
		if err != nil {
			return false, err
		}
		if ownedBy {
			return true, nil
		}
	}
	orCond := builder.Or(AccessibleGroupCondition(user, g.OwnerID, u, level))
	if level == perm.AccessModeRead {
		orCond = orCond.Or(builder.Eq{"`repo_group`.visibility": structs.VisibleTypePublic})
	}
	return db.GetEngine(ctx).Table(g.TableName()).Where(builder.And(builder.Eq{"`repo_group`.id": g.ID}, orCond)).Exist()
}

func (g *Group) IsOwnedBy(ctx context.Context, userID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where(
			builder.Or(
				UserOrgTeamPermCond("`repo_group`.id", userID, g.OwnerID, perm.AccessModeOwner),
				universalGroupPermBuilder("`repo_group`.id", userID, g.OwnerID, false)).
				And(builder.Eq{"`repo_group`.id": g.ID})).
		Table(g.TableName()).
		Exist()
}

func (g *Group) CanCreateIn(ctx context.Context, userID int64) (bool, error) {
	cond := builder.Eq{
		"team_user.uid":                 userID,
		"repo_group_team.group_id":      g.ID,
		"repo_group_team.can_create_in": true,
	}

	isAdmin, err := g.IsAdminOf(ctx, userID)
	if err != nil {
		return false, err
	}

	res, err := db.GetEngine(ctx).
		Join("INNER", "team_user", "team_user.team_id = repo_group_team.team_id").
		Where(cond).
		Table("repo_group_team").
		Exist()
	if err != nil {
		return false, err
	}
	return isAdmin || res, nil
}

func (g *Group) IsAdminOf(ctx context.Context, userID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where(
			builder.Or(
				UserOrgTeamPermCond("`repo_group`.id", userID, g.OwnerID, perm.AccessModeAdmin),
				universalGroupPermBuilder("`repo_group`.id", userID, g.OwnerID, false)).
				And(builder.Eq{"`repo_group`.id": g.ID})).
		Table(g.TableName()).
		Exist()
}

func (g *Group) ShortName(length int) string {
	return util.EllipsisDisplayString(g.Name, length)
}

func (g *Group) IsPrivateBecauseOfParentPermissions(ctx context.Context, user *user_model.User) (bool, error) {
	cond := AccessibleParentGroupCond(ctx, "`repo_group`.`id`", g.ID, user)
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
		return nil, ErrGroupNotExist{id}
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
	CanCreateIn   optional.Option[bool]
	ActorID       int64
	Name          string
}

func (opts FindGroupsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OwnerID != 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if opts.ParentGroupID > 0 {
		cond = cond.And(builder.Eq{"parent_group_id": opts.ParentGroupID})
	} else if opts.ParentGroupID == 0 {
		cond = cond.And(builder.Eq{"parent_group_id": 0})
	}
	if opts.CanCreateIn.Has() && opts.ActorID > 0 {
		cond = cond.And(builder.In("id",
			builder.Select("repo_group_team.group_id").
				From("repo_group_team").
				Where(builder.Eq{"team_user.uid": opts.ActorID}).
				Join("INNER", "team_user", "team_user.team_id = repo_group_team.team_id").
				And(builder.Eq{"repo_group_team.can_create_in": true})))
	}
	if opts.Name != "" {
		cond = cond.And(builder.Eq{"lower_name": opts.Name})
	}
	return cond
}

func FindGroups(ctx context.Context, opts *FindGroupsOptions) (RepoGroupList, error) {
	sess := db.GetEngine(ctx).Where(opts.ToConds())
	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, opts)
	}

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
	groupList := make([]*Group, 0, 20)
	currentGroupID := groupID
	for {
		if currentGroupID < 1 {
			break
		}
		if len(groupList) >= 20 {
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

func groupHierarchyCTEBuilder(cond builder.Cond) builder.Cond {
	firstPart := builder.Select(fmt.Sprintf("repo_group.*"), "1 as depth").
		From("repo_group").
		Where(builder.And(builder.Eq{
			"parent_group_id": 0,
		}, cond))
	secondPart := builder.Select("r.*", "h.depth + 1").
		From("repo_group", "r").
		Join("INNER", "group_hierarchy h", "r.parent_group_id = h.id")

	firstSql, _ := firstPart.ToBoundSQL()
	secondSql, _ := secondPart.ToBoundSQL()
	return builder.Expr(firstSql + " UNION ALL " + secondSql)
}

func AccessibleParentGroupCond(ctx context.Context, idStr string, groupID int64, user *user_model.User) builder.Cond {
	owner, err := GetOwnerByGroupID(ctx, groupID)
	if err != nil {
		return builder.Exists(builder.Select("1 as dummy").Where(builder.Eq{
			"dummy": 1,
		}))
	}
	accessibleCond := AccessibleGroupCondition(user, owner.ID, unit.TypeInvalid, perm.AccessModeRead)
	unionBldr := groupHierarchyCTEBuilder(accessibleCond)
	unionSql, err := builder.ToBoundSQL(unionBldr)
	if err != nil {

	}
	s := db.GetEngine(ctx)
	s.SQL("WITH RECURSIVE group_hierarchy AS ("+unionSql+") SELECT id from group_hierarchy", unionSql)
	var g []*Group
	err = s.Find(&g)
	if err != nil {
		log.Info("%s", err.Error())
	}
	return builder.In(idStr, builder.Expr("(WITH RECURSIVE group_hierarchy AS ("+unionSql+") SELECT id from group_hierarchy)"))
	//db.GetEngine(ctx).SQL()
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

func UpdateGroup(ctx context.Context, group *Group) error {
	sess := db.GetEngine(ctx)
	_, err := sess.Table(group.TableName()).ID(group.ID).Update(group)
	return err
}

func MoveGroup(ctx context.Context, group *Group, newParent int64, newSortOrder int) error {
	sess := db.GetEngine(ctx)
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
		siblings = append(append(ng.Subgroups[0:min(newSortOrder, len(ng.Subgroups))], group), ng.Subgroups[newSortOrder:]...)
	} else if newParent <= 0 {
		tmpSiblings, err = FindGroups(ctx, &FindGroupsOptions{
			OwnerID:       group.OwnerID,
			ParentGroupID: 0,
		})
		tmpSiblings2 := make(RepoGroupList, newSortOrder)
		copy(tmpSiblings2, tmpSiblings[0:newSortOrder])
		tmpSiblings2 = append(tmpSiblings2, group)

		siblings = append(tmpSiblings2, tmpSiblings[newSortOrder:]...)
	}
	parentGroupChain, err := GetParentGroupChain(ctx, newParent)
	if err != nil {
		return err
	}
	if len(parentGroupChain) >= 20 {
		return ErrGroupTooDeep{
			ID: group.ID,
		}
	}
	err = group.LoadOwner(ctx)
	if err != nil {
		return err
	}
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

	if group.ParentGroup != nil && newParent != 0 {
		group.ParentGroup = nil
		if err = group.LoadParentGroup(ctx); err != nil {
			return err
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
	if !has {
		return nil, user_model.ErrUserNotExist{}
	}
	return user, err
}
