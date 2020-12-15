// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/common"
	"code.gitea.io/gitea/modules/git/service"
	"code.gitea.io/gitea/modules/log"
)

// ___
//  |  ._  _   _
//  |  |  (/_ (/_
//

// GetTree find the tree object in the repository.
func (repo *Repository) GetTree(idStr string) (service.Tree, error) {
	id := StringHash(idStr)

	if !id.Valid() {
		res, err := git.NewCommand("rev-parse", "--verify", idStr).RunInDir(repo.Path())
		if err != nil {
			return nil, err
		}
		if len(res) > 0 {
			idStr = res[:len(res)-1]
		}
		id = StringHash(idStr)
	}

	return repo.getTree(id)
}

func (repo *Repository) getTree(id service.Hash) (*Tree, error) {

	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdinReader.Close()
		_ = stdinWriter.Close()
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go common.PipeCommand(git.NewCommand("cat-file", "--batch"),
		repo.Path(),
		stdoutWriter,
		stdinReader)

	// Write the initial ID
	if _, err := stdinWriter.Write([]byte(id.String() + "\n")); err != nil {
		return nil, err
	}

	// Create buffered reader from the stdout
	bufReader := bufio.NewReader(stdoutReader)
	var treeID service.Hash
	for treeID == nil {
		_, typ, size, err := ReadBatchLine(bufReader)
		if err != nil {
			return nil, err
		}
		switch typ {
		case "tag":
			objectIDStr, err := ReadTagObjectID(bufReader, size)
			if err != nil {
				return nil, err
			}
			// Write the object ID
			if _, err := stdinWriter.Write([]byte(objectIDStr + "\n")); err != nil {
				return nil, err
			}
		case "commit":
			treeIDStr, err := ReadTreeID(bufReader, size)
			if err != nil {
				return nil, err
			}
			treeID = StringHash(treeIDStr)
			_ = stdinWriter.Close()
		case "tree":
			treeID = id
			_ = stdinWriter.Close()
		default:
			_ = stdoutReader.CloseWithError(fmt.Errorf("unknown typ: %s", typ))
			log.Error("ID %s is not a tree object or doesn't contain a tree. underlying type: %s", id.String(), typ)
			_ = stdinWriter.Close()
			return nil, git.ErrNotExist{
				ID: id.String(),
			}

		}
	}

	return &Tree{Object: Object{
		hash: treeID,
		repo: repo,
	},
		resolvedID: id,
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
