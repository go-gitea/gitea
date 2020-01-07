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
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

const (
	hookBatchSize = 30
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
	if os.Getenv(models.EnvIsInternal) == "true" {
		return nil
	}

	setup("hooks/pre-receive.log", false)

	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		if setting.OnlyAllowPushIfGiteaEnvironmentSet {
			fail(`Rejecting changes as Gitea environment not set.
If you are pushing over SSH you must push with a key managed by
Gitea or set your environment appropriately.`, "")
		} else {
			return nil
		}
	}

	// the environment setted on serv command
	isWiki := (os.Getenv(models.EnvRepoIsWiki) == "true")
	username := os.Getenv(models.EnvRepoUsername)
	reponame := os.Getenv(models.EnvRepoName)
	userID, _ := strconv.ParseInt(os.Getenv(models.EnvPusherID), 10, 64)
	prID, _ := strconv.ParseInt(os.Getenv(models.ProtectedBranchPRID), 10, 64)
	isDeployKey, _ := strconv.ParseBool(os.Getenv(models.EnvIsDeployKey))

	hookOptions := private.HookOptions{
		UserID:                          userID,
		GitAlternativeObjectDirectories: os.Getenv(private.GitAlternativeObjectDirectories),
		GitObjectDirectory:              os.Getenv(private.GitObjectDirectory),
		GitQuarantinePath:               os.Getenv(private.GitQuarantinePath),
		ProtectedBranchID:               prID,
		IsDeployKey:                     isDeployKey,
	}

	scanner := bufio.NewScanner(os.Stdin)

	oldCommitIDs := make([]string, hookBatchSize)
	newCommitIDs := make([]string, hookBatchSize)
	refFullNames := make([]string, hookBatchSize)
	count := 0
	total := 0
	lastline := 0

	commsChan := make(chan string, 10)
	tickerChan := make(chan struct{})
	go func() {
		sb := strings.Builder{}
		hasWritten := false
		ticker := time.NewTicker(1 * time.Second)
	loop:
		for {
			select {
			case s, ok := <-commsChan:
				if ok {
					sb.WriteString(s)
				} else if hasWritten {
					hasWritten = true
					os.Stdout.WriteString(sb.String())
					os.Stdout.Sync()
					sb.Reset()
					break loop
				}
			case <-ticker.C:
				hasWritten = true
				os.Stdout.WriteString(sb.String())
				os.Stdout.Sync()
				sb.Reset()
			}
		}
		sb.Reset()
		ticker.Stop()
		close(tickerChan)
	}()

	for scanner.Scan() {
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
		total++
		lastline++

		// If the ref is a branch, check if it's protected
		if strings.HasPrefix(refFullName, git.BranchPrefix) {
			oldCommitIDs[count] = oldCommitID
			newCommitIDs[count] = newCommitID
			refFullNames[count] = refFullName
			count++
			commsChan <- "*"

			if count >= hookBatchSize {
				fmt.Fprintf(os.Stdout, " Checking %d branches\n", count)

				hookOptions.OldCommitIDs = oldCommitIDs
				hookOptions.NewCommitIDs = newCommitIDs
				hookOptions.RefFullNames = refFullNames
				statusCode, msg := private.HookPreReceive(username, reponame, hookOptions)
				switch statusCode {
				case http.StatusOK:
					// no-op
				case http.StatusInternalServerError:
					close(commsChan)
					<-tickerChan
					fail("Internal Server Error", msg)
				default:
					close(commsChan)
					<-tickerChan
					fail(msg, "")
				}
				count = 0
				lastline = 0
			}
		} else {
			commsChan <- "."
		}
		if lastline >= hookBatchSize {
			commsChan <- "\n"
			lastline = 0
		}
	}

	if count > 0 {
		hookOptions.OldCommitIDs = oldCommitIDs[:count]
		hookOptions.NewCommitIDs = newCommitIDs[:count]
		hookOptions.RefFullNames = refFullNames[:count]

		commsChan <- fmt.Sprintf(" Checking %d branches\n", count)
		os.Stdout.Sync()

		statusCode, msg := private.HookPreReceive(username, reponame, hookOptions)
		switch statusCode {
		case http.StatusInternalServerError:
			close(commsChan)
			<-tickerChan
			fail("Internal Server Error", msg)
		case http.StatusForbidden:
			close(commsChan)
			<-tickerChan
			fail(msg, "")
		}
	} else if lastline > 0 {
		commsChan <- "\n"
		lastline = 0
	}

	commsChan <- fmt.Sprintf("Checked %d references in total\n", total)
	close(commsChan)
	<-tickerChan
	return nil
}

func runHookUpdate(c *cli.Context) error {
	// Update is empty and is kept only for backwards compatibility
	return nil
}

func runHookPostReceive(c *cli.Context) error {
	if os.Getenv(models.EnvIsInternal) == "true" {
		return nil
	}

	setup("hooks/post-receive.log", false)

	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		if setting.OnlyAllowPushIfGiteaEnvironmentSet {
			fail(`Rejecting changes as Gitea environment not set.
If you are pushing over SSH you must push with a key managed by
Gitea or set your environment appropriately.`, "")
		} else {
			return nil
		}
	}

	commsChan := make(chan string, 10)
	tickerChan := make(chan struct{})
	go func() {
		sb := strings.Builder{}
		hasWritten := false
		ticker := time.NewTicker(1 * time.Second)
	loop:
		for {
			select {
			case s, ok := <-commsChan:
				if ok {
					sb.WriteString(s)
				} else if hasWritten {
					hasWritten = true
					os.Stdout.WriteString(sb.String())
					os.Stdout.Sync()
					sb.Reset()
					break loop
				}
			case <-ticker.C:
				hasWritten = true
				os.Stdout.WriteString(sb.String())
				os.Stdout.Sync()
				sb.Reset()
			}
		}
		sb.Reset()
		ticker.Stop()
		close(tickerChan)
	}()

	// the environment setted on serv command
	repoUser := os.Getenv(models.EnvRepoUsername)
	isWiki := (os.Getenv(models.EnvRepoIsWiki) == "true")
	repoName := os.Getenv(models.EnvRepoName)
	pusherID, _ := strconv.ParseInt(os.Getenv(models.EnvPusherID), 10, 64)
	pusherName := os.Getenv(models.EnvPusherName)

	hookOptions := private.HookOptions{
		UserName:                        pusherName,
		UserID:                          pusherID,
		GitAlternativeObjectDirectories: os.Getenv(private.GitAlternativeObjectDirectories),
		GitObjectDirectory:              os.Getenv(private.GitObjectDirectory),
		GitQuarantinePath:               os.Getenv(private.GitQuarantinePath),
	}
	oldCommitIDs := make([]string, hookBatchSize)
	newCommitIDs := make([]string, hookBatchSize)
	refFullNames := make([]string, hookBatchSize)
	count := 0
	total := 0
	wasEmpty := false
	masterPushed := false
	results := make([]private.HookPostReceiveBranchResult, 0)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		// TODO: support news feeds for wiki
		if isWiki {
			continue
		}

		fields := bytes.Fields(scanner.Bytes())
		if len(fields) != 3 {
			continue
		}

		commsChan <- "."
		oldCommitIDs[count] = string(fields[0])
		newCommitIDs[count] = string(fields[1])
		refFullNames[count] = string(fields[2])
		if refFullNames[count] == git.BranchPrefix+"master" && newCommitIDs[count] != git.EmptySHA && count == total {
			masterPushed = true
		}
		count++
		total++

		if count >= hookBatchSize {
			commsChan <- fmt.Sprintf(" Processing %d references\n", count)
			hookOptions.OldCommitIDs = oldCommitIDs
			hookOptions.NewCommitIDs = newCommitIDs
			hookOptions.RefFullNames = refFullNames
			resp, err := private.HookPostReceive(repoUser, repoName, hookOptions)
			if resp == nil {
				close(commsChan)
				<-tickerChan
				hookPrintResults(results)
				fail("Internal Server Error", err)
			}
			wasEmpty = wasEmpty || resp.RepoWasEmpty
			results = append(results, resp.Results...)
			count = 0
		}
	}

	if count == 0 {
		if wasEmpty && masterPushed {
			// We need to tell the repo to reset the default branch to master
			err := private.SetDefaultBranch(repoUser, repoName, "master")
			if err != nil {
				close(commsChan)
				<-tickerChan
				fail("Internal Server Error", "SetDefaultBranch failed with Error: %v", err)
			}
		}
		commsChan <- fmt.Sprintf("Processed %d references in total\n", total)

		close(commsChan)
		<-tickerChan
		hookPrintResults(results)
		return nil
	}

	hookOptions.OldCommitIDs = oldCommitIDs[:count]
	hookOptions.NewCommitIDs = newCommitIDs[:count]
	hookOptions.RefFullNames = refFullNames[:count]

	commsChan <- fmt.Sprintf(" Processing %d references\n", count)

	resp, err := private.HookPostReceive(repoUser, repoName, hookOptions)
	if resp == nil {
		close(commsChan)
		<-tickerChan
		hookPrintResults(results)
		fail("Internal Server Error", err)
	}
	wasEmpty = wasEmpty || resp.RepoWasEmpty
	results = append(results, resp.Results...)

	commsChan <- fmt.Sprintf("Processed %d references in total\n", total)

	if wasEmpty && masterPushed {
		// We need to tell the repo to reset the default branch to master
		err := private.SetDefaultBranch(repoUser, repoName, "master")
		if err != nil {
			fail("Internal Server Error", "SetDefaultBranch failed with Error: %v", err)
		}
	}

	close(commsChan)
	<-tickerChan
	hookPrintResults(results)

	return nil
}

func hookPrintResults(results []private.HookPostReceiveBranchResult) {
	for _, res := range results {
		if !res.Message {
			continue
		}

		fmt.Fprintln(os.Stderr, "")
		if res.Create {
			fmt.Fprintf(os.Stderr, "Create a new pull request for '%s':\n", res.Branch)
			fmt.Fprintf(os.Stderr, "  %s\n", res.URL)
		} else {
			fmt.Fprint(os.Stderr, "Visit the existing pull request:\n")
			fmt.Fprintf(os.Stderr, "  %s\n", res.URL)
		}
		fmt.Fprintln(os.Stderr, "")
		os.Stderr.Sync()
	}
}
