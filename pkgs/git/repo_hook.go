// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

// GetHook get one hook according the name on a repository
func (repo *Repository) GetHook(name string) (*Hook, error) {
	return GetHook(repo.Path, name)
}

// Hooks get all the hooks on the repository
func (repo *Repository) Hooks() ([]*Hook, error) {
	return ListHooks(repo.Path)
}
