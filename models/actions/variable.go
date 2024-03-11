// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
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
	if v.OwnerID != 0 && v.RepoID != 0 {
		return errors.New("a variable should not be bound to an owner and a repository at the same time")
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

func (opts FindVariablesOpts) ToConds() builder.Cond {
	cond := builder.NewCond()
	cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	return cond
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

func GetVariablesOfRun(ctx context.Context, run *ActionRun) (map[string]string, error) {
	variables := map[string]string{}

	// Global
	globalVariables, err := db.Find[ActionVariable](ctx, FindVariablesOpts{})
	if err != nil {
		log.Error("find global variables: %v", err)
		return nil, err
	}

	// Org / User level
	ownerVariables, err := db.Find[ActionVariable](ctx, FindVariablesOpts{OwnerID: run.Repo.OwnerID})
	if err != nil {
		log.Error("find variables of org: %d, error: %v", run.Repo.OwnerID, err)
		return nil, err
	}

	// Repo level
	repoVariables, err := db.Find[ActionVariable](ctx, FindVariablesOpts{RepoID: run.RepoID})
	if err != nil {
		log.Error("find variables of repo: %d, error: %v", run.RepoID, err)
		return nil, err
	}

	// Level precedence: Repo > Org / User > Global
	for _, v := range append(globalVariables, append(ownerVariables, repoVariables...)...) {
		variables[v.Name] = v.Data
	}

	return variables, nil
}
