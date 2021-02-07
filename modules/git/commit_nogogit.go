// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"errors"
	"os/exec"
	"strings"
)

// HasPreviousCommit returns true if a given commitHash is contained in commit's parents
func (c *Commit) HasPreviousCommit(commitHash SHA1) (bool, error) {
	if err := CheckGitVersionAtLeast("1.8"); err != nil {
		_, err := NewCommand("merge-base", "--ancestor", commitHash.String(), c.ID.String()).RunInDir(c.repo.Path)
		if err != nil {
			return true, nil
		}
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			if exitError.ProcessState.ExitCode() == 1 && len(exitError.Stderr) == 0 {
				return false, nil
			}
		}
		return false, err
	}

	result, err := NewCommand("rev-list", "-n1", commitHash.String()+".."+c.ID.String(), "--").RunInDir(c.repo.Path)
	if err != nil {
		return false, err
	}

	return len(strings.TrimSpace(result)) > 0, nil
}
