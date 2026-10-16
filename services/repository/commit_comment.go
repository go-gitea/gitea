// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	activities_model "gitea.dev/models/activities"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	"gitea.dev/modules/references"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
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
	if user_model.IsUserBlockedBy(ctx, doer, repo.OwnerID) {
		return nil, user_model.ErrBlockedUser
	}

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
	status, oldName, err := commitDiffFile(ctx, gitRepo, parentSHA, fullSHA, treePath)
	if err != nil {
		return err
	} else if status == "" {
		return ErrCommitCommentCoordinates
	}

	// --skip-to matches the raw path, unlike a pathspec, and keeps rename
	// detection. No line limit: the page loads a file in full on request.
	diff, err := gitdiff.GetDiffForAPI(ctx, gitRepo, &gitdiff.DiffOptions{
		DiffCommonOptions: gitdiff.DiffCommonOptions{BeforeCommitID: parentSHA, AfterCommitID: fullSHA},
		SkipTo:            treePath,
		MaxLines:          -1,
		MaxLineCharacters: setting.Git.MaxGitDiffLineCharacters,
		// only a type change is split into two patches; the next file may be huge
		MaxFiles: util.Iif(status == "T", 2, 1),
	})
	if err != nil {
		return err
	}
	// A type change (file to symlink) is a deletion plus an addition of the same
	// path, and only one of them holds the side being commented on.
	fileIdx := slices.IndexFunc(diff.Files, func(f *gitdiff.DiffFile) bool {
		return f.Name == treePath && f.Type != util.Iif(line < 0, gitdiff.DiffFileAdd, gitdiff.DiffFileDel)
	})
	if fileIdx < 0 && !slices.ContainsFunc(diff.Files, func(f *gitdiff.DiffFile) bool { return f.Name == treePath }) {
		// the tree diff has the file, so git failed rather than the request
		return fmt.Errorf("no diff for %s in commit %s", treePath, fullSHA)
	}
	// a binary file or a rename without changes renders no rows to comment on
	if fileIdx < 0 || diff.Files[fileIdx].IsBin || len(diff.Files[fileIdx].Sections) == 0 {
		return ErrCommitCommentCoordinates
	}

	sideSHA, sidePath := fullSHA, treePath
	if line < 0 {
		// the diff posts a renamed file's new name for both sides
		sideSHA, sidePath = parentSHA, oldName
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

// commitDiffFile looks treePath up among the files the commit's diff shows and
// returns its status letters (empty when absent) and its name in the parent,
// which differs only for a rename. -z keeps git from quoting unusual names.
func commitDiffFile(ctx context.Context, gitRepo *git.Repository, parentSHA, fullSHA, treePath string) (status, oldName string, _ error) {
	cmd := gitcmd.NewCommand("diff-tree", "-r", "-z", "--name-status", "--no-commit-id", "--root").
		AddOptionFormat("--find-renames=%s", setting.Git.DiffRenameSimilarityThreshold)
	if parentSHA != "" {
		cmd.AddDynamicArguments(parentSHA)
	}
	stdout, _, err := cmd.AddDynamicArguments(fullSHA).WithRepo(gitRepo).RunStdString(ctx)
	if err != nil {
		return "", "", err
	}
	fields := strings.Split(stdout, "\x00")
	for i := 0; i+1 < len(fields); i += 2 {
		status, oldName, newName := fields[i], fields[i+1], fields[i+1]
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			i++
			if i+1 >= len(fields) {
				break
			}
			newName = fields[i+1]
		}
		if newName == treePath {
			return status, oldName, nil
		}
	}
	return "", "", nil
}
