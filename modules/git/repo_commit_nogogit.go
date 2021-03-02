// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

// ResolveReference resolves a name to a reference
func (repo *Repository) ResolveReference(name string) (string, error) {
	stdout, err := NewCommand("show-ref", "--hash", name).RunInDir(repo.Path)
	if err != nil {
		if strings.Contains(err.Error(), "not a valid ref") {
			return "", ErrNotExist{name, ""}
		}
		return "", err
	}
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return "", ErrNotExist{name, ""}
	}

	return stdout, nil
}

// GetRefCommitID returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) GetRefCommitID(name string) (string, error) {
	stdout, err := NewCommand("show-ref", "--verify", "--hash", name).RunInDir(repo.Path)
	if err != nil {
		if strings.Contains(err.Error(), "not a valid ref") {
			return "", ErrNotExist{name, ""}
		}
		return "", err
	}

	return strings.TrimSpace(stdout), nil
}

// IsCommitExist returns true if given commit exists in current repository.
func (repo *Repository) IsCommitExist(name string) bool {
	_, err := NewCommand("cat-file", "-e", name).RunInDir(repo.Path)
	return err == nil
}

func (repo *Repository) getCommit(id SHA1) (*Commit, error) {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go func() {
		stderr := strings.Builder{}
		err := NewCommand("cat-file", "--batch").RunInDirFullPipeline(repo.Path, stdoutWriter, &stderr, strings.NewReader(id.String()+"\n"))
		if err != nil {
			_ = stdoutWriter.CloseWithError(ConcatenateError(err, (&stderr).String()))
		} else {
			_ = stdoutWriter.Close()
		}
	}()

	bufReader := bufio.NewReader(stdoutReader)
	_, typ, size, err := ReadBatchLine(bufReader)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, ErrNotExist{ID: id.String()}
		}
		return nil, err
	}

	switch typ {
	case "missing":
		return nil, ErrNotExist{ID: id.String()}
	case "tag":
		// then we need to parse the tag
		// and load the commit
		data, err := ioutil.ReadAll(io.LimitReader(bufReader, size))
		if err != nil {
			return nil, err
		}
		tag, err := parseTagData(data)
		if err != nil {
			return nil, err
		}
		tag.repo = repo

		commit, err := tag.Commit()
		if err != nil {
			return nil, err
		}

		commit.CommitMessage = strings.TrimSpace(tag.Message)
		commit.Author = tag.Tagger
		commit.Signature = tag.Signature

		return commit, nil
	case "commit":
		return CommitFromReader(repo, id, io.LimitReader(bufReader, size))
	default:
		_ = stdoutReader.CloseWithError(fmt.Errorf("unknown typ: %s", typ))
		log("Unknown typ: %s", typ)
		return nil, ErrNotExist{
			ID: id.String(),
		}
	}
}
