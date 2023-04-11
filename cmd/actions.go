// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	actions_model "code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"

	"github.com/urfave/cli"
)

// Cmdembedded represents the available extract sub-command.
var (
	CmdActions = cli.Command{
		Name:        "actions",
		Usage:       "",
		Description: "Commands for managing Gitea Actions",
		Subcommands: []cli.Command{
			subcmdActionsGenRunnerToken,
		},
	}

	subcmdActionsGenRunnerToken = cli.Command{
		Name:    "generate-runner-token",
		Usage:   "Generate a new token for a runner to use to register with the server",
		Action:  runGenerateActionsRunnerToken,
		Aliases: []string{"grt"},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "scope, s",
				Value: "",
				Usage: "{owner}[/{repo}]",
			},
		},
	}
)

func runGenerateActionsRunnerToken(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	if err := initDB(ctx); err != nil {
		return err
	}

	scope := c.String("scope")

	owner, repo, err := parseScope(ctx, scope)
	if err != nil {
		return err
	}

	token, err := actions_model.GetUnactivatedRunnerToken(ctx, owner, repo)
	if errors.Is(err, util.ErrNotExist) {
		token, err = actions_model.NewRunnerToken(ctx, owner, repo)
		if err != nil {
			return fmt.Errorf("CreateRunnerToken: %s", err)
		}
	} else if err != nil {
		return fmt.Errorf("GetUnactivatedRunnerToken: %s", err)
	}

	fmt.Printf("%s", token.Token)

	return nil
}

func parseScope(ctx context.Context, scope string) (ownerID, repoID int64, err error) {
	ownerID = 0
	repoID = 0
	if scope == "" {
		return ownerID, repoID, nil
	}

	ownerName, repoName, found := strings.Cut(scope, "/")

	u, err := user_model.GetUserByName(ctx, ownerName)
	if err != nil {
		return ownerID, repoID, err
	}

	if !found {
		return u.ID, repoID, nil
	}

	r, err := repo_model.GetRepositoryByName(u.ID, repoName)
	if err != nil {
		return ownerID, repoID, err
	}
	repoID = r.ID
	return ownerID, repoID, nil
}
