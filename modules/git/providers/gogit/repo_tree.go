// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/service"
	"github.com/go-git/go-git/v5/plumbing"
)

// ___
//  |  ._  _   _
//  |  |  (/_ (/_
//

// GetTree find the tree object in the repository.
func (repo *Repository) GetTree(idStr string) (service.Tree, error) {
	hash := StringHash(idStr)

	if !hash.Valid() {
		res, err := git.NewCommand("rev-parse", "--verify", idStr).RunInDir(repo.Path())
		if err != nil {
			return nil, err
		}
		if len(res) > 0 {
			idStr = res[:len(res)-1]
		}
		hash = StringHash(idStr)
	}
	id := ToPlumbingHash(hash)

	resolvedID := hash
	commitObject, err := repo.gogitRepo.CommitObject(id)
	if err == nil {
		id = commitObject.TreeHash
	}

	gogitTree, err := repo.gogitRepo.TreeObject(id)
	if err != nil {
		if err == plumbing.ErrObjectNotFound {
			return nil, git.ErrNotExist{
				ID: idStr,
			}
		}
		return nil, err
	}

	return &Tree{
		Object: Object{
			hash: hash,
			repo: repo,
		},
		gogitTree:  gogitTree,
		resolvedID: resolvedID,
	}, nil
}

// CommitTree creates a commit from a given tree id for the user with provided message
func (repo *Repository) CommitTree(author *service.Signature, committer *service.Signature, tree service.Tree, opts service.CommitTreeOpts) (service.Hash, error) {
	err := git.LoadGitVersion()
	if err != nil {
		return SHA1{}, err
	}

	commitTimeStr := time.Now().Format(time.RFC3339)

	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+author.Name,
		"GIT_AUTHOR_EMAIL="+author.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+committer.Name,
		"GIT_COMMITTER_EMAIL="+committer.Email,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)
	cmd := git.NewCommand("commit-tree", tree.ID().String())

	for _, parent := range opts.Parents {
		cmd.AddArguments("-p", parent)
	}

	messageBytes := new(bytes.Buffer)
	_, _ = messageBytes.WriteString(opts.Message)
	_, _ = messageBytes.WriteString("\n")

	if git.CheckGitVersionAtLeast("1.7.9") == nil && (opts.KeyID != "" || opts.AlwaysSign) {
		cmd.AddArguments(fmt.Sprintf("-S%s", opts.KeyID))
	}

	if git.CheckGitVersionAtLeast("2.0.0") == nil && opts.NoGPGSign {
		cmd.AddArguments("--no-gpg-sign")
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	err = cmd.RunInDirTimeoutEnvFullPipeline(env, -1, repo.Path(), stdout, stderr, messageBytes)

	if err != nil {
		return StringHash(""), git.ConcatenateError(err, stderr.String())
	}
	return StringHash(strings.TrimSpace(stdout.String())), nil
}
