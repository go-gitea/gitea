// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"fmt"
)

// FileBlame return the Blame object of file
func (repo *Repository) FileBlame(revision, path, file string) ([]byte, error) {
	return NewCommand(repo.Ctx, "blame", "--root", "--", file).RunInDirBytes(path)
}

// LineBlame returns the latest commit at the given line
func (repo *Repository) LineBlame(revision, path, file string, line uint) (*Commit, error) {
	stdout := new(bytes.Buffer)
	err := NewCommand(repo.Ctx, "blame", fmt.Sprintf("-L %d,%d", line, line), "-p", revision, "--", file).RunWithContext(&RunContext{Dir: path, Timeout: -1, Stdout: stdout})
	res := stdout.String()
	if err != nil {
		return nil, err
	}
	if len(res) < 40 {
		return nil, fmt.Errorf("invalid result of blame: %s", res)
	}
	return repo.GetCommit(res[:40])
}
