// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
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
			_ = stdoutWriter.CloseWithError(ConcatenateError(err, stderr.String()))
		} else {
			_ = stdoutWriter.Close()
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
			_ = stdoutReader.CloseWithError(err)
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
		_ = stdoutReader.CloseWithError(fmt.Errorf("unknown typ: %s", typ))
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
