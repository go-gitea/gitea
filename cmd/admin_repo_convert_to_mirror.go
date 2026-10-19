// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"github.com/urfave/cli/v3"
)

func newRepoConvertToMirrorCommand() *cli.Command {
	return &cli.Command{
		Name:      "repo-convert-to-mirror",
		Usage:     "Convert a repository to a mirror",
		ArgsUsage: "{owner}/{repo}",
		Action:    runRepoConvertToMirror,
	}
}

func runRepoConvertToMirror(ctx context.Context, c *cli.Command) error {
	if c.NArg() != 1 {
		return errors.New("repository must be provided as {owner}/{repo}")
	}
	owner, name, found := strings.Cut(c.Args().First(), "/")
	if !found || owner == "" || name == "" || strings.Contains(name, "/") {
		return errors.New("repository must be provided as {owner}/{repo}")
	}

	if !setting.IsInTesting {
		if err := initDB(ctx); err != nil {
			return err
		}
		if err := git.InitSimple(); err != nil {
			return err
		}
	}

	var repo *repo_model.Repository
	err := db.WithTx(ctx, func(ctx context.Context) error {
		var err error
		repo, err = repo_model.GetRepositoryByOwnerAndName(ctx, owner, name)
		if err != nil {
			return err
		}
		if repo.IsMirror {
			return fmt.Errorf("repository %q is already a mirror", repo.FullName())
		}

		if _, err := repo_model.GetMirrorByRepoID(ctx, repo.ID); err == nil {
			return fmt.Errorf("repository %q already has a mirror configuration", repo.FullName())
		} else if !errors.Is(err, util.ErrNotExist) {
			return err
		}

		mirror := &repo_model.Mirror{RepoID: repo.ID}
		repo.IsMirror = true
		if err := repo_model.UpdateRepositoryColsNoAutoTime(ctx, repo, "is_mirror"); err != nil {
			return err
		}
		if _, err := db.GetEngine(ctx).Cols("repo_id").Insert(mirror); err != nil {
			return err
		}
		return git.ManagedRemoteAdd(ctx, repo, mirror.GetRemoteName(), "", git.RemoteOptionMirrorFetch)
	})
	if err != nil {
		return fmt.Errorf("runRepoConvertToMirror: %w", err)
	}

	cprintf(c, "Converted repository %s to a mirror. Configure it in the repository settings.\n", repo.FullName())
	return nil
}
