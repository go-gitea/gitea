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
			subcmdActionsSetRunnerToken,
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
		},
	}

	subcmdActionsSetRunnerToken = &cli.Command{
		Name:    "set-runner-token",
		Usage:   "Set a new token for a runner to as register token",
		Action:  runSetActionsRunnerToken,
		Aliases: []string{"srt"},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "scope",
				Aliases: []string{"s"},
				Value:   "",
				Usage:   "{owner}[/{repo}] - leave empty for a global runner",
			},
			&cli.StringFlag{
				Name:    "token",
				Aliases: []string{"t"},
				Value:   "",
				Usage:   "[{token}] - leave empty will generate a new token, otherwise will update the token to database. The token MUST be a 40 digital string containing only [0-9a-zA-Z]",
			},
		},
	}
)

func runGenerateActionsRunnerToken(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setting.MustInstalled()

	scope := c.String("scope")

	respText, extra := private.GenerateActionsRunnerToken(ctx, scope)
	if extra.HasError() {
		return handleCliResponseExtra(extra)
	}
	_, _ = fmt.Printf("%s\n", respText.Text)
	return nil
}

func runSetActionsRunnerToken(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setting.MustInstalled()

	scope := c.String("scope")
	putToken := c.String("token")

	respText, extra := private.SetActionsRunnerToken(ctx, scope, putToken)
	if extra.HasError() {
		return handleCliResponseExtra(extra)
	}
	_, _ = fmt.Printf("%s\n", respText.Text)
	return nil
}
