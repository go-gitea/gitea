// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"os"
	"strconv"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
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

	// the environment setted on serv command
	repoID, _ := strconv.ParseInt(os.Getenv(models.ProtectedBranchRepoID), 10, 64)
	isWiki := (os.Getenv(models.EnvRepoIsWiki) == "true")
	username := os.Getenv(models.EnvRepoUsername)
	reponame := os.Getenv(models.EnvRepoName)
	repoPath := models.RepoPath(username, reponame)

	buf := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		buf.Write(scanner.Bytes())
		buf.WriteByte('\n')

		// TODO: support news feeds for wiki
		if isWiki {
			continue
		}

		fields := bytes.Fields(scanner.Bytes())
		if len(fields) != 3 {
			continue
		}

		oldCommitID := string(fields[0])
		newCommitID := string(fields[1])
		refFullName := string(fields[2])

		branchName := strings.TrimPrefix(refFullName, git.BranchPrefix)
		protectBranch, err := models.GetProtectedBranchBy(repoID, branchName)
		if err != nil {
			log.GitLogger.Fatal(2, "retrieve protected branches information failed")
		}

		if protectBranch != nil {
			fail(fmt.Sprintf("protected branch %s can not be pushed to", branchName), "")
		}

		// check and deletion
		if newCommitID == git.EmptySHA {
			fail(fmt.Sprintf("Branch '%s' is protected from deletion", branchName), "")
		}

		// Check force push
		output, err := git.NewCommand("rev-list", oldCommitID, "^"+newCommitID).RunInDir(repoPath)
		if err != nil {
			fail("Internal error", "Fail to detect force push: %v", err)
		} else if len(output) > 0 {
			fail(fmt.Sprintf("Branch '%s' is protected from force push", branchName), "")
		}
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

	return nil
}

func runHookPostReceive(c *cli.Context) error {
	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		return nil
	}

	if err := setup("hooks/post-receive.log"); err != nil {
		fail("Hook post-receive init failed", fmt.Sprintf("setup: %v", err))
	}

	// the environment setted on serv command
	repoUser := os.Getenv(models.EnvRepoUsername)
	repoUserSalt := os.Getenv(models.EnvRepoUserSalt)
	isWiki := (os.Getenv(models.EnvRepoIsWiki) == "true")
	repoName := os.Getenv(models.EnvRepoName)
	pusherID, _ := strconv.ParseInt(os.Getenv(models.EnvPusherID), 10, 64)
	pusherName := os.Getenv(models.EnvPusherName)

	buf := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		buf.Write(scanner.Bytes())
		buf.WriteByte('\n')

		// TODO: support news feeds for wiki
		if isWiki {
			continue
		}

		fields := bytes.Fields(scanner.Bytes())
		if len(fields) != 3 {
			continue
		}

		oldCommitID := string(fields[0])
		newCommitID := string(fields[1])
		refFullName := string(fields[2])

		if err := models.PushUpdate(models.PushUpdateOptions{
			RefFullName:  refFullName,
			OldCommitID:  oldCommitID,
			NewCommitID:  newCommitID,
			PusherID:     pusherID,
			PusherName:   pusherName,
			RepoUserName: repoUser,
			RepoName:     repoName,
		}); err != nil {
			log.GitLogger.Error(2, "Update: %v", err)
		}

		// Ask for running deliver hook and test pull request tasks.
		reqURL := setting.LocalURL + repoUser + "/" + repoName + "/tasks/trigger?branch=" +
			strings.TrimPrefix(refFullName, git.BranchPrefix) + "&secret=" + base.EncodeMD5(repoUserSalt) + "&pusher=" + com.ToStr(pusherID)
		log.GitLogger.Trace("Trigger task: %s", reqURL)

		resp, err := httplib.Head(reqURL).SetTLSClientConfig(&tls.Config{
			InsecureSkipVerify: true,
		}).Response()
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode/100 != 2 {
				log.GitLogger.Error(2, "Failed to trigger task: not 2xx response code")
			}
		} else {
			log.GitLogger.Error(2, "Failed to trigger task: %v", err)
		}
	}

	return nil
}
