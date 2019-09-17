// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/private"

	"github.com/urfave/cli"
)

var (
	// CmdHook represents the available hooks sub-command.
	CmdHook = cli.Command{
		Name:        "hook",
		Usage:       "Delegate commands to corresponding Git hooks",
		Description: "This should only be called by Git",
		Subcommands: []cli.Command{
			subcmdHookPreReceive,
			subcmdHookUpdate,
			subcmdHookPostReceive,
		},
	}

	subcmdHookPreReceive = cli.Command{
		Name:        "pre-receive",
		Usage:       "Delegate pre-receive Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookPreReceive,
	}
	subcmdHookUpdate = cli.Command{
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

	setup("hooks/pre-receive.log")

	// the environment setted on serv command
	isWiki := (os.Getenv(models.EnvRepoIsWiki) == "true")
	username := os.Getenv(models.EnvRepoUsername)
	reponame := os.Getenv(models.EnvRepoName)
	userID, _ := strconv.ParseInt(os.Getenv(models.EnvPusherID), 10, 64)
	prID, _ := strconv.ParseInt(os.Getenv(models.ProtectedBranchPRID), 10, 64)

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

		// If the ref is a branch, check if it's protected
		if strings.HasPrefix(refFullName, git.BranchPrefix) {
			statusCode, msg := private.HookPreReceive(username, reponame, private.HookOptions{
				OldCommitID:                     oldCommitID,
				NewCommitID:                     newCommitID,
				RefFullName:                     refFullName,
				UserID:                          userID,
				GitAlternativeObjectDirectories: os.Getenv(private.GitAlternativeObjectDirectories),
				GitObjectDirectory:              os.Getenv(private.GitObjectDirectory),
				GitQuarantinePath:               os.Getenv(private.GitQuarantinePath),
				ProtectedBranchID:               prID,
			})
			switch statusCode {
			case http.StatusInternalServerError:
				fail("Internal Server Error", msg)
			case http.StatusForbidden:
				fail(msg, "")
			}
		}
	}

	return nil
}

func runHookUpdate(c *cli.Context) error {
	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		return nil
	}

	setup("hooks/update.log")

	return nil
}

func runHookPostReceive(c *cli.Context) error {
	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		return nil
	}

	setup("hooks/post-receive.log")

	// the environment setted on serv command
	repoUser := os.Getenv(models.EnvRepoUsername)
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

		res, err := private.HookPostReceive(repoUser, repoName, private.HookOptions{
			OldCommitID: oldCommitID,
			NewCommitID: newCommitID,
			RefFullName: refFullName,
			UserID:      pusherID,
			UserName:    pusherName,
		})

		if res == nil {
			fail("Internal Server Error", err)
		}

		if res["message"] == false {
			continue
		}

		fmt.Fprintln(os.Stderr, "")
		if res["create"] == true {
			fmt.Fprintf(os.Stderr, "Create a new pull request for '%s':\n", res["branch"])
			fmt.Fprintf(os.Stderr, "  %s\n", res["url"])
		} else {
			fmt.Fprint(os.Stderr, "Visit the existing pull request:\n")
			fmt.Fprintf(os.Stderr, "  %s\n", res["url"])
		}
		fmt.Fprintln(os.Stderr, "")
	}

	return nil
}
