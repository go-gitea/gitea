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

// GetRevisionRefs Gets the list of revisions of a PR specified by prID.
func (repo *Repository) GetRevisionRefs(prID int64) ([]*Reference, error) {
	refdir := fmt.Sprintf(revisionDirRef, prID)

	var revisions, error = repo.GetRefsFiltered(refdir)
	if error != nil {
		return nil, error
	}

	return revisions, nil
}

// GetLastRevisionIndex Gets the last used index for the revisions versions of a PR.
func (repo *Repository) GetLastRevisionIndex(prID int64) (int64, error) {
	var refs, error = repo.GetRevisionRefs(prID)
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

func (repo *Repository) getNextRevisionIndex(prID int64) (int64, error) {
	var max, error = repo.GetLastRevisionIndex(prID)
	if error != nil {
		return -1, error
	}

	return max + 1, nil
}

// InitializeRevisions Enables revisioning and creates the first revision for a PR.
func (repo *Repository) InitializeRevisions(prID int64, commit string) (int64, error) {
	var latestRef = fmt.Sprintf(latestRevisionRef, prID)

	error := repo.UpsertRef(latestRef, commit)

	if error != nil {
		return -1, error
	}

	return repo.CreateNewRevision(prID, commit)
}

// CreateNewRevision Creates a new revision for the PR specified by prID.
func (repo *Repository) CreateNewRevision(prID int64, commit string) (int64, error) {
	var latestRef = fmt.Sprintf(latestRevisionRef, prID)
	_, err := repo.GetRefCommitID(latestRef)
	if err != nil {
		// this is an old PR without previous revision information,
		// new pushes shouldn't just start to create revisions
		return -1, nil
	}

	// FIXME: this is racy. Post receive hooks don't block concurrent pushes to the same branch. :(
	var next, error = repo.getNextRevisionIndex(prID)
	if error != nil {
		return -1, error
	}
	ref := fmt.Sprintf(revisionRef, prID, next)

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
	var prID, index int64
	n, err := fmt.Sscanf(ref, revisionRef, &prID, &index)

	if err != nil || n != 2 {
		return nil
	}

	return &index
}

// GetRevisionRef returns the git ref for a revision.
func GetRevisionRef(prID, index int64) string {
	return fmt.Sprintf(revisionRef, prID, index)
}
