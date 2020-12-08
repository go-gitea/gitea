// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogit

import "code.gitea.io/gitea/modules/git"

//  _
// |_)  _  ._ _   _  _|_  _
// | \ (/_ | | | (_)  |_ (/_
//

// AddRemote adds a new remote to repository.
// FIXME: this is just a copy of the native function
func (repo *Repository) AddRemote(name, url string, fetch bool) error {
	cmd := git.NewCommand("remote", "add")
	if fetch {
		cmd.AddArguments("-f")
	}
	cmd.AddArguments(name, url)

	_, err := cmd.RunInDir(repo.Path())
	return err
}

// RemoveRemote removes a remote from repository.
func (repo *Repository) RemoveRemote(name string) error {
	_, err := git.NewCommand("remote", "rm", name).RunInDir(repo.Path())
	return err
}
