// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

type ActionVariable struct {
	ID          int64              `xorm:"pk autoincr"`
	OwnerID     int64              `xorm:"UNIQUE(owner_repo_name)"`
	RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name)"`
	Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data        string             `xorm:"LONGTEXT NOT NULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionVariable))
}

func (v *ActionVariable) Validate() error {
	if v.OwnerID == 0 && v.RepoID == 0 {
		return errors.New("the variable is not bound to any scope")
	}
	return nil
}

func InsertVariable(ctx context.Context, ownerID, repoID int64, name, data string) (*ActionVariable, error) {
	variable := &ActionVariable{
		OwnerID: ownerID,
		RepoID:  repoID,
		Name:    strings.ToUpper(name),
		Data:    data,
	}
	if err := variable.Validate(); err != nil {
		return variable, err
	}
	return variable, db.Insert(ctx, variable)
}

type FindVariablesOpts struct {
	db.ListOptions
	OwnerID int64
	RepoID  int64
}

func (opts *FindVariablesOpts) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OwnerID > 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	return cond
}

func FindVariables(ctx context.Context, opts FindVariablesOpts) ([]*ActionVariable, error) {
	var variables []*ActionVariable
	sess := db.GetEngine(ctx)
	if opts.PageSize != 0 {
		sess = db.SetSessionPagination(sess, &opts.ListOptions)
	}
	return variables, sess.Where(opts.toConds()).Find(&variables)
}

func GetVariableByID(ctx context.Context, variableID int64) (*ActionVariable, error) {
	var variable ActionVariable
	has, err := db.GetEngine(ctx).Where("id=?", variableID).Get(&variable)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("variable with id %d: %w", variableID, util.ErrNotExist)
	}
	return &variable, nil
}

func UpdateVariable(ctx context.Context, variable *ActionVariable) (bool, error) {
	count, err := db.GetEngine(ctx).ID(variable.ID).Cols("name", "data").
		Update(&ActionVariable{
			Name: variable.Name,
			Data: variable.Data,
		})
	return count != 0, err
}
