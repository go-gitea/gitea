package group

import (
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"context"
	"errors"
	"fmt"
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

func (Group) TableName() string { return "repo_group" }

func init() {
	db.RegisterModel(new(Group))
}

func (g *Group) doLoadSubgroups(ctx context.Context, recursive bool, currentLevel int) error {
	if currentLevel >= 20 {
		return ErrGroupTooDeep{
			g.ID,
		}
	}
	if g.Subgroups != nil {
		return nil
	}
	var err error
	g.Subgroups, err = FindGroups(ctx, &FindGroupsOptions{
		ParentGroupID: g.ID,
	})
	if err != nil {
		return err
	}
	if recursive {
		for _, group := range g.Subgroups {
			err = group.doLoadSubgroups(ctx, recursive, currentLevel+1)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *Group) LoadSubgroups(ctx context.Context, recursive bool) error {
	err := g.doLoadSubgroups(ctx, recursive, 0)
	return err
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

	has, err := db.GetEngine(ctx).ID(id).Get(g)
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
	if opts.ParentGroupID != 0 {
		cond = cond.And(builder.Eq{"parent_group_id": opts.ParentGroupID})
	} else {
		cond = cond.And(builder.IsNull{"parent_group_id"})
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

type ErrGroupNotExist struct {
	ID int64
}

// IsErrGroupNotExist checks if an error is a ErrCommentNotExist.
func IsErrGroupNotExist(err error) bool {
	var errGroupNotExist ErrGroupNotExist
	ok := errors.As(err, &errGroupNotExist)
	return ok
}

func (err ErrGroupNotExist) Error() string {
	return fmt.Sprintf("group does not exist [id: %d]", err.ID)
}

func (err ErrGroupNotExist) Unwrap() error {
	return util.ErrNotExist
}

type ErrGroupTooDeep struct {
	ID int64
}

func IsErrGroupTooDeep(err error) bool {
	var errGroupTooDeep ErrGroupTooDeep
	ok := errors.As(err, &errGroupTooDeep)
	return ok
}

func (err ErrGroupTooDeep) Error() string {
	return fmt.Sprintf("group has reached or exceeded the subgroup nesting limit [id: %d]", err.ID)
}
