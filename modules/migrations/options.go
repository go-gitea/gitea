// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "net/url"

// MigrationSource represents a migration source
type MigrationSource string

// enumerates all MigrationSources
const (
	MigrateFromPlainGit                 = "git"
	MigrateFromGithub   MigrationSource = "github"
)

// MigrateOptions defines the way a repository gets migrated
type MigrateOptions struct {
	RemoteURL    string
	AuthUsername string
	AuthPassword string
	Name         string

	Wiki         bool
	Issues       bool
	Milestones   bool
	Labels       bool
	Comments     bool
	PullRequests bool
	Private      bool
	Mirror       bool
}

// Source returns the migration source
func (opts MigrateOptions) Source() (MigrationSource, error) {
	u, err := url.Parse(opts.RemoteURL)
	if err != nil {
		return "", err
	}

	switch u.Host {
	case "github.com":
		return MigrateFromGithub, nil
	}
	return MigrateFromPlainGit, nil
}
