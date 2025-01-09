package group

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"xorm.io/builder"
)

// Group represents a group of repositories for a user or organization
type Group struct {
	ID          int64 `xorm:"pk autoincr"`
	OwnerID     int64 `xorm:"UNIQUE(s) index NOT NULL"`
	OwnerName   string
	Owner       *user_model.User `xorm:"-"`
	LowerName   string           `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name        string           `xorm:"INDEX NOT NULL"`
	FullName    string           `xorm:"TEXT"` // displayed in places like navigation menus
	Description string           `xorm:"TEXT"`
	IsPrivate   bool
	Visibility  structs.VisibleType `xorm:"NOT NULL DEFAULT 0"`
	Avatar      string              `xorm:"VARCHAR(64)"`

	ParentGroupID int64     `xorm:"INDEX DEFAULT NULL"`
	ParentGroup   *Group    `xorm:"-"`
	Subgroups     GroupList `xorm:"-"`
}

// GroupLink returns the link to this group
func (g *Group) GroupLink() string {
	return setting.AppSubURL + "/" + url.PathEscape(g.OwnerName) + "/groups/" + strconv.FormatInt(g.ID, 10)
}

func (Group) TableName() string { return "repo_group" }

func init() {
	db.RegisterModel(new(Group))
	db.RegisterModel(new(GroupTeam))
	db.RegisterModel(new(GroupUnit))
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
	g.Subgroups, err = FindGroupsByCond(ctx, cond, g.ID)
	if err != nil {
		return err
	}
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
	return g.doLoadSubgroups(ctx, recursive, AccessibleGroupCondition(doer, unit.TypeInvalid), 0)
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

func (g *Group) GetGroupByID(ctx context.Context, id int64) (*Group, error) {
	group := new(Group)

	has, err := db.GetEngine(ctx).ID(id).Get(group)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrGroupNotExist{id}
	}
	return group, nil
}

type FindGroupsOptions struct {
	db.ListOptions
	OwnerID       int64
	ParentGroupID int64
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
			builder.Select("group_team.group_id").
				From("group_team").
				Where(builder.Eq{"team_user.uid": opts.ActorID}).
				Join("INNER", "team_user", "team_user.team_id = group_team.team_id").
				And(builder.Eq{"group_team.can_create_in": true})))
	}
	if opts.Name != "" {
		cond = cond.And(builder.Eq{"lower_name": opts.Name})
	}
	return cond
}

func FindGroups(ctx context.Context, opts *FindGroupsOptions) (GroupList, error) {
	sess := db.GetEngine(ctx).Where(opts.ToConds())
	if opts.Page > 0 {
		sess = db.SetSessionPagination(sess, opts)
	}
	groups := make([]*Group, 0, 10)
	return groups, sess.
		Asc("repo_group.id").
		Find(&groups)
}

func FindGroupsByCond(ctx context.Context, cond builder.Cond, parentGroupID int64) (GroupList, error) {
	if parentGroupID > 0 {
		cond = cond.And(builder.Eq{"repo_group.id": parentGroupID})
	} else {
		cond = cond.And(builder.IsNull{"repo_group.id"})
	}
	sess := db.GetEngine(ctx).Where(cond)
	groups := make([]*Group, 0)
	return groups, sess.
		Asc("repo_group.id").
		Find(&groups)
}

// GetParentGroupChain returns a slice containing a group and its ancestors
func GetParentGroupChain(ctx context.Context, groupID int64) (GroupList, error) {
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

func GetParentGroupIDChain(ctx context.Context, groupID int64) (ids []int64, err error) {
	groupList, err := GetParentGroupChain(ctx, groupID)
	if err != nil {
		return nil, err
	}
	ids = util.SliceMap(groupList, func(g *Group) int64 {
		return g.ID
	})
	return
}

// ParentGroupCond returns a condition matching a group and its ancestors
func ParentGroupCond(idStr string, groupID int64) builder.Cond {
	groupList, err := GetParentGroupIDChain(db.DefaultContext, groupID)
	if err != nil {
		log.Info("Error building group cond: %w", err)
		return builder.NotIn(idStr)
	}
	return builder.In(idStr, groupList)
}

func MoveGroup(ctx context.Context, group *Group, newParent int64, newSortOrder int) error {
	sess := db.GetEngine(ctx)
	ng, err := GetGroupByID(ctx, newParent)
	if err != nil {
		return err
	}
	if ng.OwnerID != group.OwnerID {
		return fmt.Errorf("group[%d]'s ownerID is not equal to new paretn group[%d]'s owner ID", group.ID, ng.ID)
	}
	group.ParentGroupID = newParent
	group.SortOrder = newSortOrder
	if _, err = sess.Table(group.TableName()).
		Where("id = ?", group.ID).
		MustCols("parent_group_id").
		Update(group, &Group{
			ID: group.ID,
		}); err != nil {
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
