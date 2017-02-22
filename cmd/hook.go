// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"code.gitea.io/gitea/models"

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
	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		return nil
	}

	if err := setup("hooks/update.log"); err != nil {
		fail("Hook update init failed", fmt.Sprintf("setup: %v", err))
	}

	args := c.Args()
	if len(args) != 3 {
		fail("Arguments received are not equal to three", "Arguments received are not equal to three")
	} else if len(args[0]) == 0 {
		fail("First argument 'refName' is empty", "First argument 'refName' is empty")
	}

	uuid := os.Getenv(envUpdateTaskUUID)
	if err := models.AddUpdateTask(&models.UpdateTask{
		UUID:        uuid,
		RefName:     args[0],
		OldCommitID: args[1],
		NewCommitID: args[2],
	}); err != nil {
		fail("Internal error", "Fail to add update task '%s': %v", uuid, err)
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
