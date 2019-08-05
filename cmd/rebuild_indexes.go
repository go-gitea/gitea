// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

// CmdRebuildIndexes represents the available rebuild-indexes sub-command
var CmdRebuildIndexes = cli.Command{
	Name:        "rebuild-indexes",
	Usage:       "Rebuild text indexes",
	Description: "This command rebuilds text indexes for issues and repositories",
	Action:      runRebuildIndexes,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "repositories",
			Usage: "Rebuild text indexes for repository content",
		},
		cli.BoolFlag{
			Name:  "issues",
			Usage: "Rebuild text indexes for issues",
		},
		cli.BoolFlag{
			Name:  "all",
			Usage: "Rebuild all text indexes",
		},
	},
}

func runRebuildIndexes(ctx *cli.Context) error {
	setting.NewContext()
	setting.NewServices()

	var rebuildIssues bool
	var rebuildRepositories bool

	if ctx.IsSet("repositories") || ctx.IsSet("all") {
		rebuildRepositories = true
		log.Info("Rebuild text indexes for repository content")
	}

	if ctx.IsSet("issues") || ctx.IsSet("all") {
		rebuildIssues = true
		log.Info("Rebuild text indexes for issues")
	}

	if !rebuildIssues && !rebuildRepositories {
		return errors.New("At least one of --repositories, --issues or --all must be used")
	}

	if !rebuildIssues && !setting.Indexer.RepoIndexerEnabled {
		return errors.New("Repository level text indexes are not enabled")
	}

	if err := initDB(); err != nil {
		return err
	}

	repos, err := models.GetAllRepositories()
	if err != nil {
		return err
	}

	for _, r := range repos {
		fmt.Printf("Rebuilding text indexes for %s\n", r.FullName())
		if rebuildRepositories {
			if err = private.RebuildRepoIndex(r.ID); err != nil {
				fmt.Printf("Internal error: %v\n", err)
				return err
			}
		}
	}

	log.Info("Rebuild text indexes done")
	fmt.Println("Done.")
	return nil
}
