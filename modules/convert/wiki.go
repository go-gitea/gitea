// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"time"

	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	wiki_service "code.gitea.io/gitea/services/wiki"
)

// ToWikiCommit convert a git commit into a WikiCommit
func ToWikiCommit(commit *git.Commit) *api.WikiCommit {
	return &api.WikiCommit{
		ID: commit.ID.String(),
		Author: &api.CommitUser{
			Identity: api.Identity{
				Name:  commit.Author.Name,
				Email: commit.Author.Email,
			},
			Date: commit.Author.When.UTC().Format(time.RFC3339),
		},
		Committer: &api.CommitUser{
			Identity: api.Identity{
				Name:  commit.Committer.Name,
				Email: commit.Committer.Email,
			},
			Date: commit.Committer.When.UTC().Format(time.RFC3339),
		},
		Message: commit.CommitMessage,
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

// ToWikiPage converts different data to a WikiPage
func ToWikiPage(title string, lastCommit *git.Commit, commitsCount int64, content []byte, sidebarContent []byte, footerContent []byte) *api.WikiPage {
	return &api.WikiPage{
		WikiPageMetaData: ToWikiPageMetaData(title, lastCommit.Author.When),
		Content:          string(content),
		CommitCount:      commitsCount,
		LastCommit:       ToWikiCommit(lastCommit),
		Sidebar:          string(sidebarContent),
		Footer:           string(footerContent),
	}
}

// ToWikiPageMetaData converts meta information to a WikiPageMetaData
func ToWikiPageMetaData(page string, updated time.Time) *api.WikiPageMetaData {
	return &api.WikiPageMetaData{
		Title:   page,
		SubURL:  wiki_service.NameToSubURL(page),
		Updated: updated,
	}
}
