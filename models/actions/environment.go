// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"gitea.dev/models/db"
	"gitea.dev/modules/actions/workflowpattern"
	"gitea.dev/modules/git"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// ActionEnvironment is a named deployment target holding its own secrets and variables.
type ActionEnvironment struct {
	ID     int64  `xorm:"pk autoincr"`
	RepoID int64  `xorm:"UNIQUE(repo_lower_name) NOT NULL"`
	Name   string `xorm:"NOT NULL"`
	// carries the unique constraint, so lookups ignore the MySQL/MSSQL collation
	LowerName string `xorm:"UNIQUE(repo_lower_name) NOT NULL"`

	// newline-separated because a branch name may contain a comma
	AllowedBranchPatterns string `xorm:"TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionEnvironment))
}

const EnvironmentNameMaxLength = 255

type ErrEnvironmentNotFound struct {
	Name string
}

func (err ErrEnvironmentNotFound) Error() string {
	return fmt.Sprintf("environment not found [name: %s]", err.Name)
}

func (err ErrEnvironmentNotFound) Unwrap() error {
	return util.ErrNotExist
}

type FindEnvironmentsOptions struct {
	db.ListOptions
	RepoID int64
}

func (opts FindEnvironmentsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID != 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	return cond
}

func (opts FindEnvironmentsOptions) ToOrders() string {
	return "lower_name ASC"
}

func ValidateEnvironmentName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > EnvironmentNameMaxLength {
		return util.ErrorWrapTranslatable(util.NewInvalidArgumentErrorf("invalid environment name %q", name), "environments.name_invalid", EnvironmentNameMaxLength)
	}
	return nil
}

func SplitBranchPatterns(patterns string) []string {
	result := []string{}
	for pattern := range strings.SplitSeq(patterns, "\n") {
		if pattern = strings.TrimSpace(pattern); pattern != "" {
			result = append(result, pattern)
		}
	}
	return result
}

// qualifyPattern scopes a bare pattern to branches, so that tags and pull refs need an explicit "refs/..." pattern.
func qualifyPattern(pattern string) string {
	negated := strings.HasPrefix(pattern, "!")
	pattern = strings.TrimPrefix(pattern, "!")
	if !strings.HasPrefix(pattern, "refs/") {
		pattern = git.BranchPrefix + pattern
	}
	return util.Iif(negated, "!", "") + pattern
}

func JoinBranchPatterns(patterns []string) (string, error) {
	var kept []string
	for _, pattern := range patterns {
		if pattern = strings.TrimSpace(pattern); pattern == "" {
			continue
		}
		if _, err := workflowpattern.CompilePatterns(qualifyPattern(pattern)); err != nil {
			return "", util.ErrorWrapTranslatable(util.NewInvalidArgumentErrorf("invalid branch pattern %q: %v", pattern, err), "environments.branch_pattern_invalid", pattern)
		}
		kept = append(kept, pattern)
	}
	return strings.Join(kept, "\n"), nil
}

func (env *ActionEnvironment) BranchPatterns() []string {
	return SplitBranchPatterns(env.AllowedBranchPatterns)
}

// MatchesRef takes a full ref name; an uncompilable policy denies rather than grants.
func (env *ActionEnvironment) MatchesRef(ref string) bool {
	patterns := env.BranchPatterns()
	if len(patterns) == 0 {
		return true
	}
	for i, pattern := range patterns {
		patterns[i] = qualifyPattern(pattern)
	}
	compiled, err := workflowpattern.CompilePatterns(patterns...)
	if err != nil {
		return false
	}
	return !workflowpattern.Skip(compiled, []string{ref})
}

func (env *ActionEnvironment) SettingsLink(repoLink string) string {
	return repoLink + "/settings/actions/environments/" + url.PathEscape(env.Name)
}

func GetEnvironmentByRepoAndName(ctx context.Context, repoID int64, name string) (*ActionEnvironment, error) {
	env, has, err := db.Get[ActionEnvironment](ctx, builder.Eq{
		"repo_id":    repoID,
		"lower_name": strings.ToLower(name),
	})
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrEnvironmentNotFound{Name: name}
	}
	return env, nil
}

func InsertEnvironment(ctx context.Context, repoID int64, name, allowedBranchPatterns string) (*ActionEnvironment, error) {
	env := &ActionEnvironment{
		RepoID:                repoID,
		Name:                  name,
		LowerName:             strings.ToLower(name),
		AllowedBranchPatterns: allowedBranchPatterns,
	}
	return env, db.Insert(ctx, env)
}

func UpdateEnvironment(ctx context.Context, env *ActionEnvironment) error {
	_, err := db.GetEngine(ctx).ID(env.ID).Cols("allowed_branch_patterns").Update(env)
	return err
}

func DeleteEnvironment(ctx context.Context, repoID, envID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		cond := builder.Eq{"repo_id": repoID, "environment_id": envID}
		if _, err := db.GetEngine(ctx).Table("secret").Where(cond).Delete(); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Where(cond).Delete(new(ActionVariable)); err != nil {
			return err
		}
		_, err := db.GetEngine(ctx).Where("id = ? AND repo_id = ?", envID, repoID).Delete(new(ActionEnvironment))
		return err
	})
}
