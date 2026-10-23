// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"errors"
	"fmt"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/timeutil"
	git_service "gitea.dev/services/git"
	notify_service "gitea.dev/services/notify"

	"xorm.io/builder"
)

// CreateRefComment creates a commit reference comment to issue.
func CreateRefComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, content, commitSHA string) error {
	if len(commitSHA) == 0 {
		return errors.New("cannot create reference with empty commit SHA")
	}

	if user_model.IsUserBlockedBy(ctx, doer, issue.PosterID, repo.OwnerID) {
		if !access_model.IsUserRepoAdmin(ctx, repo, doer) {
			return user_model.ErrBlockedUser
		}
	}

	// Check if same reference from same commit has already existed.
	has, err := db.Exist[issues_model.Comment](ctx, builder.Eq{
		"`type`":     issues_model.CommentTypeCommitRef,
		"issue_id":   issue.ID,
		"commit_sha": commitSHA,
	})
	if err != nil {
		return fmt.Errorf("check reference comment: %w", err)
	} else if has {
		return nil
	}

	_, err = issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Type:      issues_model.CommentTypeCommitRef,
		Doer:      doer,
		Repo:      repo,
		Issue:     issue,
		CommitSHA: commitSHA,
		Content:   content,
	})
	return err
}

// CreateIssueComment creates a plain issue comment.
func CreateIssueComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, issue *issues_model.Issue, content string, attachments []string) (*issues_model.Comment, error) {
	if user_model.IsUserBlockedBy(ctx, doer, issue.PosterID, repo.OwnerID) {
		if !access_model.IsUserRepoAdmin(ctx, repo, doer) {
			return nil, user_model.ErrBlockedUser
		}
	}

	comment, err := issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Type:        issues_model.CommentTypeComment,
		Doer:        doer,
		Repo:        repo,
		Issue:       issue,
		Content:     content,
		Attachments: attachments,
	})
	if err != nil {
		return nil, err
	}

	mentions, err := issues_model.FindAndUpdateIssueMentions(ctx, issue, doer, comment.Content)
	if err != nil {
		return nil, err
	}

	// reload issue to ensure it has the latest data, especially the number of comments
	issue, err = issues_model.GetIssueByID(ctx, issue.ID)
	if err != nil {
		return nil, err
	}

	notify_service.CreateIssueComment(ctx, doer, repo, issue, comment, mentions)

	return comment, nil
}

// UpdateComment updates information of comment.
func UpdateComment(ctx context.Context, c *issues_model.Comment, contentVersion int, doer *user_model.User, oldContent string) error {
	if err := c.LoadIssue(ctx); err != nil {
		return err
	}
	if err := c.Issue.LoadRepo(ctx); err != nil {
		return err
	}

	if user_model.IsUserBlockedBy(ctx, doer, c.Issue.PosterID, c.Issue.Repo.OwnerID) {
		if !access_model.IsUserRepoAdmin(ctx, c.Issue.Repo, doer) {
			return user_model.ErrBlockedUser
		}
	}

	needsContentHistory := c.Content != oldContent && c.Type.HasContentSupport()
	if needsContentHistory {
		hasContentHistory, err := issues_model.HasIssueContentHistory(ctx, c.IssueID, c.ID)
		if err != nil {
			return err
		}
		if !hasContentHistory {
			if err = issues_model.SaveIssueContentHistory(ctx, c.PosterID, c.IssueID, c.ID,
				c.CreatedUnix, oldContent, true); err != nil {
				return err
			}
		}
	}

	if err := issues_model.UpdateComment(ctx, c, contentVersion, doer); err != nil {
		return err
	}

	if needsContentHistory {
		err := issues_model.SaveIssueContentHistory(ctx, doer.ID, c.IssueID, c.ID, timeutil.TimeStampNow(), c.Content, false)
		if err != nil {
			return err
		}
	}

	notify_service.UpdateComment(ctx, doer, c, oldContent)

	return nil
}

// DeleteComment deletes the comment
func DeleteComment(ctx context.Context, doer *user_model.User, comment *issues_model.Comment) error {
	err := db.WithTx(ctx, func(ctx context.Context) error {
		return issues_model.DeleteComment(ctx, comment)
	})
	if err != nil {
		return err
	}

	notify_service.DeleteComment(ctx, doer, comment)

	return nil
}

// LoadCommentPushCommits Load push commits
func LoadCommentPushCommits(ctx context.Context, c *issues_model.Comment) error {
	return LoadCommentsPushCommits(ctx, []*issues_model.Comment{c})
}

// LoadCommentsPushCommits loads the push commits of many comments at once.
// Commit users, signatures and commit statuses are resolved in one batch per repository
// instead of once per push comment, which matters for pull requests with many pushes.
func LoadCommentsPushCommits(ctx context.Context, comments []*issues_model.Comment) error {
	groups := map[int64]*pushCommitsRepoGroup{}
	var groupOrder []int64

	for _, c := range comments {
		if c.Content == "" || c.Commits != nil || c.Type != issues_model.CommentTypePullRequestPush {
			continue
		}

		var data issues_model.PushActionContent
		if err := json.Unmarshal([]byte(c.Content), &data); err != nil {
			log.Debug("Unmarshal: %v", err) // no need to show 500 error to end user when the JSON is broken
			continue
		}

		c.IsForcePush = data.IsForcePush

		if c.IsForcePush {
			if len(data.CommitIDs) == 2 {
				c.OldCommit, c.NewCommit = data.CommitIDs[0], data.CommitIDs[1]
			}
			continue
		}

		if err := c.LoadIssue(ctx); err != nil {
			return err
		}
		if err := c.Issue.LoadRepo(ctx); err != nil {
			return err
		}
		g := groups[c.Issue.Repo.ID]
		if g == nil {
			g = &pushCommitsRepoGroup{repo: c.Issue.Repo}
			groups[c.Issue.Repo.ID] = g
			groupOrder = append(groupOrder, c.Issue.Repo.ID)
		}
		g.items = append(g.items, pendingPushComment{comment: c, ids: data.CommitIDs})
	}

	for _, repoID := range groupOrder {
		g := groups[repoID]
		if err := loadPushCommitsForRepo(ctx, g.repo, g.items); err != nil {
			return err
		}
	}
	return nil
}

const pushCommitsConvertChunkSize = 100

type pendingPushComment struct {
	comment *issues_model.Comment
	ids     []string
}

type pushCommitsRepoGroup struct {
	repo  *repo_model.Repository
	items []pendingPushComment
}

func loadPushCommitsForRepo(ctx context.Context, repo *repo_model.Repository, items []pendingPushComment) error {
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return err
	}
	defer closer.Close()

	// collect the unique commit IDs of all comments, keeping the first-seen order
	var allIDs []string
	seen := map[string]struct{}{}
	for _, it := range items {
		for _, id := range it.ids {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				allIDs = append(allIDs, id)
			}
		}
	}

	// chunked to keep the commit status query under DB bind-parameter limits
	byID := make(map[string]*git_model.SignCommitWithStatuses, len(allIDs))
	for start := 0; start < len(allIDs); start += pushCommitsConvertChunkSize {
		chunk := allIDs[start:min(start+pushCommitsConvertChunkSize, len(allIDs))]
		converted, err := git_service.ConvertFromGitCommit(ctx, gitRepo.GetCommitsFromIDs(ctx, chunk), repo, "") // no current ref sub path for PR commit list
		if err != nil {
			log.Debug("ConvertFromGitCommit: %v", err) // no need to show 500 error to end user when the commit does not exist
			continue
		}
		for _, c := range converted {
			byID[c.GitCommit.ID.String()] = c
		}
	}

	for _, it := range items {
		var commits []*git_model.SignCommitWithStatuses
		for _, id := range it.ids {
			if sc, ok := byID[id]; ok {
				commits = append(commits, sc)
			}
		}
		it.comment.Commits = commits
		it.comment.CommitsNum = int64(len(commits))
	}
	return nil
}
