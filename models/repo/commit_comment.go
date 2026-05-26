// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"errors"
	"fmt"
	"html/template"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/timeutil"
)

var ErrInvalidCommitCommentLine = errors.New("commit comment line must be non-zero")

type CommitComment struct {
	ID          int64              `xorm:"pk autoincr"`
	RepoID      int64              `xorm:"INDEX NOT NULL"`
	CommitSHA   string             `xorm:"VARCHAR(64) INDEX NOT NULL"`
	TreePath    string             `xorm:"VARCHAR(4000) NOT NULL"`
	Line        int64              `xorm:"NOT NULL"`
	PosterID    int64              `xorm:"INDEX NOT NULL"`
	Content     string             `xorm:"LONGTEXT NOT NULL"`
	Patch       string             `xorm:"LONGTEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	Poster          *user_model.User `xorm:"-"`
	RenderedContent template.HTML    `xorm:"-"`
}

func init() {
	db.RegisterModel(new(CommitComment))
}

func (c *CommitComment) HashTag() string {
	return fmt.Sprintf("commitcomment-%d", c.ID)
}

func (c *CommitComment) DiffSide() string {
	if c.Line < 0 {
		return "previous"
	}
	return "proposed"
}

func (c *CommitComment) UnsignedLine() uint64 {
	if c.Line < 0 {
		return uint64(-c.Line)
	}
	return uint64(c.Line)
}

func (c *CommitComment) LoadPoster(ctx context.Context) error {
	if c.Poster != nil {
		return nil
	}
	var err error
	c.Poster, err = user_model.GetUserByID(ctx, c.PosterID)
	return err
}

type CommitCommentList []*CommitComment

func (cl CommitCommentList) LoadPosters(ctx context.Context) error {
	if len(cl) == 0 {
		return nil
	}
	posterIDs := make([]int64, 0, len(cl))
	seen := make(map[int64]struct{})
	for _, c := range cl {
		if _, ok := seen[c.PosterID]; ok {
			continue
		}
		seen[c.PosterID] = struct{}{}
		posterIDs = append(posterIDs, c.PosterID)
	}
	posterMap, err := user_model.GetUsersMapByIDs(ctx, posterIDs)
	if err != nil {
		return err
	}
	for _, c := range cl {
		if p, ok := posterMap[c.PosterID]; ok {
			c.Poster = p
		} else {
			c.Poster = user_model.NewGhostUser()
		}
	}
	return nil
}

type FileCommitComments struct {
	Left  map[int][]*CommitComment
	Right map[int][]*CommitComment
}

type CommitCommentsForDiff map[string]*FileCommitComments

func FindCommitCommentsByCommitSHA(ctx context.Context, repoID int64, commitSHA string) (CommitCommentList, error) {
	comments := make(CommitCommentList, 0)
	if err := db.GetEngine(ctx).
		Where("repo_id = ? AND commit_sha = ?", repoID, commitSHA).
		OrderBy("created_unix ASC").
		Find(&comments); err != nil {
		return nil, err
	}
	if err := comments.LoadPosters(ctx); err != nil {
		return nil, err
	}
	return comments, nil
}

func FindCommitCommentsForDiff(ctx context.Context, repoID int64, commitSHA string) (CommitCommentsForDiff, error) {
	comments, err := FindCommitCommentsByCommitSHA(ctx, repoID, commitSHA)
	if err != nil {
		return nil, err
	}
	result := make(CommitCommentsForDiff)
	for _, c := range comments {
		fcc, ok := result[c.TreePath]
		if !ok {
			fcc = &FileCommitComments{
				Left:  make(map[int][]*CommitComment),
				Right: make(map[int][]*CommitComment),
			}
			result[c.TreePath] = fcc
		}
		if c.Line < 0 {
			idx := int(-c.Line)
			fcc.Left[idx] = append(fcc.Left[idx], c)
		} else {
			idx := int(c.Line)
			fcc.Right[idx] = append(fcc.Right[idx], c)
		}
	}
	return result, nil
}

func CreateCommitComment(ctx context.Context, c *CommitComment) error {
	if c.Line == 0 {
		return ErrInvalidCommitCommentLine
	}
	_, err := db.GetEngine(ctx).Insert(c)
	return err
}

func DeleteCommitComment(ctx context.Context, repoID, id int64) error {
	_, err := db.GetEngine(ctx).
		Where("repo_id = ? AND id = ?", repoID, id).
		Delete(&CommitComment{})
	return err
}

func GetCommitCommentByID(ctx context.Context, repoID, id int64) (*CommitComment, error) {
	c := &CommitComment{}
	has, err := db.GetEngine(ctx).
		Where("repo_id = ? AND id = ?", repoID, id).
		Get(c)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, db.ErrNotExist{Resource: "CommitComment", ID: id}
	}
	return c, nil
}
