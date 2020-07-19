// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"strings"

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
		cli.StringFlag{
			Name:  "clone_addr",
			Value: "",
			Usage: "The URL will be clone, currently could be a github or gitlab http/https URL",
		},
		cli.StringFlag{
			Name:  "auth_username",
			Value: "",
			Usage: "The username or personal token to visit the clone_addr, it's required",
		},
		cli.StringFlag{
			Name:  "auth_password",
			Value: "",
			Usage: "The password to visit the clone_addr if auth_username is a real user name",
		},
		cli.StringFlag{
			Name:  "owner_name",
			Value: "",
			Usage: "The data will be stored on a directory with owner name if not empty",
		},
		cli.StringFlag{
			Name:  "repo_name",
			Value: "",
			Usage: "The data will be stored on a directory with repository name if not empty",
		},
		cli.StringFlag{
			Name:  "units",
			Value: "",
			Usage: `Which items will be migrated, one or more units should be separated as comma. 
wiki, issues, labels, releases, release_assets, milestones, pull_requests, comments are allowed. Empty means all units.`,
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

	var opts = base.MigrateOptions{
		CloneAddr:    ctx.String("clone_addr"),
		AuthUsername: ctx.String("auth_username"),
		AuthPassword: ctx.String("auth_password"),
		RepoName:     ctx.String("repo_name"),
	}

	if len(ctx.String("units")) == 0 {
		opts.Wiki = true
		opts.Issues = true
		opts.Milestones = true
		opts.Labels = true
		opts.Releases = true
		opts.Comments = true
		opts.PullRequests = true
		opts.ReleaseAssets = true
	} else {
		units := strings.Split(ctx.String("units"), ",")
		for _, unit := range units {
			switch strings.ToLower(unit) {
			case "wiki":
				opts.Wiki = true
			case "issues":
				opts.Issues = true
			case "milestones":
				opts.Milestones = true
			case "labels":
				opts.Labels = true
			case "releases":
				opts.Releases = true
			case "release_assets":
				opts.ReleaseAssets = true
			case "comments":
				opts.Comments = true
			case "pull_requests":
				opts.PullRequests = true
			}
		}
	}

	if err := migrations.DumpRepository(
		context.Background(),
		ctx.String("repo_dir"),
		ctx.String("owner_name"),
		opts,
	); err != nil {
		log.Fatal("Failed to dump repository: %v", err)
		return err
	}

	return nil
}
