// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// ArchiverStatus represents repo archive status
type ArchiverStatus int

// enumerate all repo archive statuses
const (
	ArchiverGenerating = iota // the archiver is generating
	ArchiverReady             // it's ready
)

// RepoArchiver represents all archivers
type RepoArchiver struct { //revive:disable-line:exported
	ID          int64           `xorm:"pk autoincr"`
	RepoID      int64           `xorm:"index unique(s)"`
	Type        git.ArchiveType `xorm:"unique(s)"`
	Status      ArchiverStatus
	CommitID    string             `xorm:"VARCHAR(40) unique(s)"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL created"`
}

func init() {
	db.RegisterModel(new(RepoArchiver))
}

// RelativePath returns relative path
func (archiver *RepoArchiver) RelativePath() (string, error) {
	return fmt.Sprintf("%d/%s/%s.%s", archiver.RepoID, archiver.CommitID[:2], archiver.CommitID, archiver.Type.String()), nil
}

var delRepoArchiver = new(RepoArchiver)

// DeleteRepoArchiver delete archiver
func DeleteRepoArchiver(ctx context.Context, archiver *RepoArchiver) error {
	_, err := db.GetEngine(db.DefaultContext).ID(archiver.ID).Delete(delRepoArchiver)
	return err
}

// GetRepoArchiver get an archiver
func GetRepoArchiver(ctx context.Context, repoID int64, tp git.ArchiveType, commitID string) (*RepoArchiver, error) {
	var archiver RepoArchiver
	has, err := db.GetEngine(ctx).Where("repo_id=?", repoID).And("`type`=?", tp).And("commit_id=?", commitID).Get(&archiver)
	if err != nil {
		return nil, err
	}
	if has {
		return &archiver, nil
	}
	return nil, nil
}

// AddRepoArchiver adds an archiver
func AddRepoArchiver(ctx context.Context, archiver *RepoArchiver) error {
	_, err := db.GetEngine(ctx).Insert(archiver)
	return err
}

// UpdateRepoArchiverStatus updates archiver's status
func UpdateRepoArchiverStatus(ctx context.Context, archiver *RepoArchiver) error {
	_, err := db.GetEngine(ctx).ID(archiver.ID).Cols("status").Update(archiver)
	return err
}

// DeleteAllRepoArchives deletes all repo archives records
func DeleteAllRepoArchives() error {
	_, err := db.GetEngine(db.DefaultContext).Where("1=1").Delete(new(RepoArchiver))
	return err
}

// FindRepoArchiversOption represents an archiver options
type FindRepoArchiversOption struct {
	db.ListOptions
	OlderThan time.Duration
}

func (opts FindRepoArchiversOption) toConds() builder.Cond {
	var cond = builder.NewCond()
	if opts.OlderThan > 0 {
		cond = cond.And(builder.Lt{"created_unix": time.Now().Add(-opts.OlderThan).Unix()})
	}
	return cond
}

// FindRepoArchives find repo archivers
func FindRepoArchives(opts FindRepoArchiversOption) ([]*RepoArchiver, error) {
	var archivers = make([]*RepoArchiver, 0, opts.PageSize)
	start, limit := opts.GetSkipTake()
	err := db.GetEngine(db.DefaultContext).Where(opts.toConds()).
		Asc("created_unix").
		Limit(limit, start).
		Find(&archivers)
	return archivers, err
}

// SetArchiveRepoState sets if a repo is archived
func SetArchiveRepoState(repo *Repository, isArchived bool) (err error) {
	repo.IsArchived = isArchived
	_, err = db.GetEngine(db.DefaultContext).Where("id = ?", repo.ID).Cols("is_archived").NoAutoTime().Update(repo)
	return
}
