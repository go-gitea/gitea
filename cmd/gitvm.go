// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"

	gitvm_cli "code.gitea.io/gitea/modules/gitvm/cli"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli/v3"
)

// CmdGitVM represents the gitvm command
var CmdGitVM = &cli.Command{
	Name:        "gitvm",
	Usage:       "GitVM proof spine operations",
	Description: "Commands for managing and verifying the GitVM proof spine",
	Commands: []*cli.Command{
		{
			Name:   "verify",
			Usage:  "Verify the integrity of the GitVM ledger",
			Action: runGitVMVerify,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "data-dir",
					Aliases: []string{"d"},
					Usage:   "Path to the Gitea data directory",
				},
			},
		},
	},
}

func runGitVMVerify(ctx context.Context, c *cli.Command) error {
	// Load settings to get data directory
	setting.InitCfgProvider(c.String("config"))
	setting.LoadCommonSettings()

	dataDir := c.String("data-dir")
	if dataDir == "" {
		dataDir = setting.AppDataPath
	}

	if err := gitvm_cli.Verify(dataDir); err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	return nil
}
