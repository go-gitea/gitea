// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gituser

import (
	"context"
	"net/url"

	"gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
)

// CommitParticipant is one participant of a commit (its author or a co-author):
// a git identity, optionally matched to a Gitea user.
type CommitParticipant struct {
	GitIdentity *git.CommitIdentity // git identity (name/email), never nil
	GiteaUser   *user.User          // matched Gitea user, nil if unmatched
}

// UserCommit represents a commit with matched of database "author" user.
type UserCommit struct {
	GitCommit       *git.Commit
	AuthorUser      *user.User
	AvatarStackData *AvatarStackData
}

func RepoCommitSearchByEmailLink(repoLink string, ref git.RefName) string {
	if curRefWebLinkPath := ref.RefWebLinkPath(); curRefWebLinkPath != "" {
		return repoLink + "/commits/" + curRefWebLinkPath + "/search?q=" + url.QueryEscape("author:") + "{email}"
	}
	return ""
}

// GetUserCommitsByGitCommits checks if authors' e-mails of commits are corresponding to users.
func GetUserCommitsByGitCommits(ctx context.Context, gitCommits []*git.Commit, repoLink string, currentRef git.RefName) ([]*UserCommit, error) {
	userCommits := make([]*UserCommit, 0, len(gitCommits))
	emailSet := make(container.Set[string])
	for _, c := range gitCommits {
		emailSet.Add(c.Author.Email)
		emailSet.Add(c.Committer.Email)
		for _, p := range c.AllParticipantIdentities() {
			emailSet.Add(p.Email)
		}
	}

	emailUserMap, err := user.GetUsersByEmails(ctx, emailSet.Values())
	if err != nil {
		return nil, err
	}

	searchByEmailLink := RepoCommitSearchByEmailLink(repoLink, currentRef)
	for _, c := range gitCommits {
		uc := &UserCommit{
			AuthorUser:      emailUserMap.GetByEmail(c.Author.Email), // FIXME: why GetUserCommitsByGitCommits uses "Author", but ParseCommitsWithSignature uses "Committer"?
			GitCommit:       c,
			AvatarStackData: BuildAvatarStackData(ctx, c.AllParticipantIdentities(), emailUserMap),
		}
		uc.AvatarStackData.SearchByEmailLink = searchByEmailLink
		userCommits = append(userCommits, uc)
	}
	return userCommits, nil
}
