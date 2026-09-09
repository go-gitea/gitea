// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"
	"regexp"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/container"
	"gitea.dev/modules/util"
)

// ActionRunnerGroupRef records that a repository may use runners in the named group.
type ActionRunnerGroupRef struct {
	ID        int64  `xorm:"pk autoincr"`
	GroupName string `xorm:"VARCHAR(255) UNIQUE(group_repo) NOT NULL"`
	RepoID    int64  `xorm:"INDEX UNIQUE(group_repo) NOT NULL"`
}

func init() {
	db.RegisterModel(new(ActionRunnerGroupRef))
}

var RunnerGroupNamePattern = regexp.MustCompile(`^[a-z0-9._-]{1,64}$`)

// NormalizeRunnerGroupNames canonicalizes a comma-separated list, so that names compare equal
// regardless of the database collation.
func NormalizeRunnerGroupNames(input string) ([]string, error) {
	names := make(container.Set[string])
	for _, name := range util.SplitTrimSpace(strings.ToLower(input), ",") {
		if !RunnerGroupNamePattern.MatchString(name) {
			return nil, util.NewInvalidArgumentErrorf("invalid runner group name %q", name)
		}
		names.Add(name)
	}
	return util.Sorted(names.Values()), nil
}

func GetRepoRunnerGroups(ctx context.Context, repoID int64) ([]string, error) {
	var names []string
	err := db.GetEngine(ctx).Table(new(ActionRunnerGroupRef)).
		Cols("group_name").Where("repo_id = ?", repoID).Asc("group_name").Find(&names)
	return names, err
}

// SetRepoRunnerGroups bumps the task version so idle runners reconsider newly eligible work.
func SetRepoRunnerGroups(ctx context.Context, ownerID, repoID int64, names []string) error {
	current, err := GetRepoRunnerGroups(ctx, repoID)
	if err != nil {
		return err
	}
	if util.SliceSortedEqual(current, names) {
		return nil
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := db.DeleteBeans(ctx, &ActionRunnerGroupRef{RepoID: repoID}); err != nil {
			return err
		}
		refs := make([]*ActionRunnerGroupRef, 0, len(names))
		for _, name := range names {
			refs = append(refs, &ActionRunnerGroupRef{RepoID: repoID, GroupName: name})
		}
		if len(refs) > 0 {
			if err := db.Insert(ctx, refs); err != nil {
				return err
			}
		}
		return IncreaseTaskVersion(ctx, ownerID, repoID)
	})
}

func FindKnownRunnerGroupNames(ctx context.Context) ([]string, error) {
	var refNames []string
	if err := db.GetEngine(ctx).Table(new(ActionRunnerGroupRef)).Distinct("group_name").Find(&refNames); err != nil {
		return nil, err
	}
	var runners []*ActionRunner
	if err := db.GetEngine(ctx).Distinct("groups").Find(&runners); err != nil {
		return nil, err
	}

	names := container.SetOf(refNames...)
	for _, runner := range runners {
		names.AddMultiple(runner.Groups...)
	}
	return util.Sorted(names.Values()), nil
}
