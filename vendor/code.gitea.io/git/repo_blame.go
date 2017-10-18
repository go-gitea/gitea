// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

// FileBlame return the Blame object of file
func (repo *Repository) FileBlame(revision, path, file string) ([]byte, error) {
	return NewCommand("blame", "--root", file).RunInDirBytes(path)
}
