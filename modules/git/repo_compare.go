// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
)

// GetMergeBase checks and returns merge base of two branches and the reference used as base.
func (repo *Repository) GetMergeBase(tmpRemote, base, head string) (string, string, error) {
	if tmpRemote == "" {
		tmpRemote = "origin"
	}

	if tmpRemote != "origin" {
		tmpBaseName := RemotePrefix + tmpRemote + "/tmp_" + base
		// Fetch commit into a temporary branch in order to be able to handle commits and tags
		_, _, err := gitcmd.NewCommand("fetch", "--no-tags").
			AddDynamicArguments(tmpRemote).
			AddDashesAndList(base + ":" + tmpBaseName).
			WithDir(repo.Path).
			RunStdString(repo.Ctx)
		if err == nil {
			base = tmpBaseName
		}
	}

	stdout, _, err := gitcmd.NewCommand("merge-base").
		AddDashesAndList(base, head).
		WithDir(repo.Path).
		RunStdString(repo.Ctx)
	return strings.TrimSpace(stdout), base, err
}

var patchCommits = regexp.MustCompile(`^From\s(\w+)\s`)

// GetDiffBinary generates and returns patch data between given revisions, including binary diffs.
func (repo *Repository) GetDiffBinary(compareArg string, w io.Writer) error {
	return gitcmd.NewCommand("diff", "-p", "--binary", "--histogram").
		AddDynamicArguments(compareArg).
		WithDir(repo.Path).
		WithStdout(w).
		Run(repo.Ctx)
}

// ReadPatchCommit will check if a diff patch exists and return stats
func (repo *Repository) ReadPatchCommit(prID int64) (commitSHA string, err error) {
	// Migrated repositories download patches to "pulls" location
	patchFile := fmt.Sprintf("pulls/%d.patch", prID)
	loadPatch, err := os.Open(filepath.Join(repo.Path, patchFile))
	if err != nil {
		return "", err
	}
	defer loadPatch.Close()
	// Read only the first line of the patch - usually it contains the first commit made in patch
	scanner := bufio.NewScanner(loadPatch)
	scanner.Scan()
	// Parse the Patch stats, sometimes Migration returns a 404 for the patch file
	commitSHAGroups := patchCommits.FindStringSubmatch(scanner.Text())
	if len(commitSHAGroups) != 0 {
		commitSHA = commitSHAGroups[1]
	} else {
		return "", errors.New("patch file doesn't contain valid commit ID")
	}
	return commitSHA, nil
}
