// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"os"
	"strconv"
	"strings"

	"github.com/urfave/cli"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// CmdUpdate represents the available update sub-command.
var CmdUpdate = cli.Command{
	Name:        "update",
	Usage:       "This command should only be called by Git hook",
	Description: `Update get pushed info and insert into database`,
	Action:      runUpdate,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "custom/conf/app.ini",
			Usage: "Custom configuration file path",
		},
	},
}

func runUpdate(c *cli.Context) error {
	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	}

	setup("update.log")

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
		UUID:        os.Getenv("GITEA_UUID"),
		RefName:     args[0],
		OldCommitID: args[1],
		NewCommitID: args[2],
	}

	if err := models.AddUpdateTask(&task); err != nil {
		log.GitLogger.Fatal(2, "AddUpdateTask: %v", err)
	}

	return nil
}
