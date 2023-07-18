// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/migrations"

	"github.com/urfave/cli/v2"
)

// CmdDumpRepository represents the available dump repository sub-command.
var CmdDumpRepository = &cli.Command{
	Name:        "dump-repo",
	Usage:       "Dump the repository from git/github/gitea/gitlab",
	Description: "This is a command for dumping the repository data.",
	Action:      runDumpRepository,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "git_service",
			Value: "",
			Usage: "Git service, git, github, gitea, gitlab. If clone_addr could be recognized, this could be ignored.",
		},
		&cli.StringFlag{
			Name:    "repo_dir",
			Aliases: []string{"r"},
			Value:   "./data",
			Usage:   "Repository dir path to store the data",
		},
		&cli.StringFlag{
			Name:  "clone_addr",
			Value: "",
			Usage: "The URL will be clone, currently could be a git/github/gitea/gitlab http/https URL",
		},
		&cli.StringFlag{
			Name:  "auth_username",
			Value: "",
			Usage: "The username to visit the clone_addr",
		},
		&cli.StringFlag{
			Name:  "auth_password",
			Value: "",
			Usage: "The password to visit the clone_addr",
		},
		&cli.StringFlag{
			Name:  "auth_token",
			Value: "",
			Usage: "The personal token to visit the clone_addr",
		},
		&cli.StringFlag{
			Name:  "owner_name",
			Value: "",
			Usage: "The data will be stored on a directory with owner name if not empty",
		},
		&cli.StringFlag{
			Name:  "repo_name",
			Value: "",
			Usage: "The data will be stored on a directory with repository name if not empty",
		},
		&cli.StringFlag{
			Name:  "units",
			Value: "",
			Usage: `Which items will be migrated, one or more units should be separated as comma.
wiki, issues, labels, releases, release_assets, milestones, pull_requests, comments are allowed. Empty means all units.`,
		},
	},
}

func runDumpRepository(ctx *cli.Context) error {
	stdCtx, cancel := installSignals()
	defer cancel()

	if err := initDB(stdCtx); err != nil {
		return err
	}

	// migrations.GiteaLocalUploader depends on git module
	if err := git.InitSimple(context.Background()); err != nil {
		return err
	}

	log.Info("AppPath: %s", setting.AppPath)
	log.Info("AppWorkPath: %s", setting.AppWorkPath)
	log.Info("Custom path: %s", setting.CustomPath)
	log.Info("Log path: %s", setting.Log.RootPath)
	log.Info("Configuration file: %s", setting.CustomConf)

	var (
		serviceType structs.GitServiceType
		cloneAddr   = ctx.String("clone_addr")
		serviceStr  = ctx.String("git_service")
	)

	if strings.HasPrefix(strings.ToLower(cloneAddr), "https://github.com/") {
		serviceStr = "github"
	} else if strings.HasPrefix(strings.ToLower(cloneAddr), "https://gitlab.com/") {
		serviceStr = "gitlab"
	} else if strings.HasPrefix(strings.ToLower(cloneAddr), "https://gitea.com/") {
		serviceStr = "gitea"
	}
	if serviceStr == "" {
		return errors.New("git_service missed or clone_addr cannot be recognized")
	}
	serviceType = convert.ToGitServiceType(serviceStr)

	opts := base.MigrateOptions{
		GitServiceType: serviceType,
		CloneAddr:      cloneAddr,
		AuthUsername:   ctx.String("auth_username"),
		AuthPassword:   ctx.String("auth_password"),
		AuthToken:      ctx.String("auth_token"),
		RepoName:       ctx.String("repo_name"),
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
			switch strings.ToLower(strings.TrimSpace(unit)) {
			case "":
				continue
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
			default:
				return errors.New("invalid unit: " + unit)
			}
		}
	}

	// the repo_dir will be removed if error occurs in DumpRepository
	// make sure the directory doesn't exist or is empty, prevent from deleting user files
	repoDir := ctx.String("repo_dir")
	if exists, err := util.IsExist(repoDir); err != nil {
		return fmt.Errorf("unable to stat repo_dir %q: %w", repoDir, err)
	} else if exists {
		if isDir, _ := util.IsDir(repoDir); !isDir {
			return fmt.Errorf("repo_dir %q already exists but it's not a directory", repoDir)
		}
		if dir, _ := os.ReadDir(repoDir); len(dir) > 0 {
			return fmt.Errorf("repo_dir %q is not empty", repoDir)
		}
	}

	if err := migrations.DumpRepository(
		context.Background(),
		repoDir,
		ctx.String("owner_name"),
		opts,
	); err != nil {
		log.Fatal("Failed to dump repository: %v", err)
		return err
	}

	log.Trace("Dump finished!!!")

	return nil
}
