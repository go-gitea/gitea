// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func (repo *Repository) getTree(id SHA1) (*Tree, error) {
	treePath := filepathFromSHA1(repo.Path, id.String())
	if isFile(treePath) {
		_, err := NewCommand("ls-tree", id.String()).RunInDir(repo.Path)
		if err != nil {
			return nil, ErrNotExist{id.String(), ""}
		}
	}

	return NewTree(repo, id), nil
}

// GetTree find the tree object in the repository.
func (repo *Repository) GetTree(idStr string) (*Tree, error) {
	if len(idStr) != 40 {
		res, err := NewCommand("rev-parse", idStr).RunInDir(repo.Path)
		if err != nil {
			return nil, err
		}
		if len(res) > 0 {
			idStr = res[:len(res)-1]
		}
	}
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}
	return repo.getTree(id)
}

// CommitTreeOpts represents the possible options to CommitTree
type CommitTreeOpts struct {
	Parents   []string
	Message   string
	KeyID     string
	NoGPGSign bool
}

// CommitTree creates a commit from a given tree id for the user with provided message
func (repo *Repository) CommitTree(sig *Signature, tree *Tree, opts CommitTreeOpts) (SHA1, error) {
	commitTimeStr := time.Now().Format(time.UnixDate)

	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+sig.Name,
		"GIT_AUTHOR_EMAIL="+sig.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+sig.Name,
		"GIT_COMMITTER_EMAIL="+sig.Email,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)
	cmd := NewCommand("commit-tree", tree.ID.String())

	for _, parent := range opts.Parents {
		cmd.AddArguments("-p", parent)
	}

	cmd.AddArguments("-m", opts.Message)

	if opts.KeyID != "" {
		cmd.AddArguments(fmt.Sprintf("-S%s", opts.KeyID))
	}

	if opts.NoGPGSign {
		cmd.AddArguments("--no-gpg-sign")
	}

	res, err := cmd.RunInDirWithEnv(repo.Path, env)

	if err != nil {
		return SHA1{}, err
	}
	return NewIDFromString(strings.TrimSpace(res))
}
