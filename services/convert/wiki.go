// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
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

// ToWikiPageMetaData converts meta information to a WikiPageMetaData
func ToWikiPageMetaData(wikiName wiki_service.WebPath, lastCommit *git.Commit, repo *repo_model.Repository) *api.WikiPageMetaData {
	subURL := string(wikiName)
	_, title := wiki_service.WebPathToUserTitle(wikiName)
	return &api.WikiPageMetaData{
		Title:      title,
		HTMLURL:    util.URLJoin(repo.HTMLURL(), "wiki", subURL),
		SubURL:     subURL,
		LastCommit: ToWikiCommit(lastCommit),
	}
}
