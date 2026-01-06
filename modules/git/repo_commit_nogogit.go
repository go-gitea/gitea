// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !gogit

package git

import (
	"errors"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/git/catfile"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/log"
)

// ResolveReference resolves a name to a reference
func (repo *Repository) ResolveReference(name string) (string, error) {
	stdout, _, err := gitcmd.NewCommand("show-ref", "--hash").
		AddDynamicArguments(name).
		WithDir(repo.Path).
		RunStdString(repo.Ctx)
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
	objInfo, err := repo.objectPool.ObjectInfo(name)
	if err != nil {
		if catfile.IsErrObjectNotFound(err) {
			return "", ErrNotExist{name, ""}
		}
		return "", err
	}
	return objInfo.ID, nil
}

// IsCommitExist returns true if given commit exists in current repository.
func (repo *Repository) IsCommitExist(name string) bool {
	if err := catfile.EnsureValidGitRepository(repo.Ctx, repo.Path); err != nil {
		log.Error("IsCommitExist: %v", err)
		return false
	}
	_, _, err := gitcmd.NewCommand("cat-file", "-e").
		AddDynamicArguments(name).
		WithDir(repo.Path).
		RunStdString(repo.Ctx)
	return err == nil
}

func (repo *Repository) getCommit(id ObjectID) (*Commit, error) {
	objectInfo, contentReader, err := repo.objectPool.Object(id.String())
	if err != nil {
		if errors.Is(err, io.EOF) || catfile.IsErrObjectNotFound(err) {
			return nil, ErrNotExist{ID: id.String()}
		}
		return nil, err
	}

	switch objectInfo.Type {
	case "missing":
		contentReader.Close()
		return nil, ErrNotExist{ID: id.String()}
	case "tag":
		// then we need to parse the tag
		// and load the commit
		data, err := io.ReadAll(io.LimitReader(contentReader, objectInfo.Size))
		if err != nil {
			contentReader.Close()
			return nil, err
		}
		_, err = contentReader.Discard(1)
		if err != nil {
			contentReader.Close()
			return nil, err
		}
		contentReader.Close()

		tag, err := parseTagData(id.Type(), data)
		if err != nil {
			return nil, err
		}

		commit, err := repo.getCommit(tag.Object)
		if err != nil {
			return nil, err
		}

		return commit, nil
	case "commit":
		defer contentReader.Close()
		commit, err := CommitFromReader(repo, id, io.LimitReader(contentReader, objectInfo.Size))
		if err != nil {
			return nil, err
		}
		_, err = contentReader.Discard(1)
		if err != nil {
			return nil, err
		}

		return commit, nil
	default:
		defer contentReader.Close()
		log.Debug("Unknown typ: %s", objectInfo.Type)
		if err := catfile.DiscardFull(contentReader, objectInfo.Size+1); err != nil {
			return nil, err
		}
		return nil, ErrNotExist{
			ID: id.String(),
		}
	}
}

// ConvertToGitID returns a GitHash object from a potential ID string
func (repo *Repository) ConvertToGitID(commitID string) (ObjectID, error) {
	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return nil, err
	}
	if len(commitID) == objectFormat.FullLength() && objectFormat.IsValid(commitID) {
		ID, err := NewIDFromString(commitID)
		if err == nil {
			return ID, nil
		}
	}

	objInfo, err := repo.objectPool.ObjectInfo(commitID)
	if err != nil {
		if catfile.IsErrObjectNotFound(err) {
			return nil, ErrNotExist{commitID, ""}
		}
		return nil, err
	}
	return MustIDFromString(objInfo.ID), nil
}
