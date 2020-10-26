// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/process"
)

var rangeDiffCommitStartRegex = regexp.MustCompile(`^(\d+|-|\+):`)

// RangeDiffEntry git range-diff entry.
type RangeDiffEntry struct {
	OldIndex          *int64
	Old               string
	Relation          string
	NewIndex          *int64
	New               string
	Commit            *Commit
	MetaDiff          string
	CommitMessageDiff string
	InterDiff         string
}

var gitTimeFormatString = "Mon 02 Jan 15:04:05 2006 -0700"

func getMeta(commit *Commit) string {
	if commit == nil {
		return ""
	}

	return "Author: " + commit.Author.String() + "\n" +
		"AuthorDate: " + commit.Author.When.Format(gitTimeFormatString) + "\n" +
		"Commit: " + commit.Committer.String() + "\n" +
		"CommitDate: " + commit.Committer.When.Format(gitTimeFormatString) + "\n"
}

func getPatch(repo *Repository, commit string) (string, error) {
	if commit == "" {
		return "", nil
	}

	// FIXME: graceful: These commands should likely have a timeout
	ctx, cancel := context.WithCancel(DefaultContext)
	defer cancel()
	var cmd = exec.CommandContext(ctx, GitExecutable, "show", commit)
	cmd.Dir = repo.Path
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("StdoutPipe: %v", err)
	}

	if err = cmd.Start(); err != nil {
		return "", fmt.Errorf("Start: %v", err)
	}

	pid := process.GetManager().Add(fmt.Sprintf("GetDiffRange [repo_path: %s]", repo.Path), cancel)
	defer process.GetManager().Remove(pid)

	buf := new(strings.Builder)
	_, err = io.Copy(buf, stdout)
	if err != nil {
		return "", err
	}

	if err = cmd.Wait(); err != nil {
		return "", fmt.Errorf("Wait: %v", err)
	}

	return buf.String(), nil
}

func (entry *RangeDiffEntry) loadDiff(repo *Repository) error {
	var (
		oldC *Commit
		newC *Commit
		err  error
	)

	oldC = nil
	if entry.Old != "" {
		oldC, err = repo.GetCommit(entry.Old)
		if err != nil {
			return err
		}
	}

	newC = nil
	if entry.New != "" {
		newC, err = repo.GetCommit(entry.New)
		if err != nil {
			return err
		}
	}

	if newC != nil {
		entry.Commit = newC
	} else {
		if oldC == nil {
			return errors.New("Couldn't load commits")
		}
		entry.Commit = oldC
	}

	oldMeta := getMeta(oldC)
	newMeta := getMeta(newC)

	if oldMeta != newMeta {
		entry.MetaDiff = GetUnifiedGitDiff("META", oldMeta, newMeta)
	}

	oldCommitMessage := ""
	newCommitMessage := ""

	if oldC != nil {
		oldCommitMessage = oldC.CommitMessage
	}
	if newC != nil {
		newCommitMessage = newC.CommitMessage
	}

	if oldCommitMessage != newCommitMessage {
		entry.CommitMessageDiff = GetUnifiedGitDiff("COMMIT_MSG", oldCommitMessage, newCommitMessage)
	}

	oldPatch, err := getPatch(repo, entry.Old)
	if err != nil {
		return err
	}
	newPatch, err := getPatch(repo, entry.New)
	if err != nil {
		return err
	}

	interdiff, err := GetInterdiff(oldPatch, newPatch)
	if err != nil {
		return err
	}

	entry.InterDiff = interdiff

	return nil
}

// GetRangeDiff git range-diff two pachsets
func (repo *Repository) GetRangeDiff(rev1, rev2 string) ([]RangeDiffEntry, error) {

	stdout, err := NewCommand("range-diff", fmt.Sprintf("%s...%s", rev1, rev2), "--no-color").RunInDir(repo.Path)
	if err != nil {
		return nil, err
	}

	var rangeDiff []RangeDiffEntry

	for _, line := range strings.Split(stdout, "\n") {
		if rangeDiffCommitStartRegex.MatchString(line) {
			fields := strings.Fields(line)
			var diff RangeDiffEntry

			oldIndexString := fields[0][:len(fields[0])-1]

			if !strings.HasPrefix(oldIndexString, "-") {
				oldIndex, err := strconv.ParseInt(oldIndexString, 10, 64)

				if err != nil {
					return nil, err
				}

				diff.OldIndex = &oldIndex
			}

			if !strings.HasPrefix(fields[1], "-") {
				diff.Old = fields[1]
			}

			diff.Relation = fields[2]

			newIndexString := fields[3][:len(fields[3])-1]

			if !strings.HasPrefix(newIndexString, "-") {
				newIndex, err := strconv.ParseInt(newIndexString, 10, 64)

				if err != nil {
					return nil, err
				}

				diff.NewIndex = &newIndex
			}

			if !strings.HasPrefix(fields[4], "-") {
				diff.New = fields[4]
			}

			err = diff.loadDiff(repo)
			if err != nil {
				return nil, err
			}

			if err != nil {
				return nil, err
			}

			rangeDiff = append(rangeDiff, diff)
		}
	}

	return rangeDiff, nil
}
