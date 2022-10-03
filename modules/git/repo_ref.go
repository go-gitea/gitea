// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

// GetRefs returns all references of the repository.
func (repo *Repository) GetRefs() ([]*Reference, error) {
	return repo.GetRefsFiltered("")
}

// GetReference gets the Reference object that a refName refers to
func (repo *Repository) GetReference(refName string) (*Reference, error) {
	refs, err := repo.GetRefsFiltered(refName)
	if err != nil {
		return nil, err
	}
	var ref *Reference
	for _, ref = range refs {
		if ref.Name == refName {
			return ref, nil
		}
	}
	return nil, ErrRefNotFound{RefName: refName}
}
