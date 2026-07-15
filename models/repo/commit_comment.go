// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"html/template"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
)

// CommitComment represents a comment on a line of code in a commit diff.
// This is a standalone model unrelated to the issue/PR Comment system.
type CommitComment struct {
	ID        int64            `xorm:"pk autoincr"`
	RepoID    int64            `xorm:"INDEX NOT NULL DEFAULT 0"`
	CommitSHA string           `xorm:"VARCHAR(64) INDEX NOT NULL DEFAULT ''"`
	TreePath  string           `xorm:"VARCHAR(4000) NOT NULL DEFAULT ''"`
	Line      int64            `xorm:"NOT NULL DEFAULT 0"` // + is right, - is left
	Content   string           `xorm:"LONGTEXT NOT NULL"`
	Patch     string           `xorm:"LONGTEXT"`
	PosterID  int64            `xorm:"INDEX NOT NULL DEFAULT 0"`
	Poster    *user_model.User `xorm:"-"`

	CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix     timeutil.TimeStamp `xorm:"INDEX updated"`
	RenderedContent template.HTML      `xorm:"-"`
}

func init() {
	db.RegisterModel(new(CommitComment))
}

// DiffSide returns "previous" if Line is a LOC of the previous contents and "proposed" if it is a LOC of the proposed contents.
func (c *CommitComment) DiffSide() string {
	if c.Line < 0 {
		return "previous"
	}
	return "proposed"
}

// UnsignedLine returns the LOC of the code comment without + or -
func (c *CommitComment) UnsignedLine() uint64 {
	if c.Line < 0 {
		return uint64(c.Line * -1)
	}
	return uint64(c.Line)
}

// HashTag returns unique hash tag for the commit comment.
func (c *CommitComment) HashTag() string {
	return fmt.Sprintf("commitcomment-%d", c.ID)
}

// CommitCommentList provides helpers for CommitComment slices
type CommitCommentList []*CommitComment

func (l CommitCommentList) LoadPosters(ctx context.Context) error {
	if len(l) == 0 {
		return nil
	}
	posterIDs := container.FilterSlice(l, func(c *CommitComment) (int64, bool) {
		return c.PosterID, c.Poster == nil && c.PosterID > 0
	})
	posterMaps, err := user_model.GetUsersMapByIDs(ctx, posterIDs)
	if err != nil {
		return err
	}
	for _, comment := range l {
		if comment.Poster == nil {
			comment.Poster = user_model.GetPossibleUserFromMap(comment.PosterID, posterMaps)
		}
	}
	return nil
}

var ErrInvalidCommitCommentLine = util.ErrInvalidArgument

func CreateCommitComment(ctx context.Context, doerID int64, repoID int64, commitSHA string, treePath string, line int64, content string, patch string) (*CommitComment, error) {
	if line == 0 {
		return nil, ErrInvalidCommitCommentLine
	}
	comment := &CommitComment{
		RepoID:    repoID,
		CommitSHA: commitSHA,
		TreePath:  treePath,
		Line:      line,
		Patch:     patch,
		Content:   content,
		PosterID:  doerID,
	}
	if err := db.Insert(ctx, comment); err != nil {
		return nil, fmt.Errorf("CreateCommitComment: %w", err)
	}
	return comment, nil
}

func DeleteCommitComment(ctx context.Context, id int64, repoID int64) error {
	affected, err := db.GetEngine(ctx).Where("id = ? AND repo_id = ?", id, repoID).Delete(new(CommitComment))
	if err != nil {
		return fmt.Errorf("DeleteCommitComment: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("commit comment %d not found in repo %d", id, repoID)
	}
	return nil
}

func FindCommitCommentByID(ctx context.Context, id int64) (*CommitComment, error) {
	comment := new(CommitComment)
	has, err := db.GetEngine(ctx).ID(id).Get(comment)
	if err != nil {
		return nil, fmt.Errorf("FindCommitCommentByID: %w", err)
	}
	if !has {
		return nil, fmt.Errorf("commit comment %d not found", id)
	}
	return comment, nil
}

// FindCommitCommentsByCommitSHA returns all commit comments for a given commit.
func FindCommitCommentsByCommitSHA(ctx context.Context, repoID int64, commitSHA string) (CommitCommentList, error) {
	comments := make([]*CommitComment, 0, 10)
	err := db.GetEngine(ctx).Where("repo_id = ? AND commit_sha = ?", repoID, commitSHA).
		Find(&comments)
	if err != nil {
		return nil, fmt.Errorf("FindCommitCommentsByCommitSHA: %w", err)
	}
	return comments, nil
}

// FindCommitCommentsForDiff returns commit comments organized by file -> line -> comments.
func FindCommitCommentsForDiff(ctx context.Context, repoID int64, commitSHA string) (map[string]map[int64][]*CommitComment, error) {
	allComments, err := FindCommitCommentsByCommitSHA(ctx, repoID, commitSHA)
	if err != nil {
		return nil, err
	}
	err = CommitCommentList(allComments).LoadPosters(ctx)
	if err != nil {
		return nil, err
	}
	fileMap := make(map[string]map[int64][]*CommitComment)
	for _, c := range allComments {
		if fileMap[c.TreePath] == nil {
			fileMap[c.TreePath] = make(map[int64][]*CommitComment)
		}
		var lineKey int64
		if c.Line < 0 {
			lineKey = int64(c.UnsignedLine()) * -1
		} else {
			lineKey = int64(c.UnsignedLine())
		}
		fileMap[c.TreePath][lineKey] = append(fileMap[c.TreePath][lineKey], c)
	}
	return fileMap, nil
}
