// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package comments

import (
	"bytes"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/gitdiff"
)

// CreateIssueComment creates a plain issue comment.
func CreateIssueComment(doer *models.User, repo *models.Repository, issue *models.Issue, content string, attachments []string) (*models.Comment, error) {
	comment, err := models.CreateComment(&models.CreateCommentOptions{
		Type:        models.CommentTypeComment,
		Doer:        doer,
		Repo:        repo,
		Issue:       issue,
		Content:     content,
		Attachments: attachments,
	})
	if err != nil {
		return nil, err
	}

	mode, _ := models.AccessLevel(doer, repo)
	if err = models.PrepareWebhooks(repo, models.HookEventIssueComment, &api.IssueCommentPayload{
		Action:     api.HookIssueCommentCreated,
		Issue:      issue.APIFormat(),
		Comment:    comment.APIFormat(),
		Repository: repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	} else {
		go models.HookQueue.Add(repo.ID)
	}
	return comment, nil
}

// CreateCodeComment creates a plain code comment at the specified line / path
func CreateCodeComment(doer *models.User, repo *models.Repository, issue *models.Issue, content, treePath string, line, reviewID int64) (*models.Comment, error) {
	var commitID, patch string
	pr, err := models.GetPullRequestByIssueID(issue.ID)
	if err != nil {
		return nil, fmt.Errorf("GetPullRequestByIssueID: %v", err)
	}
	if err := pr.GetBaseRepo(); err != nil {
		return nil, fmt.Errorf("GetHeadRepo: %v", err)
	}
	gitRepo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
	if err != nil {
		return nil, fmt.Errorf("OpenRepository: %v", err)
	}
	defer gitRepo.Close()

	// FIXME validate treePath
	// Get latest commit referencing the commented line
	// No need for get commit for base branch changes
	if line > 0 {
		commit, err := gitRepo.LineBlame(pr.GetGitRefName(), gitRepo.Path, treePath, uint(line))
		if err == nil {
			commitID = commit.ID.String()
		} else if !strings.Contains(err.Error(), "exit status 128 - fatal: no such path") {
			return nil, fmt.Errorf("LineBlame[%s, %s, %s, %d]: %v", pr.GetGitRefName(), gitRepo.Path, treePath, line, err)
		}
	}

	// Only fetch diff if comment is review comment
	if reviewID != 0 {
		headCommitID, err := gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			return nil, fmt.Errorf("GetRefCommitID[%s]: %v", pr.GetGitRefName(), err)
		}
		patchBuf := new(bytes.Buffer)
		if err := gitdiff.GetRawDiffForFile(gitRepo.Path, pr.MergeBase, headCommitID, gitdiff.RawDiffNormal, treePath, patchBuf); err != nil {
			return nil, fmt.Errorf("GetRawDiffForLine[%s, %s, %s, %s]: %v", err, gitRepo.Path, pr.MergeBase, headCommitID, treePath)
		}
		patch = gitdiff.CutDiffAroundLine(patchBuf, int64((&models.Comment{Line: line}).UnsignedLine()), line < 0, setting.UI.CodeCommentLines)
	}
	return models.CreateComment(&models.CreateCommentOptions{
		Type:      models.CommentTypeCode,
		Doer:      doer,
		Repo:      repo,
		Issue:     issue,
		Content:   content,
		LineNum:   line,
		TreePath:  treePath,
		CommitSHA: commitID,
		ReviewID:  reviewID,
		Patch:     patch,
	})
}

// UpdateComment updates information of comment.
func UpdateComment(c *models.Comment, doer *models.User, oldContent string) error {
	if err := models.UpdateComment(c, doer); err != nil {
		return err
	}

	if err := c.LoadPoster(); err != nil {
		return err
	}
	if err := c.LoadIssue(); err != nil {
		return err
	}

	if err := c.Issue.LoadAttributes(); err != nil {
		return err
	}

	mode, _ := models.AccessLevel(doer, c.Issue.Repo)
	if err := models.PrepareWebhooks(c.Issue.Repo, models.HookEventIssueComment, &api.IssueCommentPayload{
		Action:  api.HookIssueCommentEdited,
		Issue:   c.Issue.APIFormat(),
		Comment: c.APIFormat(),
		Changes: &api.ChangesPayload{
			Body: &api.ChangesFromPayload{
				From: oldContent,
			},
		},
		Repository: c.Issue.Repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", c.ID, err)
	} else {
		go models.HookQueue.Add(c.Issue.Repo.ID)
	}

	return nil
}

// DeleteComment deletes the comment
func DeleteComment(comment *models.Comment, doer *models.User) error {
	if err := models.DeleteComment(comment, doer); err != nil {
		return err
	}

	if err := comment.LoadPoster(); err != nil {
		return err
	}
	if err := comment.LoadIssue(); err != nil {
		return err
	}

	if err := comment.Issue.LoadAttributes(); err != nil {
		return err
	}

	mode, _ := models.AccessLevel(doer, comment.Issue.Repo)

	if err := models.PrepareWebhooks(comment.Issue.Repo, models.HookEventIssueComment, &api.IssueCommentPayload{
		Action:     api.HookIssueCommentDeleted,
		Issue:      comment.Issue.APIFormat(),
		Comment:    comment.APIFormat(),
		Repository: comment.Issue.Repo.APIFormat(mode),
		Sender:     doer.APIFormat(),
	}); err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	} else {
		go models.HookQueue.Add(comment.Issue.Repo.ID)
	}

	return nil
}
