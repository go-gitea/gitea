// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

// CmdDumpRepository represents the available dump repository sub-command.
var CmdDumpRepository = cli.Command{
	Name:        "dump-repo",
	Usage:       "Dump the repository from github/gitlab",
	Description: "This is a command for dumping the repository data.",
	Action:      runDumpRepository,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "repo_dir, r",
			Value: "./data",
			Usage: "Repository dir path",
		},
	},
}

func runDumpRepository(ctx *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	log.Trace("AppPath: %s", setting.AppPath)
	log.Trace("AppWorkPath: %s", setting.AppWorkPath)
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)
	setting.InitDBConfig()

	if err := migrations.DumpRepository(
		context.Background(),
		ctx.String("repo_dir"),
		ctx.String("owner_name"),
		base.MigrateOptions{
			CloneAddr:    ctx.String("clone_addr"),
			AuthUsername: ctx.String("auth_username"),
			AuthPassword: ctx.String("auth_password"),
			RepoName:     ctx.String("repo_name"),
			Wiki:         true,
			Issues:       true,
			Milestones:   true,
			Labels:       true,
			Releases:     true,
			Comments:     true,
			PullRequests: true,
		}); err != nil {
		log.Fatal("Failed to dump repository: %v", err)
		return err
	}

	return nil
}
