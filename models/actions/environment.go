// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"fmt"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/glob"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// ActionEnvironment represents a deployment environment for a repository.
// Secrets and variables can be scoped to an environment and optionally protected by branch policies.
type ActionEnvironment struct {
	ID     int64  `xorm:"pk autoincr"`
	RepoID int64  `xorm:"UNIQUE(repo_name) NOT NULL"`
	Name   string `xorm:"UNIQUE(repo_name) NOT NULL"`

	// ProtectedBranches is a glob pattern list (comma-separated) that restricts
	// which branches can access this environment's secrets and variables. Empty means no restriction.
	ProtectedBranches string `xorm:"TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ActionEnvironment))
}

// ErrEnvironmentNotFound is returned when an environment does not exist.
type ErrEnvironmentNotFound struct {
	Name string
}

func (err ErrEnvironmentNotFound) Error() string {
	return fmt.Sprintf("environment not found [name: %s]", err.Name)
}

func (err ErrEnvironmentNotFound) Unwrap() error {
	return util.ErrNotExist
}

// ErrEnvironmentAlreadyExists is returned when creating a duplicate environment.
type ErrEnvironmentAlreadyExists struct {
	Name string
}

func (err ErrEnvironmentAlreadyExists) Error() string {
	return fmt.Sprintf("environment already exists [name: %s]", err.Name)
}

func (err ErrEnvironmentAlreadyExists) Unwrap() error {
	return util.ErrAlreadyExist
}

// FindEnvironmentsOptions holds filter parameters for listing environments.
type FindEnvironmentsOptions struct {
	db.ListOptions
	RepoID int64
	Name   string
}

func (opts FindEnvironmentsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID != 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.Name != "" {
		cond = cond.And(builder.Eq{"name": opts.Name})
	}
	return cond
}

// GetEnvironmentByRepoAndName returns the environment matching the given repo and name.
func GetEnvironmentByRepoAndName(ctx context.Context, repoID int64, name string) (*ActionEnvironment, error) {
	envs, err := db.Find[ActionEnvironment](ctx, FindEnvironmentsOptions{RepoID: repoID, Name: name})
	if err != nil {
		return nil, err
	}
	if len(envs) == 0 {
		return nil, ErrEnvironmentNotFound{Name: name}
	}
	return envs[0], nil
}

// GetEnvironmentByID returns the environment with the given id.
func GetEnvironmentByID(ctx context.Context, id int64) (*ActionEnvironment, error) {
	env := &ActionEnvironment{}
	has, err := db.GetEngine(ctx).ID(id).Get(env)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrEnvironmentNotFound{}
	}
	return env, nil
}

// InsertEnvironment creates a new environment for a repository.
func InsertEnvironment(ctx context.Context, repoID int64, name, protectedBranches string) (*ActionEnvironment, error) {
	env := &ActionEnvironment{
		RepoID:            repoID,
		Name:              name,
		ProtectedBranches: protectedBranches,
	}
	return env, db.Insert(ctx, env)
}

// UpdateEnvironment updates mutable fields of an environment.
func UpdateEnvironment(ctx context.Context, env *ActionEnvironment) error {
	_, err := db.GetEngine(ctx).ID(env.ID).Cols("name", "protected_branches").Update(env)
	return err
}

// DeleteEnvironment removes an environment and all its associated secrets and variables.
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

const protectedBranchGlobSeparator = '/'

// ValidateProtectedBranches reports an error if any comma-separated glob pattern in protectedBranches fails to compile.
func ValidateProtectedBranches(protectedBranches string) error {
	for pattern := range strings.SplitSeq(protectedBranches, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if _, err := glob.Compile(pattern, protectedBranchGlobSeparator); err != nil {
			return util.NewInvalidArgumentErrorf("invalid branch pattern %q: %v", pattern, err)
		}
	}
	return nil
}

// MatchesBranch reports whether ref (e.g. "refs/heads/main" or "refs/tags/v1.0") may access
// this environment's secrets and variables. An empty policy allows all refs.
func (env *ActionEnvironment) MatchesBranch(ref string) bool {
	if env.ProtectedBranches == "" {
		return true
	}
	// Strip refs/heads/ or refs/tags/ prefix for comparison
	shortRef := strings.TrimPrefix(ref, "refs/heads/")
	if shortRef == ref {
		shortRef = strings.TrimPrefix(ref, "refs/tags/")
	}
	for pattern := range strings.SplitSeq(env.ProtectedBranches, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		g, err := glob.Compile(pattern, protectedBranchGlobSeparator)
		if err != nil {
			// Skip malformed patterns so one bad glob doesn't deny an otherwise matching ref.
			continue
		}
		ok := g.Match(shortRef)
		if ok {
			return true
		}
	}
	return false
}
