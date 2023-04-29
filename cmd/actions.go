// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

var (
	// CmdActions represents the available actions sub-commands.
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
				Usage: "{owner}[/{repo}] - leave empty for a global runner",
			},
		},
	}
)

func runGenerateActionsRunnerToken(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setting.Init(&setting.Options{})

	scope := c.String("scope")

	respText, extra := private.GenerateActionsRunnerToken(ctx, scope)
	if extra.HasError() {
		return handleCliResponseExtra(extra)
	}
	_, _ = fmt.Printf("%s\n", respText)
	return nil
}
