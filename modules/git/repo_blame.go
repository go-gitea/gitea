// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import "fmt"

// FileBlame return the Blame object of file
func (repo *Repository) FileBlame(revision, path, file string) ([]byte, error) {
	stdout, _, err := NewCommand(repo.Ctx, "blame", "--root").AddDashesAndList(file).RunStdBytes(&RunOpts{Dir: path})
	return stdout, err
}

// LineBlame returns the latest commit at the given line
func (repo *Repository) LineBlame(revision, path, file string, line uint) (*Commit, error) {
	res, _, err := NewCommand(repo.Ctx, "blame").
		AddArguments(CmdArg(fmt.Sprintf("-L %d,%d", line, line))).
		AddArguments("-p").AddDynamicArguments(revision).
		AddDashesAndList(file).RunStdString(&RunOpts{Dir: path})
	if err != nil {
		return nil, err
	}
	if len(res) < 40 {
		return nil, fmt.Errorf("invalid result of blame: %s", res)
	}
	return repo.GetCommit(res[:40])
}
