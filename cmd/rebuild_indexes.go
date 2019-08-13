// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/indexer/issues"
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
	}

	if ctx.IsSet("issues") || ctx.IsSet("all") {
		rebuildIssues = true
	}

	if !rebuildIssues && !rebuildRepositories {
		fmt.Printf("At least one of --repositories, --issues or --all must be used\n")
		return nil
	}

	if rebuildRepositories && !setting.Indexer.RepoIndexerEnabled {
		fmt.Printf("Repository indexes are not enabled\n")
		rebuildRepositories = false
	}

	if rebuildIssues && setting.Indexer.IssueType != "bleve" {
		log.ColorFprintf(os.Stdout, "Issue index type '%s' does not support or does not require rebuilding\n", setting.Indexer.IssueType)
		rebuildIssues = false
	}

	if rebuildRepositories {
		attemptRebuild("Rebuild repository indexes", private.RebuildRepoIndex, indexer.DropRepoIndex)
	}

	if rebuildIssues {
		attemptRebuild("Rebuild issue indexes", private.RebuildIssueIndex, issues.DropIssueIndex)
	}

	fmt.Println("Rebuild done or in process.")
	return nil
}

func attemptRebuild(msg string, onlineRebuild func() error, offlineDrop func() error) {
	log.Info(msg)
	fmt.Printf("%s: attempting through Gitea API...\n", msg)
	if err := onlineRebuild(); err != nil {
		// FIXME: there's no good way of knowing if Gitea is running
		log.ColorFprintf(os.Stdout, "Error (disregard if it's a connection error): %v\n", err)
		// Attempt a direct delete
		fmt.Printf("Gitea seems to be down; marking index files for recycling the next time Gitea runs.\n")
		if err := offlineDrop(); err != nil {
			log.ColorFprintf(os.Stdout, "Internal error: %v\n", err)
			log.Fatal("Rebuild indexes: %v", err)
		}
	}
}
