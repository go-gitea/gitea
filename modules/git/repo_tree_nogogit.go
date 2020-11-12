// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build nogogit

package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

func (repo *Repository) getTree(id SHA1) (*Tree, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go func() {
		stderr := &strings.Builder{}
		err := NewCommand("cat-file", "--batch").RunInDirFullPipeline(repo.Path, stdoutWriter, stderr, strings.NewReader(id.String()+"\n"))
		if err != nil {
			stdoutWriter.CloseWithError(ConcatenateError(err, stderr.String()))
		} else {
			stdoutWriter.Close()
		}
	}()

	bufReader := bufio.NewReader(stdoutReader)
	// ignore the SHA
	_, typ, _, err := ReadBatchLine(bufReader)
	if err != nil {
		return nil, err
	}

	switch typ {
	case "tag":
		resolvedID := id
		data, err := ioutil.ReadAll(bufReader)
		if err != nil {
			return nil, err
		}
		tag, err := parseTagData(data)
		if err != nil {
			return nil, err
		}
		commit, err := tag.Commit()
		if err != nil {
			return nil, err
		}
		commit.Tree.ResolvedID = resolvedID
		log("tag.commit.Tree: %s %v", commit.Tree.ID.String(), commit.Tree.repo)
		return &commit.Tree, nil
	case "commit":
		commit, err := CommitFromReader(repo, id, bufReader)
		if err != nil {
			stdoutReader.CloseWithError(err)
			return nil, err
		}
		commit.Tree.ResolvedID = commit.ID
		log("commit.Tree: %s %v", commit.Tree.ID.String(), commit.Tree.repo)
		return &commit.Tree, nil
	case "tree":
		stdoutReader.Close()
		tree := NewTree(repo, id)
		tree.ResolvedID = id
		return tree, nil
	default:
		stdoutReader.CloseWithError(fmt.Errorf("unknown typ: %s", typ))
		return nil, ErrNotExist{
			ID: id.String(),
		}
	}
}

// GetTree find the tree object in the repository.
func (repo *Repository) GetTree(idStr string) (*Tree, error) {
	if len(idStr) != 40 {
		res, err := NewCommand("rev-parse", "--verify", idStr).RunInDir(repo.Path)
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
	Parents    []string
	Message    string
	KeyID      string
	NoGPGSign  bool
	AlwaysSign bool
}

// CommitTree creates a commit from a given tree id for the user with provided message
func (repo *Repository) CommitTree(author *Signature, committer *Signature, tree *Tree, opts CommitTreeOpts) (SHA1, error) {
	err := LoadGitVersion()
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
	cmd := NewCommand("commit-tree", tree.ID.String())

	for _, parent := range opts.Parents {
		cmd.AddArguments("-p", parent)
	}

	messageBytes := new(bytes.Buffer)
	_, _ = messageBytes.WriteString(opts.Message)
	_, _ = messageBytes.WriteString("\n")

	if CheckGitVersionAtLeast("1.7.9") == nil && (opts.KeyID != "" || opts.AlwaysSign) {
		cmd.AddArguments(fmt.Sprintf("-S%s", opts.KeyID))
	}

	if CheckGitVersionAtLeast("2.0.0") == nil && opts.NoGPGSign {
		cmd.AddArguments("--no-gpg-sign")
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	err = cmd.RunInDirTimeoutEnvFullPipeline(env, -1, repo.Path, stdout, stderr, messageBytes)

	if err != nil {
		return SHA1{}, ConcatenateError(err, stderr.String())
	}
	return NewIDFromString(strings.TrimSpace(stdout.String()))
}
