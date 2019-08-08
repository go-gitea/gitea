// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/urfave/cli"
)

type rebuildStepFunc func(repoID int64) (bool, error)

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
		fmt.Printf("At least one of --repositories, --issues or --all must be used\n")
		return nil
	}

	if !rebuildIssues && !setting.Indexer.RepoIndexerEnabled {
		fmt.Printf("Repository level text indexes are not enabled\n")
		return nil
	}

	if err := initDB(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}

	for page := 1; ; page++ {
		repos, _, err := models.SearchRepositoryByName(&models.SearchRepoOptions{
			Page:        page,
			PageSize:    models.RepositoryListDefaultPageSize,
			OrderBy:     models.SearchOrderByID,
			Private:     true,
			Collaborate: util.OptionalBoolFalse,
		})
		if err != nil {
			log.Error("SearchRepositoryByName: %v", err)
			return err
		}
		if len(repos) == 0 {
			break
		}

		for _, repo := range repos {
			fmt.Printf("Rebuilding text indexes for %s\n", repo.FullName())
			if rebuildRepositories {
				if err := rebuildStep(repo.ID, private.RebuildRepoIndex); err != nil {
					return err
				}
			}
			if rebuildIssues {
				if err := rebuildStep(repo.ID, private.RebuildIssueIndex); err != nil {
					return err
				}
			}
		}
	}

	fmt.Println("Done")
	return nil
}

func rebuildStep(repoID int64, fn rebuildStepFunc) error {
	for {
		toobusy, err := fn(repoID)
		if err != nil {
			fmt.Printf("Internal error: %v\n", err)
			return err
		}
		if !toobusy {
			return nil
		}
		fmt.Printf("Server too busy; backing off...\n")
		time.Sleep(1 * time.Second)
	}
}
