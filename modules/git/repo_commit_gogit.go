// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build gogit
// +build gogit

package git

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GetRefCommitID returns the last commit ID string of given reference (branch or tag).
func (repo *Repository) GetRefCommitID(name string) (string, error) {
	ref, err := repo.gogitRepo.Reference(plumbing.ReferenceName(name), true)
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

// ConvertToSHA1 returns a Hash object from a potential ID string
func (repo *Repository) ConvertToSHA1(commitID string) (SHA1, error) {
	if len(commitID) == 40 {
		sha1, err := NewIDFromString(commitID)
		if err == nil {
			return sha1, nil
		}
	}

	actualCommitID, _, err := NewCommand(repo.Ctx, "rev-parse", "--verify", commitID).RunStdString(&RunOpts{Dir: repo.Path})
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision or path") ||
			strings.Contains(err.Error(), "fatal: Needed a single revision") {
			return SHA1{}, ErrNotExist{commitID, ""}
		}
		return SHA1{}, err
	}

	return NewIDFromString(actualCommitID)
}

// IsCommitExist returns true if given commit exists in current repository.
func (repo *Repository) IsCommitExist(name string) bool {
	hash := plumbing.NewHash(name)
	_, err := repo.gogitRepo.CommitObject(hash)
	return err == nil
}

func convertPGPSignatureForTag(t *object.Tag) *CommitGPGSignature {
	if t.PGPSignature == "" {
		return nil
	}

	var w strings.Builder
	var err error

	if _, err = fmt.Fprintf(&w,
		"object %s\ntype %s\ntag %s\ntagger ",
		t.Target.String(), t.TargetType.Bytes(), t.Name); err != nil {
		return nil
	}

	if err = t.Tagger.Encode(&w); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, "\n\n"); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, t.Message); err != nil {
		return nil
	}

	return &CommitGPGSignature{
		Signature: t.PGPSignature,
		Payload:   strings.TrimSpace(w.String()) + "\n",
	}
}

func (repo *Repository) getCommit(id SHA1) (*Commit, error) {
	var tagObject *object.Tag

	gogitCommit, err := repo.gogitRepo.CommitObject(id)
	if err == plumbing.ErrObjectNotFound {
		tagObject, err = repo.gogitRepo.TagObject(id)
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

	if tagObject != nil {
		commit.CommitMessage = strings.TrimSpace(tagObject.Message)
		commit.Author = &tagObject.Tagger
		commit.Signature = convertPGPSignatureForTag(tagObject)
	}

	tree, err := gogitCommit.Tree()
	if err != nil {
		return nil, err
	}

	commit.Tree.ID = tree.Hash
	commit.Tree.gogitTree = tree

	return commit, nil
}
