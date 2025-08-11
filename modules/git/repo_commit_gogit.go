// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/hash"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GetRefCommitID returns the last commit ID string of given reference.
func (repo *Repository) GetRefCommitID(name string) (string, error) {
	if plumbing.IsHash(name) {
		return name, nil
	}
	refName := plumbing.ReferenceName(name)
	if err := refName.Validate(); err != nil {
		return "", err
	}
	ref, err := repo.gogitRepo.Reference(refName, true)
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return "", ErrNotExist{
				ID: name,
			}
		}
		return "", err
	}

	return ref.Hash().String(), nil
}

// SetReference sets the commit ID string of given reference (e.g. branch or tag).
func (repo *Repository) SetReference(name, commitID string) error {
	return repo.gogitRepo.Storer.SetReference(plumbing.NewReferenceFromStrings(name, commitID))
}

// RemoveReference removes the given reference (e.g. branch or tag).
func (repo *Repository) RemoveReference(name string) error {
	return repo.gogitRepo.Storer.RemoveReference(plumbing.ReferenceName(name))
}

// ConvertToHash returns a Hash object from a potential ID string
func (repo *Repository) ConvertToGitID(commitID string) (ObjectID, error) {
	objectFormat, err := repo.GetObjectFormat()
	if err != nil {
		return nil, err
	}
	if len(commitID) == hash.HexSize && objectFormat.IsValid(commitID) {
		ID, err := NewIDFromString(commitID)
		if err == nil {
			return ID, nil
		}
	}

	actualCommitID, _, err := NewCommand("rev-parse", "--verify").AddDynamicArguments(commitID).RunStdString(repo.Ctx, &RunOpts{Dir: repo.Path})
	actualCommitID = strings.TrimSpace(actualCommitID)
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision or path") ||
			strings.Contains(err.Error(), "fatal: Needed a single revision") {
			return objectFormat.EmptyObjectID(), ErrNotExist{commitID, ""}
		}
		return objectFormat.EmptyObjectID(), err
	}

	return NewIDFromString(actualCommitID)
}

// IsCommitExist returns true if given commit exists in current repository.
func (repo *Repository) IsCommitExist(name string) bool {
	hash, err := repo.ConvertToGitID(name)
	if err != nil {
		return false
	}
	_, err = repo.gogitRepo.CommitObject(plumbing.Hash(hash.RawValue()))
	return err == nil
}

func (repo *Repository) getCommit(id ObjectID) (*Commit, error) {
	var tagObject *object.Tag

	commitID := plumbing.Hash(id.RawValue())
	gogitCommit, err := repo.gogitRepo.CommitObject(commitID)
	if err == plumbing.ErrObjectNotFound {
		tagObject, err = repo.gogitRepo.TagObject(commitID)
		if err == plumbing.ErrObjectNotFound {
			return nil, ErrNotExist{
				ID: id.String(),
			}
		}
		if err == nil {
			gogitCommit, err = repo.gogitRepo.CommitObject(tagObject.Target)
		}
		// if we get a plumbing.ErrObjectNotFound here then the repository is broken and it should be 500
	}
	if err != nil {
		return nil, err
	}

	commit := convertCommit(gogitCommit)
	commit.repo = repo

	tree, err := gogitCommit.Tree()
	if err != nil {
		return nil, err
	}

	commit.Tree.ID = ParseGogitHash(tree.Hash)
	commit.Tree.gogitTree = tree

	return commit, nil
}
