// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bytes"
	"container/list"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
	"code.gitea.io/gitea/modules/log"
)

//  _
// /   _  ._ _  ._   _. ._  _
// \_ (_) | | | |_) (_| |  (/_
//              |

// GetMergeBase checks and returns merge base of two branches and the reference used as base.
func (repo *Repository) GetMergeBase(tmpRemote string, base, head string) (string, string, error) {
	if tmpRemote == "" {
		tmpRemote = "origin"
	}

	if tmpRemote != "origin" {
		tmpBaseName := "refs/remotes/" + tmpRemote + "/tmp_" + base
		// Fetch commit into a temporary branch in order to be able to handle commits and tags
		_, err := git.NewCommand("fetch", tmpRemote, base+":"+tmpBaseName).RunInDir(repo.Path())
		if err == nil {
			base = tmpBaseName
		}
	}

	stdout, err := git.NewCommand("merge-base", "--", base, head).RunInDir(repo.Path())
	return strings.TrimSpace(stdout), base, err
}

// GetCompareInfo generates and returns compare information between base and head branches of repositories.
func (repo *Repository) GetCompareInfo(basePath, baseBranch, headBranch string) (_ *service.CompareInfo, err error) {
	var (
		remoteBranch string
		tmpRemote    string
	)

	// We don't need a temporary remote for same repository.
	if repo.Path() != basePath {
		// Add a temporary remote
		tmpRemote = strconv.FormatInt(time.Now().UnixNano(), 10)
		if err = repo.AddRemote(tmpRemote, basePath, false); err != nil {
			return nil, fmt.Errorf("AddRemote: %v", err)
		}
		defer func() {
			if err := repo.RemoveRemote(tmpRemote); err != nil {
				log.Error("GetPullRequestInfo: RemoveRemote: %v", err)
			}
		}()
	}

	compareInfo := new(service.CompareInfo)
	compareInfo.MergeBase, remoteBranch, err = repo.GetMergeBase(tmpRemote, baseBranch, headBranch)
	if err == nil {
		// FIXME: use a reader here.
		// We have a common base - therefore we know that ... should work
		logs, err := git.NewCommand("log", compareInfo.MergeBase+"..."+headBranch, LogHashFormat).RunInDirBytes(repo.Path())
		if err != nil {
			return nil, err
		}
		compareInfo.Commits, err = BatchReadCommits(repo, bytes.NewReader(logs))
		if err != nil {
			return nil, fmt.Errorf("parsePrettyFormatLogToList: %v", err)
		}
	} else {
		compareInfo.Commits = list.New()
		compareInfo.MergeBase, err = gitService.GetFullCommitID(repo.Path(), remoteBranch)
		if err != nil {
			compareInfo.MergeBase = remoteBranch
		}
	}

	// Count number of changed files.
	// This probably should be removed as we need to use shortstat elsewhere
	// Now there is git diff --shortstat but this appears to be slower than simply iterating with --nameonly
	compareInfo.NumFiles, err = repo.GetDiffNumChangedFiles(remoteBranch, headBranch)
	if err != nil {
		return nil, err
	}
	return compareInfo, nil
}

type lineCountWriter struct {
	numLines int
}

// Write counts the number of newlines in the provided bytestream
func (l *lineCountWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	l.numLines += bytes.Count(p, []byte{'\000'})
	return
}

// GetDiffNumChangedFiles counts the number of changed files
// This is substantially quicker than shortstat but...
func (repo *Repository) GetDiffNumChangedFiles(base, head string) (int, error) {
	// Now there is git diff --shortstat but this appears to be slower than simply iterating with --nameonly
	w := &lineCountWriter{}
	stderr := new(bytes.Buffer)

	if err := git.NewCommand("diff", "-z", "--name-only", base+"..."+head).
		RunInDirPipeline(repo.Path(), w, stderr); err != nil {
		if strings.Contains(stderr.String(), "no merge base") {
			// git >= 2.28 now returns an error if base and head have become unrelated.
			// previously it would return the results of git diff -z --name-only base head so let's try that...
			w = &lineCountWriter{}
			stderr.Reset()
			if err = git.NewCommand("diff", "-z", "--name-only", base, head).RunInDirPipeline(repo.Path(), w, stderr); err == nil {
				return w.numLines, nil
			}
		}
		return 0, fmt.Errorf("%v: Stderr: %s", err, stderr)
	}
	return w.numLines, nil
}

// GetDiffShortStat counts number of changed files, number of additions and deletions
func (repo *Repository) GetDiffShortStat(base, head string) (numFiles, totalAdditions, totalDeletions int, err error) {
	numFiles, totalAdditions, totalDeletions, err = git.GetDiffShortStat(repo.Path(), base+"..."+head)
	if err != nil && strings.Contains(err.Error(), "no merge base") {
		return git.GetDiffShortStat(repo.Path(), base, head)
	}
	return
}

// GetDiffOrPatch generates either diff or formatted patch data between given revisions
func (repo *Repository) GetDiffOrPatch(base, head string, w io.Writer, formatted bool) error {
	if formatted {
		return repo.GetPatch(base, head, w)
	}
	return repo.GetDiff(base, head, w)
}

// GetDiff generates and returns patch data between given revisions.
func (repo *Repository) GetDiff(base, head string, w io.Writer) error {
	return git.NewCommand("diff", "-p", "--binary", base, head).
		RunInDirPipeline(repo.Path(), w, nil)
}

// GetPatch generates and returns format-patch data between given revisions.
func (repo *Repository) GetPatch(base, head string, w io.Writer) error {
	stderr := new(bytes.Buffer)
	err := git.NewCommand("format-patch", "--binary", "--stdout", base+"..."+head).
		RunInDirPipeline(repo.Path(), w, stderr)
	if err != nil && bytes.Contains(stderr.Bytes(), []byte("no merge base")) {
		return git.NewCommand("format-patch", "--binary", "--stdout", base, head).
			RunInDirPipeline(repo.Path(), w, nil)
	}
	return err
}

// GetDiffFromMergeBase generates and return patch data from merge base to head
func (repo *Repository) GetDiffFromMergeBase(base, head string, w io.Writer) error {
	stderr := new(bytes.Buffer)
	err := git.NewCommand("diff", "-p", "--binary", base+"..."+head).
		RunInDirPipeline(repo.Path(), w, stderr)
	if err != nil && bytes.Contains(stderr.Bytes(), []byte("no merge base")) {
		return git.NewCommand("diff", "-p", "--binary", base, head).
			RunInDirPipeline(repo.Path(), w, nil)
	}
	return err
}
