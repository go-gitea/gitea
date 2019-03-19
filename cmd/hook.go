// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

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

	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	} else if c.GlobalIsSet("config") {
		setting.CustomConf = c.GlobalString("config")
	}

	setup("hooks/pre-receive.log")

	// the environment setted on serv command
	repoID, _ := strconv.ParseInt(os.Getenv(models.ProtectedBranchRepoID), 10, 64)
	isWiki := (os.Getenv(models.EnvRepoIsWiki) == "true")
	username := os.Getenv(models.EnvRepoUsername)
	reponame := os.Getenv(models.EnvRepoName)
	userIDStr := os.Getenv(models.EnvPusherID)
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
		protectBranch, err := private.GetProtectedBranchBy(repoID, branchName)
		if err != nil {
			fail("Internal error", fmt.Sprintf("retrieve protected branches information failed: %v", err))
		}

		if protectBranch != nil && protectBranch.IsProtected() {
			// check and deletion
			if newCommitID == git.EmptySHA {
				fail(fmt.Sprintf("branch %s is protected from deletion", branchName), "")
			}

			// detect force push
			if git.EmptySHA != oldCommitID {
				output, err := git.NewCommand("rev-list", "--max-count=1", oldCommitID, "^"+newCommitID).RunInDir(repoPath)
				if err != nil {
					fail("Internal error", "Fail to detect force push: %v", err)
				} else if len(output) > 0 {
					fail(fmt.Sprintf("branch %s is protected from force push", branchName), "")
				}
			}

			userID, _ := strconv.ParseInt(userIDStr, 10, 64)
			canPush, err := private.CanUserPush(protectBranch.ID, userID)
			if err != nil {
				fail("Internal error", "Fail to detect user can push: %v", err)
			} else if !canPush {
				fail(fmt.Sprintf("protected branch %s can not be pushed to", branchName), "")
			}
		}
	}

	return nil
}

func runHookUpdate(c *cli.Context) error {
	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		return nil
	}

	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	} else if c.GlobalIsSet("config") {
		setting.CustomConf = c.GlobalString("config")
	}

	setup("hooks/update.log")

	return nil
}

func runHookPostReceive(c *cli.Context) error {
	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		return nil
	}

	if c.IsSet("config") {
		setting.CustomConf = c.String("config")
	} else if c.GlobalIsSet("config") {
		setting.CustomConf = c.GlobalString("config")
	}

	setup("hooks/post-receive.log")

	// the environment setted on serv command
	repoID, _ := strconv.ParseInt(os.Getenv(models.ProtectedBranchRepoID), 10, 64)
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

		if err := private.PushUpdate(models.PushUpdateOptions{
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

		if newCommitID != git.EmptySHA && strings.HasPrefix(refFullName, git.BranchPrefix) {
			branch := strings.TrimPrefix(refFullName, git.BranchPrefix)
			repo, pullRequestAllowed, err := private.GetRepository(repoID)
			if err != nil {
				log.GitLogger.Error(2, "get repo: %v", err)
				break
			}
			if !pullRequestAllowed {
				break
			}

			baseRepo := repo
			if repo.IsFork {
				baseRepo = repo.BaseRepo
			}

			if !repo.IsFork && branch == baseRepo.DefaultBranch {
				break
			}

			pr, err := private.ActivePullRequest(baseRepo.ID, repo.ID, baseRepo.DefaultBranch, branch)
			if err != nil {
				log.GitLogger.Error(2, "get active pr: %v", err)
				break
			}

			fmt.Fprintln(os.Stderr, "")
			if pr == nil {
				if repo.IsFork {
					branch = fmt.Sprintf("%s:%s", repo.OwnerName, branch)
				}
				fmt.Fprintf(os.Stderr, "Create a new pull request for '%s':\n", branch)
				fmt.Fprintf(os.Stderr, "  %s/compare/%s...%s\n", baseRepo.HTMLURL(), util.PathEscapeSegments(baseRepo.DefaultBranch), util.PathEscapeSegments(branch))
			} else {
				fmt.Fprint(os.Stderr, "Visit the existing pull request:\n")
				fmt.Fprintf(os.Stderr, "  %s/pulls/%d\n", baseRepo.HTMLURL(), pr.Index)
			}
			fmt.Fprintln(os.Stderr, "")
		}

	}

	return nil
}
