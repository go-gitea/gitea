// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package variable

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"xorm.io/builder"
)

type Variable struct {
	ID          int64              `xorm:"pk autoincr"`
	OwnerID     int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	Name        string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Data        string             `xorm:"LONGTEXT NOT NULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(Variable))
}

type ErrVariableUnbound struct{}

func (err ErrVariableUnbound) Error() string {
	return "variable is not bound to the repo or org"
}

type ErrVariableInvalidValue struct {
	Name *string
	Data *string
}

func (err ErrVariableInvalidValue) Error() string {
	if err.Name != nil {
		return fmt.Sprintf("varibale name %s is invalid", *err.Name)
	}
	if err.Data != nil {
		return fmt.Sprintf("variable data %s is invalid", *err.Data)
	}
	return util.ErrInvalidArgument.Error()
}

// some regular expression of `variables`
// reference to: https://docs.github.com/en/actions/learn-github-actions/variables#naming-conventions-for-configuration-variables
var (
	varibaleNameReg            = regexp.MustCompile("(?i)^[A-Z_][A-Z0-9_]*$")
	variableForbiddenPrefixReg = regexp.MustCompile("(?i)^GIT(EA|HUB)_")
)

func (v *Variable) Validate() error {
	switch {
	case v.OwnerID == 0 && v.RepoID == 0:
		return ErrVariableUnbound{}
	case len(v.Name) == 0 || len(v.Name) > 50:
		return ErrVariableInvalidValue{Name: &v.Name}
	case len(v.Data) == 0:
		return ErrVariableInvalidValue{Data: &v.Data}
	case !varibaleNameReg.MatchString(v.Name) || variableForbiddenPrefixReg.MatchString(v.Name):
		return ErrVariableInvalidValue{Name: &v.Name}
	default:
		return nil
	}
}

func InsertVariable(ctx context.Context, ownerID, repoID int64, name, data string) (*Variable, error) {
	variable := &Variable{
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

func FindVariables(ctx context.Context, opts FindVariablesOpts) ([]*Variable, error) {
	var variables []*Variable
	sess := db.GetEngine(ctx)
	if opts.PageSize != 0 {
		sess = db.SetSessionPagination(sess, &opts.ListOptions)
	}
	return variables, sess.Where(opts.toConds()).Find(&variables)
}

func GetVariableByID(ctx context.Context, variableID int64) (*Variable, error) {
	var variable Variable
	has, err := db.GetEngine(ctx).Where("id=?", variableID).Get(&variable)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("variable with id %d: %w", variableID, util.ErrNotExist)
	}
	return &variable, nil
}

func UpdateVariable(ctx context.Context, variable *Variable) (bool, error) {
	count, err := db.GetEngine(ctx).ID(variable.ID).Cols("name", "data").
		Where("owner_id = ? and repo_id = ?", variable.OwnerID, variable.RepoID).
		Update(&Variable{
			Name: variable.Name,
			Data: variable.Data,
		})
	return count != 0, err
}
