// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
)

// LineBlame returns the latest commit at the given line
func (repo *Repository) LineBlame(revision, path, file string, line uint) (*Commit, error) {
	res, _, err := NewCommand("blame").
		AddOptionFormat("-L %d,%d", line, line).
		AddOptionValues("-p", revision).
		AddDashesAndList(file).RunStdString(repo.Ctx, &RunOpts{Dir: path})
	if err != nil {
		return nil, err
	}
	if len(res) < 40 {
		return nil, fmt.Errorf("invalid result of blame: %s", res)
	}
	return repo.GetCommit(res[:40])
}
