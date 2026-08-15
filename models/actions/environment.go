// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	// carries the unique constraint, so lookups ignore the collation MySQL and MSSQL apply to Name
	LowerName string `xorm:"UNIQUE(repo_lower_name) NOT NULL"`

	// one glob per line, empty allows every ref; newline-separated because a branch name may contain a comma
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

type ErrEnvironmentAlreadyExists struct {
	Name string
}

func (err ErrEnvironmentAlreadyExists) Error() string {
	return fmt.Sprintf("environment already exists [name: %s]", err.Name)
}

func (err ErrEnvironmentAlreadyExists) Unwrap() error {
	return util.ErrAlreadyExist
}

type ErrInvalidEnvironmentName struct {
	Name   string
	Reason string
}

func (err ErrInvalidEnvironmentName) Error() string {
	return fmt.Sprintf("invalid environment name %q: %s", err.Name, err.Reason)
}

func (err ErrInvalidEnvironmentName) Unwrap() error {
	return util.ErrInvalidArgument
}

type ErrInvalidBranchPattern struct {
	Pattern string
	Reason  string
}

func (err ErrInvalidBranchPattern) Error() string {
	return fmt.Sprintf("invalid branch pattern %q: %s", err.Pattern, err.Reason)
}

func (err ErrInvalidBranchPattern) Unwrap() error {
	return util.ErrInvalidArgument
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

// ValidateEnvironmentName rejects names that cannot round-trip through a URL path segment.
func ValidateEnvironmentName(name string) error {
	invalid := func(reason string) error { return ErrInvalidEnvironmentName{Name: name, Reason: reason} }
	switch {
	case name == "":
		return invalid("it cannot be empty")
	case len(name) > EnvironmentNameMaxLength:
		return invalid(fmt.Sprintf("it is longer than %d characters", EnvironmentNameMaxLength))
	case name != strings.TrimSpace(name):
		return invalid("it starts or ends with whitespace")
	case strings.ContainsAny(name, "/\\?#%"):
		return invalid(`it contains one of / \ ? # %`)
	case name == "." || name == "..":
		return invalid("it is a relative path segment")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return invalid("it contains a control character")
		}
	}
	return nil
}

// SplitBranchPatterns returns the stored patterns as a list.
func SplitBranchPatterns(patterns string) []string {
	var result []string
	for pattern := range strings.SplitSeq(patterns, "\n") {
		if pattern = strings.TrimSpace(pattern); pattern != "" {
			result = append(result, pattern)
		}
	}
	return result
}

// JoinBranchPatterns renders a pattern list into its stored form, rejecting anything MatchesRef could not compile.
func JoinBranchPatterns(patterns []string) (string, error) {
	var kept []string
	for _, pattern := range patterns {
		if pattern = strings.TrimSpace(pattern); pattern == "" {
			continue
		}
		if _, err := workflowpattern.CompilePatterns(pattern); err != nil {
			return "", ErrInvalidBranchPattern{Pattern: pattern, Reason: err.Error()}
		}
		kept = append(kept, pattern)
	}
	return strings.Join(kept, "\n"), nil
}

func (env *ActionEnvironment) BranchPatterns() []string {
	return SplitBranchPatterns(env.AllowedBranchPatterns)
}

// MatchesRef reports whether ref may deploy to this environment, using the same glob dialect as the
// `on:` branch filters. A policy that cannot be compiled denies the ref rather than granting access.
func (env *ActionEnvironment) MatchesRef(ref string) bool {
	patterns := env.BranchPatterns()
	if len(patterns) == 0 {
		return true
	}
	compiled, err := workflowpattern.CompilePatterns(patterns...)
	if err != nil {
		return false
	}
	return !workflowpattern.Skip(compiled, []string{git.RefName(ref).ShortName()})
}

// ResolveJobEnvironment returns the environment a job deploys to, or nil when it names none or the
// environment has since been deleted. A job whose ref is not allowed must be failed rather than run
// without the environment's credentials.
func ResolveJobEnvironment(ctx context.Context, job *ActionRunJob) (env *ActionEnvironment, allowed bool, err error) {
	if job.EnvironmentName == "" {
		return nil, true, nil
	}
	env, err = GetEnvironmentByRepoAndName(ctx, job.RepoID, job.EnvironmentName)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			return nil, true, nil
		}
		return nil, false, err
	}
	if err := job.LoadRun(ctx); err != nil {
		return nil, false, err
	}
	return env, env.MatchesRef(job.Run.Ref), nil
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

func GetEnvironmentByID(ctx context.Context, id int64) (*ActionEnvironment, error) {
	env := &ActionEnvironment{}
	has, err := db.GetEngine(ctx).ID(id).Get(env)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrEnvironmentNotFound{Name: fmt.Sprintf("id:%d", id)}
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
	env.LowerName = strings.ToLower(env.Name)
	_, err := db.GetEngine(ctx).ID(env.ID).Cols("name", "lower_name", "allowed_branch_patterns").Update(env)
	return err
}

// DeleteEnvironment removes an environment together with the secrets and variables scoped to it.
func DeleteEnvironment(ctx context.Context, repoID, envID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).
			Table("secret").
			Where("repo_id = ? AND environment_id = ?", repoID, envID).
			Delete(); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).
			Where("repo_id = ? AND environment_id = ?", repoID, envID).
			Delete(new(ActionVariable)); err != nil {
			return err
		}
		_, err := db.GetEngine(ctx).Where("id = ? AND repo_id = ?", envID, repoID).Delete(new(ActionEnvironment))
		return err
	})
}
