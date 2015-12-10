// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

func (repo *Repository) getTree(id sha1) (*Tree, error) {
	treePath := filepathFromSHA1(repo.Path, id.String())
	if isFile(treePath) {
		_, err := NewCommand("ls-tree", id.String()).RunInDir(repo.Path)
		if err != nil {
			return nil, ErrNotExist{id.String(), ""}
		}
	}

	return NewTree(repo, id), nil
}

// Find the tree object in the repository.
func (repo *Repository) GetTree(idStr string) (*Tree, error) {
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}
	return repo.getTree(id)
}
