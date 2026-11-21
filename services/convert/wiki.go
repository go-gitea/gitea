// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"gitea.dev/modules/git"
	api "gitea.dev/modules/structs"
)

// ToWikiCommit convert a git commit into a WikiCommit
func ToWikiCommit(commit *git.Commit) *api.WikiCommit {
	return &api.WikiCommit{
		ID:        commit.ID.String(),
		Author:    ToCommitUser(commit.Author),
		Committer: ToCommitUser(commit.Committer),
		Message:   commit.MessageUTF8(),
	}
}

// ToWikiCommitList convert a list of git commits into a WikiCommitList
func ToWikiCommitList(commits []*git.Commit, total int64) *api.WikiCommitList {
	result := make([]*api.WikiCommit, len(commits))
	for i := range commits {
		result[i] = ToWikiCommit(commits[i])
	}
	return &api.WikiCommitList{
		WikiCommits: result,
		Count:       total,
	}
}
