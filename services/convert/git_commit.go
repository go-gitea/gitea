// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"net/url"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	ctx "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/gitdiff"
)

// ToCommitUser convert a git.Signature to an api.CommitUser
func ToCommitUser(sig *git.Signature) *api.CommitUser {
	return &api.CommitUser{
		Identity: api.Identity{
			Name:  sig.Name,
			Email: sig.Email,
		},
		Date: sig.When.UTC().Format(time.RFC3339),
	}
}

// ToCommitMeta convert a git.Tag to an api.CommitMeta
func ToCommitMeta(repo *repo_model.Repository, tag *git.Tag) *api.CommitMeta {
	return &api.CommitMeta{
		SHA:     tag.Object.String(),
		URL:     util.URLJoin(repo.APIURL(), "git/commits", tag.ID.String()),
		Created: tag.Tagger.When,
	}
}

// ToPayloadCommit convert a git.Commit to api.PayloadCommit
func ToPayloadCommit(ctx context.Context, repo *repo_model.Repository, c *git.Commit) *api.PayloadCommit {
	authorUsername := ""
	if author, err := user_model.GetUserByEmail(ctx, c.Author.Email); err == nil {
		authorUsername = author.Name
	} else if !user_model.IsErrUserNotExist(err) {
		log.Error("GetUserByEmail: %v", err)
	}

	committerUsername := ""
	if committer, err := user_model.GetUserByEmail(ctx, c.Committer.Email); err == nil {
		committerUsername = committer.Name
	} else if !user_model.IsErrUserNotExist(err) {
		log.Error("GetUserByEmail: %v", err)
	}

	return &api.PayloadCommit{
		ID:      c.ID.String(),
		Message: c.Message(),
		URL:     util.URLJoin(repo.HTMLURL(), "commit", c.ID.String()),
		Author: &api.PayloadUser{
			Name:     c.Author.Name,
			Email:    c.Author.Email,
			UserName: authorUsername,
		},
		Committer: &api.PayloadUser{
			Name:     c.Committer.Name,
			Email:    c.Committer.Email,
			UserName: committerUsername,
		},
		Timestamp:    c.Author.When,
		Verification: ToVerification(ctx, c),
	}
}

type ToCommitOptions struct {
	Stat         bool
	Verification bool
	Files        bool
}

func ParseCommitOptions(ctx *ctx.APIContext) ToCommitOptions {
	return ToCommitOptions{
		Stat:         ctx.FormString("stat") == "" || ctx.FormBool("stat"),
		Files:        ctx.FormString("files") == "" || ctx.FormBool("files"),
		Verification: ctx.FormString("verification") == "" || ctx.FormBool("verification"),
	}
}

// ToCommit convert a git.Commit to api.Commit
func ToCommit(ctx context.Context, repo *repo_model.Repository, gitRepo *git.Repository, commit *git.Commit, userCache map[string]*user_model.User, opts ToCommitOptions) (*api.Commit, error) {
	var apiAuthor, apiCommitter *api.User

	// Retrieve author and committer information

	var cacheAuthor *user_model.User
	var ok bool
	if userCache == nil {
		cacheAuthor = (*user_model.User)(nil)
		ok = false
	} else {
		cacheAuthor, ok = userCache[commit.Author.Email]
	}

	if ok {
		apiAuthor = ToUser(ctx, cacheAuthor, nil)
	} else {
		author, err := user_model.GetUserByEmail(ctx, commit.Author.Email)
		if err != nil && !user_model.IsErrUserNotExist(err) {
			return nil, err
		} else if err == nil {
			apiAuthor = ToUser(ctx, author, nil)
			if userCache != nil {
				userCache[commit.Author.Email] = author
			}
		}
	}

	var cacheCommitter *user_model.User
	if userCache == nil {
		cacheCommitter = (*user_model.User)(nil)
		ok = false
	} else {
		cacheCommitter, ok = userCache[commit.Committer.Email]
	}

	if ok {
		apiCommitter = ToUser(ctx, cacheCommitter, nil)
	} else {
		committer, err := user_model.GetUserByEmail(ctx, commit.Committer.Email)
		if err != nil && !user_model.IsErrUserNotExist(err) {
			return nil, err
		} else if err == nil {
			apiCommitter = ToUser(ctx, committer, nil)
			if userCache != nil {
				userCache[commit.Committer.Email] = committer
			}
		}
	}

	// Retrieve parent(s) of the commit
	apiParents := make([]*api.CommitMeta, commit.ParentCount())
	for i := 0; i < commit.ParentCount(); i++ {
		sha, _ := commit.ParentID(i)
		apiParents[i] = &api.CommitMeta{
			URL: repo.APIURL() + "/git/commits/" + url.PathEscape(sha.String()),
			SHA: sha.String(),
		}
	}

	res := &api.Commit{
		CommitMeta: &api.CommitMeta{
			URL:     repo.APIURL() + "/git/commits/" + url.PathEscape(commit.ID.String()),
			SHA:     commit.ID.String(),
			Created: commit.Committer.When,
		},
		HTMLURL: repo.HTMLURL() + "/commit/" + url.PathEscape(commit.ID.String()),
		RepoCommit: &api.RepoCommit{
			URL: repo.APIURL() + "/git/commits/" + url.PathEscape(commit.ID.String()),
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  commit.Author.Name,
					Email: commit.Author.Email,
				},
				Date: commit.Author.When.Format(time.RFC3339),
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  commit.Committer.Name,
					Email: commit.Committer.Email,
				},
				Date: commit.Committer.When.Format(time.RFC3339),
			},
			Message: commit.Message(),
			Tree: &api.CommitMeta{
				URL:     repo.APIURL() + "/git/trees/" + url.PathEscape(commit.ID.String()),
				SHA:     commit.ID.String(),
				Created: commit.Committer.When,
			},
		},
		Author:    apiAuthor,
		Committer: apiCommitter,
		Parents:   apiParents,
	}

	// Retrieve verification for commit
	if opts.Verification {
		res.RepoCommit.Verification = ToVerification(ctx, commit)
	}

	// Retrieve files affected by the commit
	if opts.Files {
		fileStatus, err := git.GetCommitFileStatus(gitRepo.Ctx, repo.RepoPath(), commit.ID.String())
		if err != nil {
			return nil, err
		}

		affectedFileList := make([]*api.CommitAffectedFiles, 0, len(fileStatus.Added)+len(fileStatus.Removed)+len(fileStatus.Modified))
		for _, files := range [][]string{fileStatus.Added, fileStatus.Removed, fileStatus.Modified} {
			for _, filename := range files {
				affectedFileList = append(affectedFileList, &api.CommitAffectedFiles{
					Filename: filename,
				})
			}
		}

		res.Files = affectedFileList
	}

	// Get diff stats for commit
	if opts.Stat {
		diff, err := gitdiff.GetDiff(gitRepo, &gitdiff.DiffOptions{
			AfterCommitID: commit.ID.String(),
		})
		if err != nil {
			return nil, err
		}

		res.Stats = &api.CommitStats{
			Total:     diff.TotalAddition + diff.TotalDeletion,
			Additions: diff.TotalAddition,
			Deletions: diff.TotalDeletion,
		}
	}

	return res, nil
}
