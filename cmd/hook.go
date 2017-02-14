// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"code.gitea.io/gitea/models"

	"github.com/urfave/cli"
)

var (
	CmdHook = cli.Command{
		Name:        "hook",
		Usage:       "Delegate commands to corresponding Git hooks",
		Description: "All sub-commands should only be called by Git",
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

	buf := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		buf.Write(scanner.Bytes())
		buf.WriteByte('\n')
	}

	customHooksPath := os.Getenv(envRepoCustomHooksPath)
	hookCmd := exec.Command(filepath.Join(customHooksPath, "pre-receive"))
	hookCmd.Stdout = os.Stdout
	hookCmd.Stdin = buf
	hookCmd.Stderr = os.Stderr
	if err := hookCmd.Run(); err != nil {
		fail("Internal error", "Fail to execute custom pre-receive hook: %v", err)
	}
	return nil
}

func runHookUpdate(c *cli.Context) error {
	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		return nil
	}

	if err := setup("hooks/pre-receive.log"); err != nil {
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

	customHooksPath := os.Getenv(envRepoCustomHooksPath)
	hookCmd := exec.Command(filepath.Join(customHooksPath, "update"), args...)
	hookCmd.Stdout = os.Stdout
	hookCmd.Stdin = os.Stdin
	hookCmd.Stderr = os.Stderr
	if err := hookCmd.Run(); err != nil {
		fail("Internal error", "Fail to execute custom pre-receive hook: %v", err)
	}
	return nil
}

func runHookPostReceive(c *cli.Context) error {
	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		return nil
	}

	if err := setup("hooks/pre-receive.log"); err != nil {
		fail("Hook post-receive init failed", fmt.Sprintf("setup: %v", err))
	}

	customHooksPath := os.Getenv(envRepoCustomHooksPath)
	hookCmd := exec.Command(filepath.Join(customHooksPath, "post-receive"))
	hookCmd.Stdout = os.Stdout
	hookCmd.Stdin = os.Stdin
	hookCmd.Stderr = os.Stderr
	if err := hookCmd.Run(); err != nil {
		fail("Internal error", "Fail to execute custom post-receive hook: %v", err)
	}
	return nil
}
