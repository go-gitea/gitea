// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package service

// SubModuleFile represents a sub module file
type SubModuleFile interface {
	// Commit returns the commit this submodule file is associated with
	Commit() Commit

	// RefURL guesses and returns reference URL.
	RefURL(urlPrefix, repoFullName, sshDomain string) string

	// RefID returns reference ID.
	RefID() string
}

// SubModule submodule is a reference on git repository
type SubModule struct {
	Name string
	URL  string
}
