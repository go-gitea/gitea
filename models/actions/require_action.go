// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// WIP RequireAction

package actions

import (
	"context"
	"errors"
	"fmt"

	org_model "code.gitea.io/gitea/models/organization"
	repo_model "code.gitea.io/gitea/models/repo"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

type RequireAction struct {
	ID          int64                   `xorm:"pk autoincr"`
	OwnerID			int64										`xorm:"index"` // should be the Org, Global?
	Org         *org_model.Organization `xorm:"-"`
	OrgID       int64                   `xorm:"index"`
	Repo        *repo_model.Repository  `xorm:"-"`
	RepoID      int64                   `xorm:"index"`
	Name        string                  `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data        string                  `xorm:"LONGTEXT NOT NULL"`
	Link        string                  `xorm:"LONGTEXT NOT NULL"`
	CreatedUnix timeutil.TimeStamp      `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp      `xorm:"updated"`

	RepoRange   string                  // glob match which repositories could use this runner
}

func (v *RequireAction) Validate() error {
	if v.RepoID != 0 && v.OrgID != 0 && v.Data != "" {
		return nil
	} else {
		return errors.New("the action workflow need repo id , org id and the name, something missing")
	}
}

func init() {
	db.RegisterModel(new(RequireAction))
}

// ErrRequireActionNotFound represents a "require action not found" error.
type ErrRequireActionNotFound struct {
	Name string
}

func (err ErrRequireActionNotFound) Error() string {
	return fmt.Sprintf("require action was not found [name: %s]", err.Name)
}

func (err ErrRequireActionNotFound) Unwrap() error {
	return util.ErrNotExist
}



type FindRequireActionOptions struct {
	db.ListOptions
	RequireActionID	int64
	OrgID  					int64
}

func (opts FindRequireActionOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OrgID > 0 {
		cond = cond.And(builder.Eq{"org_id": opts.OrgID})
	}
	if opts.RequireActionID > 0 {
		cond = cond.And(builder.Eq{"id": opts.RequireActionID})
	}
	return cond
}

// LoadAttributes loads the attributes of the require action
func (r *RequireAction) LoadAttributes(ctx context.Context) error {
	// place holder for now.
	return nil
}

func InsertRequireAction(ctx context.Context, orgID int64, repoID int64, data string, link string) (*RequireAction, error) {
	require_action := &RequireAction{
		OrgID: 	 orgID,
		RepoID:  repoID,
		Data:    data,
		Link:    link,
	}
	if err := require_action.Validate(); err != nil {
		return require_action, err
	}
	return require_action, db.Insert(ctx, require_action)
}

// Editable checks if the require action is editable by the user
func (r *RequireAction) Editable(orgID int64, repoID int64) bool {
	if orgID == 0 && repoID == 0 {
		return true
	}
	if orgID > 0 && r.OrgID == orgID {
		return true
	}
	return repoID > 0 && r.RepoID == repoID
}

func GetRequireActionByID(ctx context.Context, require_actionID int64) (*RequireAction, error) {
	var require_action RequireAction
	has, err := db.GetEngine(ctx).Where("id=?", require_actionID).Get(&require_action)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("require action with id %d: %w", require_actionID, util.ErrNotExist)
	}
	return &require_action, nil
}

func UpdateRequireAction(ctx context.Context, require_action *RequireAction) (bool, error) {
	count, err := db.GetEngine(ctx).ID(require_action.ID).Cols("name", "data", "link").
		Update(&RequireAction{
			Name: require_action.Name,
			Data: require_action.Data,
			Link: require_action.Link,
		})
	return count != 0, err
}

func ListAvailableWorkflows(ctx context.Context, orgID int64) ([]*RequireAction, error) {
	requireActionList := []*RequireAction{}
	orgRepos, err := org_model.GetOrgRepositories(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("ListAvailableWorkflows get org repos: %w", err)
	}
	for _, repo := range orgRepos {
		repo.LoadUnits(ctx)
		actionsConfig := repo.MustGetUnit(ctx, unit.TypeActions).ActionsConfig()
		enabledWorkflows := actionsConfig.GetGlobalWorkflow()
		for _, workflow := range enabledWorkflows {
			requireAction := &RequireAction{
				OrgID:   orgID,
				RepoID:  repo.ID,
				Repo:		 repo,
				Data:    workflow,
				Link:    repo.APIURL(),
			}
			requireActionList = append(requireActionList, requireAction)
		}
	}
	for _, require_action := range requireActionList {
		if err := require_action.Validate(); err != nil {
			return requireActionList, err
		}
	}
	return requireActionList, nil
}
