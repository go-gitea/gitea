// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli/v2"
)

var (
	// CmdActions represents the available actions sub-commands.
	CmdActions = &cli.Command{
		Name:  "actions",
		Usage: "Manage Gitea Actions",
		Subcommands: []*cli.Command{
			subcmdActionsGenRunnerToken,
		},
	}

	subcmdActionsGenRunnerToken = &cli.Command{
		Name:    "generate-runner-token",
		Usage:   "Generate a new token for a runner to use to register with the server",
		Action:  runGenerateActionsRunnerToken,
		Aliases: []string{"grt"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "scope",
				Aliases: []string{"s"},
				Value:   "",
				Usage:   "{owner}[/{repo}] - leave empty for a global runner",
			},
			&cli.StringFlag{
				Name:    "put-token",
				Aliases: []string{"t"},
				Value:   "",
				Usage:   "[{token}] - leave empty will generate a new token, otherwise will update the token to database. The token MUST be a 40 digital string containing only 0-9 and a-f",
			},
		},
	}
)

func runGenerateActionsRunnerToken(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setting.MustInstalled()

	scope := c.String("scope")
	putToken := c.String("put-token")

	respText, extra := private.GenerateActionsRunnerToken(ctx, scope, putToken)
	if extra.HasError() {
		return handleCliResponseExtra(extra)
	}
	_, _ = fmt.Printf("%s\n", respText.Text)
	return nil
}
