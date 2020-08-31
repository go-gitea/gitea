// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
)

const revisionDirRef = "refs/pull/%d/revision"
const revisionRef = "refs/pull/%d/revision/%d"
const latestRevisionRef = "refs/pull/%d/revision/latest"

// GetRevisionRefs Gets the list of revisions of a PR specified by prIndex.
func (repo *Repository) GetRevisionRefs(prIndex int64) ([]*Reference, error) {
	refdir := fmt.Sprintf(revisionDirRef, prIndex)

	var revisions, error = repo.GetRefsFiltered(refdir)
	if error != nil {
		return nil, error
	}

	return revisions, nil
}

// GetLastRevisionIndex Gets the last used index for the revisions versions of a PR.
func (repo *Repository) GetLastRevisionIndex(prIndex int64) (int64, error) {
	var refs, error = repo.GetRevisionRefs(prIndex)
	if error != nil {
		return -1, error
	}

	var max int64
	max = 0

	for _, ref := range refs {
		index := GetRevisionIndexFromRef(ref.Name)

		if index != nil && *index > max {
			max = *index
		}
	}

	return max, nil
}

func (repo *Repository) getNextRevisionIndex(prIndex int64) (int64, error) {
	var max, error = repo.GetLastRevisionIndex(prIndex)
	if error != nil {
		return -1, error
	}

	return max + 1, nil
}

// InitializeRevisions Enables revisioning and creates the first revision for a PR.
func (repo *Repository) InitializeRevisions(prIndex int64, commit string) (int64, error) {
	var latestRef = fmt.Sprintf(latestRevisionRef, prIndex)

	error := repo.UpsertRef(latestRef, commit)

	if error != nil {
		return -1, error
	}

	return repo.CreateNewRevision(prIndex, commit)
}

// CreateNewRevision Creates a new revision for the PR specified by prIndex.
func (repo *Repository) CreateNewRevision(prIndex int64, commit string) (int64, error) {
	var latestRef = fmt.Sprintf(latestRevisionRef, prIndex)
	_, err := repo.GetRefCommitID(latestRef)
	if err != nil {
		// this is an old PR without previous revision information,
		// new pushes shouldn't just start to create revisions
		return -1, nil
	}

	// FIXME: this is racy. Post receive hooks don't block concurrent pushes to the same branch. :(
	var next, error = repo.getNextRevisionIndex(prIndex)
	if error != nil {
		return -1, error
	}
	ref := fmt.Sprintf(revisionRef, prIndex, next)

	if error != nil {
		return -1, error
	}

	error = repo.UpsertRef(ref, commit)

	if error != nil {
		return -1, error
	}

	error = repo.UpsertRef(latestRef, ref)

	if error != nil {
		err := repo.DeleteRef(ref)

		if err != nil {
			return -1, err
		}

		return -1, error
	}

	return next, nil
}

// GetRevisionIndexFromRef parses the index from a git reference.
func GetRevisionIndexFromRef(ref string) *int64 {
	_, index := GetPRIndexAndRevisionIndexFromRef(ref)
	return index
}

// GetPRIndexFromRef parses the index from a git reference.
func GetPRIndexFromRef(ref string) *int64 {
	prIndex, _ := GetPRIndexAndRevisionIndexFromRef(ref)
	return prIndex
}

// GetPRIndexAndRevisionIndexFromRef parses the indexes from a git reference.
func GetPRIndexAndRevisionIndexFromRef(ref string) (*int64, *int64) {
	var prIndex, index int64
	n, err := fmt.Sscanf(ref, revisionRef, &prIndex, &index)

	if err != nil || n != 2 {
		return nil, nil
	}

	return &prIndex, &index
}

// GetRevisionRef returns the git ref for a revision.
func GetRevisionRef(prIndex, index int64) string {
	return fmt.Sprintf(revisionRef, prIndex, index)
}
