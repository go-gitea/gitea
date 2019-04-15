// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

// MigrateOptions defines the way a repository gets migrated
type MigrateOptions struct {
	RemoteURL    string
	AuthUsername string
	AuthPassword string
	Name         string
	Description  string

	Wiki              bool
	Issues            bool
	Milestones        bool
	Labels            bool
	Releases          bool
	Comments          bool
	PullRequests      bool
	Private           bool
	Mirror            bool
	IgnoreIssueAuthor bool // if true will not add original author information before issues or comments content.
}
