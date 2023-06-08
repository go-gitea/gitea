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

type ActionVariable struct {
	ID          int64              `xorm:"pk autoincr"`
	OwnerID     int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	RepoID      int64              `xorm:"INDEX UNIQUE(owner_repo_name) NOT NULL DEFAULT 0"`
	Title       string             `xorm:"UNIQUE(owner_repo_name) NOT NULL"`
	Content     string             `xorm:"LONGTEXT NOT NULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionVariable))
}

type ErrVariableUnbound struct{}

func (err ErrVariableUnbound) Error() string {
	return "variable is not bound to the repo or org"
}

type ErrVariableInvalidValue struct {
	Title   *string
	Content *string
}

func (err ErrVariableInvalidValue) Error() string {
	if err.Title != nil {
		return fmt.Sprintf("variable title %s is invalid", *err.Title)
	}
	if err.Content != nil {
		return fmt.Sprintf("variable content %s is invalid", *err.Content)
	}
	return util.ErrInvalidArgument.Error()
}

// some regular expression of `variables`
// reference to: https://docs.github.com/en/actions/learn-github-actions/variables#naming-conventions-for-configuration-variables
var (
	variableTitleReg           = regexp.MustCompile("(?i)^[A-Z_][A-Z0-9_]*$")
	variableForbiddenPrefixReg = regexp.MustCompile("(?i)^GIT(EA|HUB)_")
)

func (v *ActionVariable) Validate() error {
	switch {
	case v.OwnerID == 0 && v.RepoID == 0:
		return ErrVariableUnbound{}
	case len(v.Title) == 0 || len(v.Title) > 50:
		return ErrVariableInvalidValue{Title: &v.Title}
	case len(v.Content) == 0:
		return ErrVariableInvalidValue{Content: &v.Content}
	case !variableTitleReg.MatchString(v.Title) || variableForbiddenPrefixReg.MatchString(v.Title):
		return ErrVariableInvalidValue{Title: &v.Title}
	default:
		return nil
	}
}

func InsertVariable(ctx context.Context, ownerID, repoID int64, title, content string) (*ActionVariable, error) {
	variable := &ActionVariable{
		OwnerID: ownerID,
		RepoID:  repoID,
		Title:   strings.ToUpper(title),
		Content: content,
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
	if err := variable.Validate(); err != nil {
		return false, err
	}
	count, err := db.GetEngine(ctx).ID(variable.ID).Cols("title", "content").
		Update(&ActionVariable{
			Title:   variable.Title,
			Content: variable.Content,
		})
	return count != 0, err
}
