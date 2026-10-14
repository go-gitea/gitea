// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"

	activities_model "gitea.dev/models/activities"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/references"
	"gitea.dev/modules/setting"
	"gitea.dev/services/gitdiff"
)

// ErrCommitCommentCoordinates is returned when the requested diff coordinate
// does not resolve to a real line of the commit.
var ErrCommitCommentCoordinates = errors.New("comment coordinates do not resolve to a line in this commit")

// ErrCommitCommentRootPrevious is returned when commenting on the old side of a
// commit that has no parent, and therefore no old side.
var ErrCommitCommentRootPrevious = errors.New("cannot comment on the previous side of a root commit")

// CreateCommitComment stores an inline comment on a commit diff. line is signed:
// negative for the old side, positive for the new one.
func CreateCommitComment(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, gitRepo *git.Repository, commitSHA, treePath string, line int64, content string) (*repo_model.CommitComment, error) {
	commit, err := gitRepo.GetCommit(ctx, commitSHA)
	if err != nil {
		return nil, err
	}
	fullSHA := commit.ID.String()

	var parentSHA string
	if commit.ParentCount() > 0 {
		parentID, err := commit.ParentID(0)
		if err == nil {
			parentSHA = parentID.String()
		}
	}
	if parentSHA == "" && line < 0 {
		return nil, ErrCommitCommentRootPrevious
	}

	if err := checkCommitCommentLine(ctx, gitRepo, parentSHA, fullSHA, treePath, line); err != nil {
		return nil, err
	}

	comment := &repo_model.CommitComment{
		RepoID:    repo.ID,
		CommitSHA: fullSHA,
		TreePath:  treePath,
		Line:      line,
		PosterID:  doer.ID,
		Poster:    doer,
		Content:   content,
	}
	if err := repo_model.CreateCommitComment(ctx, comment); err != nil {
		return nil, err
	}

	mentions := references.FindAllMentionsMarkdown(content)
	if err := activities_model.CreateCommitCommentNotification(ctx, doer, repo, fullSHA, comment.ID, commit.Author.Email, mentions); err != nil {
		log.Error("CreateCommitCommentNotification: %v", err)
	}
	return comment, nil
}

// checkCommitCommentLine makes sure the comment has a row on the commit page:
// the commit's diff shows the file as text, and the line exists on its side (the
// parent's copy of the file for the old side, the commit's for the new one). A
// crafted POST would otherwise store a comment that no diff row ever shows.
func checkCommitCommentLine(ctx context.Context, gitRepo *git.Repository, parentSHA, fullSHA, treePath string, line int64) error {
	// --skip-to keeps rename detection, which limiting the diff to treePath loses
	diff, err := gitdiff.GetDiffForAPI(ctx, gitRepo, &gitdiff.DiffOptions{
		DiffCommonOptions: gitdiff.DiffCommonOptions{BeforeCommitID: parentSHA, AfterCommitID: fullSHA},
		SkipTo:            treePath,
		MaxLines:          setting.Git.MaxGitDiffLines,
		MaxLineCharacters: setting.Git.MaxGitDiffLineCharacters,
		MaxFiles:          1,
	})
	if err != nil {
		return err
	}
	// a binary file or a rename without changes renders no rows to comment on
	if len(diff.Files) == 0 || diff.Files[0].Name != treePath || diff.Files[0].IsBin || len(diff.Files[0].Sections) == 0 {
		return ErrCommitCommentCoordinates
	}

	sideSHA, sidePath := fullSHA, treePath
	if line < 0 {
		// the diff posts a renamed file's new name for both sides
		sideSHA, sidePath = parentSHA, diff.Files[0].OldName
	}
	commit, err := gitRepo.GetCommit(ctx, sideSHA)
	if err != nil {
		return err
	}
	entry, err := commit.GetTreeEntryByPath(ctx, gitRepo, sidePath)
	if git.IsErrNotExist(err) {
		return ErrCommitCommentCoordinates
	} else if err != nil {
		return err
	}
	if entry.IsDir() || entry.IsSubModule() {
		return ErrCommitCommentCoordinates
	}
	_, lineCount, err := entry.Blob(gitRepo).GetBlobLineCount(ctx, nil)
	if err != nil {
		return err
	}
	if max(line, -line) > int64(lineCount) {
		return ErrCommitCommentCoordinates
	}
	return nil
}
