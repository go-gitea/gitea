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
	OwnerID     int64 `xorm:"UNIQUE(s) index NOT NULL"`
	OwnerName   string
	Owner       *user_model.User    `xorm:"-"`
	LowerName   string              `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name        string              `xorm:"TEXT INDEX NOT NULL"`
	Description string              `xorm:"TEXT"`
	Visibility  structs.VisibleType `xorm:"NOT NULL DEFAULT 0"`
	Avatar      string              `xorm:"VARCHAR(64)"`

	ParentGroupID int64         `xorm:"DEFAULT NULL"`
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
	return g.doLoadSubgroups(ctx, recursive, AccessibleGroupCondition(doer, unit.TypeInvalid, perm.AccessModeRead), 0)
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
	return db.GetEngine(ctx).Where(AccessibleGroupCondition(user, unit.TypeInvalid, level).And(builder.Eq{"`repo_group`.id": g.ID})).Exist(&Group{})
}

func (g *Group) IsOwnedBy(ctx context.Context, userID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("team_user.uid = ?", userID).
		Join("INNER", "team_user", "team_user.team_id = repo_group_team.team_id").
		And("repo_group_team.access_mode = ?", perm.AccessModeOwner).
		And("repo_group_team.group_id = ?", g.ID).
		Table("repo_group_team").
		Exist()
}

func (g *Group) CanCreateIn(ctx context.Context, userID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("team_user.uid = ?", userID).
		Join("INNER", "team_user", "team_user.team_id = repo_group_team.team_id").
		And("repo_group_team.group_id = ?", g.ID).
		And("repo_group_team.can_create_in = ?", true).
		Table("repo_group_team").
		Exist()
}

func (g *Group) IsAdminOf(ctx context.Context, userID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("team_user.uid = ?", userID).
		Join("INNER", "team_user", "team_user.team_id = repo_group_team.team_id").
		And("repo_group_team.group_id = ?", g.ID).
		And("repo_group_team.access_mode >= ?", perm.AccessModeAdmin).
		Table("repo_group_team").
		Exist()
}

func (g *Group) ShortName(length int) string {
	return util.EllipsisDisplayString(g.Name, length)
}

func GetGroupByID(ctx context.Context, id int64) (*Group, error) {
	group := new(Group)

	has, err := db.GetEngine(ctx).ID(id).Get(group)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrGroupNotExist{id}
	}
	return group, nil
}

func GetGroupByRepoID(ctx context.Context, repoID int64) (*Group, error) {
	group := new(Group)
	_, err := db.GetEngine(ctx).
		In("id", builder.
			Select("group_id").
			From("repo").
			Where(builder.Eq{"id": repoID})).
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

	if ng != nil {
		if ng.OwnerID != group.OwnerID {
			return fmt.Errorf("group[%d]'s ownerID is not equal to new parent group[%d]'s owner ID", group.ID, ng.ID)
		}
	}

	group.ParentGroupID = newParent
	group.SortOrder = newSortOrder
	if _, err = sess.Table(group.TableName()).
		ID(group.ID).
		AllCols().
		Update(group); err != nil {
		return err
	}
	if group.ParentGroup != nil && newParent != 0 {
		group.ParentGroup = nil
		if err = group.LoadParentGroup(ctx); err != nil {
			return err
		}
	}
	return nil
}
