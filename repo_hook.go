// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

func (repo *Repository) GetHook(name string) (*Hook, error) {
	return GetHook(repo.Path, name)
}

func (repo *Repository) Hooks() ([]*Hook, error) {
	return ListHooks(repo.Path)
}
