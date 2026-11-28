// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
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

type FindEnvironmentsOptions struct {
	db.ListOptions
	RepoID int64
}

func (opts FindEnvironmentsOptions) ToConds() builder.Cond {
	return builder.Eq{"repo_id": opts.RepoID}
}

func (opts FindEnvironmentsOptions) ToOrders() string {
	return "lower_name ASC"
}

func ValidateEnvironmentName(name string) error {
	// "." and ".." would be resolved as dot segments of the URL addressing the environment
	if name == "" || name == "." || name == ".." || utf8.RuneCountInString(name) > EnvironmentNameMaxLength {
		// a workflow may name anything, so the message must not grow with it
		err := util.NewInvalidArgumentErrorf("invalid environment name %q", util.EllipsisDisplayString(name, 100))
		return util.ErrorWrapTranslatable(err, "environments.name_invalid", EnvironmentNameMaxLength)
	}
	return nil
}

// qualifyPattern scopes a bare pattern to branches, so that tags and pull refs need an explicit "refs/..." pattern.
func qualifyPattern(pattern string) string {
	pattern, negated := strings.CutPrefix(pattern, "!")
	if !strings.HasPrefix(pattern, "refs/") {
		pattern = git.BranchPrefix + pattern
	}
	return util.Iif(negated, "!", "") + pattern
}

func JoinBranchPatterns(patterns []string) (string, error) {
	patterns = util.SplitTrimSpace(strings.Join(patterns, "\n"), "\n") // validate exactly what BranchPatterns reads back
	for _, pattern := range patterns {
		if _, err := workflowpattern.CompilePatterns(qualifyPattern(pattern)); err != nil {
			return "", util.ErrorWrapTranslatable(util.NewInvalidArgumentErrorf("invalid branch pattern %q: %v", pattern, err), "environments.branch_pattern_invalid", pattern)
		}
	}
	return strings.Join(patterns, "\n"), nil
}

func (env *ActionEnvironment) Scope() BelongingScope {
	return BelongingScopeEnvironment(env.RepoID, env.ID)
}

func (env *ActionEnvironment) BranchPatterns() []string {
	return util.SliceNilAsEmpty(util.SplitTrimSpace(env.AllowedBranchPatterns, "\n"))
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
		return nil, util.NewNotExistErrorf("environment %q not found", name)
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
