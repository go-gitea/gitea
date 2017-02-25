// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

var (
	// CmdHook represents the available hooks sub-command.
	CmdHook = cli.Command{
		Name:        "hook",
		Usage:       "Delegate commands to corresponding Git hooks",
		Description: "This should only be called by Git",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "config, c",
				Value: "custom/conf/app.ini",
				Usage: "Custom configuration file path",
			},
		},
		Subcommands: []cli.Command{
			subcmdHookPreReceive,
			subcmdHookUpadte,
			subcmdHookPostReceive,
		},
	}

	subcmdHookPreReceive = cli.Command{
		Name:        "pre-receive",
		Usage:       "Delegate pre-receive Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookPreReceive,
	}
	subcmdHookUpadte = cli.Command{
		Name:        "update",
		Usage:       "Delegate update Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookUpdate,
	}
	subcmdHookPostReceive = cli.Command{
		Name:        "post-receive",
		Usage:       "Delegate post-receive Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookPostReceive,
	}
)

func runHookPreReceive(c *cli.Context) error {
	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		return nil
	}
	if err := setup("hooks/pre-receive.log"); err != nil {
		fail("Hook pre-receive init failed", fmt.Sprintf("setup: %v", err))
	}

	return nil
}

func runHookUpdate(c *cli.Context) error {
	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	}

	if err := setup("hooks/update.log"); err != nil {
		fail("Hook update init failed", fmt.Sprintf("setup: %v", err))
	}

	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		log.GitLogger.Trace("SSH_ORIGINAL_COMMAND is empty")
		return nil
	}

	args := c.Args()
	if len(args) != 3 {
		log.GitLogger.Fatal(2, "Arguments received are not equal to three")
	} else if len(args[0]) == 0 {
		log.GitLogger.Fatal(2, "First argument 'refName' is empty, shouldn't use")
	}

	// protected branch check
	branchName := strings.TrimPrefix(args[0], git.BranchPrefix)
	repoID, _ := strconv.ParseInt(os.Getenv(models.ProtectedBranchRepoID), 10, 64)
	log.GitLogger.Trace("pushing to %d %v", repoID, branchName)
	accessMode := models.ParseAccessMode(os.Getenv(models.ProtectedBranchAccessMode))
	// skip admin or owner AccessMode
	if accessMode == models.AccessModeWrite {
		protectBranch, err := models.GetProtectedBranchBy(repoID, branchName)
		if err != nil {
			log.GitLogger.Fatal(2, "retrieve protected branches information failed")
		}

		if protectBranch != nil {
			log.GitLogger.Fatal(2, "protected branches can not be pushed to")
		}
	}

	task := models.UpdateTask{
		UUID:        os.Getenv(envUpdateTaskUUID),
		RefName:     args[0],
		OldCommitID: args[1],
		NewCommitID: args[2],
	}

	if err := models.AddUpdateTask(&task); err != nil {
		log.GitLogger.Fatal(2, "AddUpdateTask: %v", err)
	}

	return nil
}

func runHookPostReceive(c *cli.Context) error {
	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		return nil
	}

	if err := setup("hooks/post-receive.log"); err != nil {
		fail("Hook post-receive init failed", fmt.Sprintf("setup: %v", err))
	}

	return nil
}
