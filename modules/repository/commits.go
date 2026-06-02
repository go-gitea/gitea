// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"net/url"
	"time"

	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/gitrepo"
	api "gitea.dev/modules/structs"
)

// PushCommit represents a commit in a push operation.
// This struct is marshaled as JSON (see ActionContent2Commits)
type PushCommit struct {
	Sha1           string
	Message        string
	AuthorEmail    string
	AuthorName     string
	CommitterEmail string
	CommitterName  string
	Timestamp      time.Time
}

// PushCommits represents list of commits in a push operation.
// This struct is marshaled as JSON (see ActionContent2Commits)
type PushCommits struct {
	Commits    []*PushCommit
	HeadCommit *PushCommit
	CompareURL string
	Len        int
}

// NewPushCommits creates a new PushCommits object.
func NewPushCommits() *PushCommits {
	return &PushCommits{}
}

// ToAPIPayloadCommit converts a single PushCommit to an api.PayloadCommit object.
func ToAPIPayloadCommit(ctx context.Context, emailUsers map[string]*user_model.User, repo *repo_model.Repository, commit *PushCommit) (*api.PayloadCommit, error) {
	var err error
	authorUsername := ""
	author, ok := emailUsers[commit.AuthorEmail]
	if !ok {
		author, err = user_model.GetUserByEmail(ctx, commit.AuthorEmail)
		if err == nil {
			authorUsername = author.Name
			emailUsers[commit.AuthorEmail] = author
		}
	} else {
		authorUsername = author.Name
	}

	committerUsername := ""
	committer, ok := emailUsers[commit.CommitterEmail]
	if !ok {
		committer, err = user_model.GetUserByEmail(ctx, commit.CommitterEmail)
		if err == nil {
			// TODO: check errors other than email not found.
			committerUsername = committer.Name
			emailUsers[commit.CommitterEmail] = committer
		}
	} else {
		committerUsername = committer.Name
	}

	fileStatus, err := gitrepo.GetCommitFileStatus(ctx, repo, commit.Sha1)
	if err != nil {
		return nil, fmt.Errorf("FileStatus [commit_sha1: %s]: %w", commit.Sha1, err)
	}

	return &api.PayloadCommit{
		ID:      commit.Sha1,
		Message: commit.Message,
		URL:     fmt.Sprintf("%s/commit/%s", repo.HTMLURL(), url.PathEscape(commit.Sha1)),
		Author: &api.PayloadUser{
			Name:     commit.AuthorName,
			Email:    commit.AuthorEmail,
			UserName: authorUsername,
		},
		Committer: &api.PayloadUser{
			Name:     commit.CommitterName,
			Email:    commit.CommitterEmail,
			UserName: committerUsername,
		},
		Added:     fileStatus.Added,
		Removed:   fileStatus.Removed,
		Modified:  fileStatus.Modified,
		Timestamp: commit.Timestamp,
	}, nil
}

// ToAPIPayloadCommits converts a PushCommits object to api.PayloadCommit format.
// It returns all converted commits and, if provided, the head commit or an error otherwise.
func (pc *PushCommits) ToAPIPayloadCommits(ctx context.Context, repo *repo_model.Repository) ([]*api.PayloadCommit, *api.PayloadCommit, error) {
	commits := make([]*api.PayloadCommit, len(pc.Commits))
	var headCommit *api.PayloadCommit

	emailUsers := make(map[string]*user_model.User)

	for i, commit := range pc.Commits {
		apiCommit, err := ToAPIPayloadCommit(ctx, emailUsers, repo, commit)
		if err != nil {
			return nil, nil, err
		}

		commits[i] = apiCommit
		if pc.HeadCommit != nil && pc.HeadCommit.Sha1 == commits[i].ID {
			headCommit = apiCommit
		}
	}
	if pc.HeadCommit != nil && headCommit == nil {
		var err error
		headCommit, err = ToAPIPayloadCommit(ctx, emailUsers, repo, pc.HeadCommit)
		if err != nil {
			return nil, nil, err
		}
	}
	return commits, headCommit, nil
}

// CommitToPushCommit transforms a git.Commit to PushCommit type.
func CommitToPushCommit(commit *git.Commit) *PushCommit {
	return &PushCommit{
		Sha1:           commit.ID.String(),
		Message:        commit.MessageUTF8(),
		AuthorEmail:    commit.Author.Email,
		AuthorName:     commit.Author.Name,
		CommitterEmail: commit.Committer.Email,
		CommitterName:  commit.Committer.Name,
		Timestamp:      commit.Author.When,
	}
}

// GitToPushCommits transforms a list of git.Commits to PushCommits type.
func GitToPushCommits(gitCommits []*git.Commit) *PushCommits {
	commits := make([]*PushCommit, 0, len(gitCommits))
	for _, commit := range gitCommits {
		commits = append(commits, CommitToPushCommit(commit))
	}
	return &PushCommits{
		Commits:    commits,
		HeadCommit: nil,
		CompareURL: "",
		Len:        len(commits),
	}
}
