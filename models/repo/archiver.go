// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

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

// RelativePath returns the archive path relative to the archive storage root.
func (archiver *RepoArchiver) RelativePath() string {
	return fmt.Sprintf("%d/%s/%s.%s", archiver.RepoID, archiver.CommitID[:2], archiver.CommitID, archiver.Type.String())
}

// repoArchiverForRelativePath takes a relativePath created from (archiver *RepoArchiver) RelativePath() and creates a shell repoArchiver struct representing it
func repoArchiverForRelativePath(relativePath string) (*RepoArchiver, error) {
	parts := strings.SplitN(relativePath, "/", 3)
	if len(parts) != 3 {
		return nil, util.SilentWrap{Message: fmt.Sprintf("invalid storage path: %s", relativePath), Err: util.ErrInvalidArgument}
	}
	repoID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, util.SilentWrap{Message: fmt.Sprintf("invalid storage path: %s", relativePath), Err: util.ErrInvalidArgument}
	}
	nameExts := strings.SplitN(parts[2], ".", 2)
	if len(nameExts) != 2 {
		return nil, util.SilentWrap{Message: fmt.Sprintf("invalid storage path: %s", relativePath), Err: util.ErrInvalidArgument}
	}

	return &RepoArchiver{
		RepoID:   repoID,
		CommitID: parts[1] + nameExts[0],
		Type:     git.ToArchiveType(nameExts[1]),
	}, nil
}

var delRepoArchiver = new(RepoArchiver)

// DeleteRepoArchiver delete archiver
func DeleteRepoArchiver(ctx context.Context, archiver *RepoArchiver) error {
	_, err := db.GetEngine(ctx).ID(archiver.ID).Delete(delRepoArchiver)
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

// ExistsRepoArchiverWithStoragePath checks if there is a RepoArchiver for a given storage path
func ExistsRepoArchiverWithStoragePath(ctx context.Context, storagePath string) (bool, error) {
	// We need to invert the path provided func (archiver *RepoArchiver) RelativePath() above
	archiver, err := repoArchiverForRelativePath(storagePath)
	if err != nil {
		return false, err
	}

	return db.GetEngine(ctx).Exist(archiver)
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
func DeleteAllRepoArchives(ctx context.Context) error {
	_, err := db.GetEngine(ctx).Where("1=1").Delete(new(RepoArchiver))
	return err
}

// FindRepoArchiversOption represents an archiver options
type FindRepoArchiversOption struct {
	db.ListOptions
	OlderThan time.Duration
}

func (opts FindRepoArchiversOption) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OlderThan > 0 {
		cond = cond.And(builder.Lt{"created_unix": time.Now().Add(-opts.OlderThan).Unix()})
	}
	return cond
}

// FindRepoArchives find repo archivers
func FindRepoArchives(ctx context.Context, opts FindRepoArchiversOption) ([]*RepoArchiver, error) {
	archivers := make([]*RepoArchiver, 0, opts.PageSize)
	start, limit := opts.GetSkipTake()
	err := db.GetEngine(ctx).Where(opts.toConds()).
		Asc("created_unix").
		Limit(limit, start).
		Find(&archivers)
	return archivers, err
}

// SetArchiveRepoState sets if a repo is archived
func SetArchiveRepoState(ctx context.Context, repo *Repository, isArchived bool) (err error) {
	repo.IsArchived = isArchived

	if isArchived {
		repo.ArchivedUnix = timeutil.TimeStampNow()
	} else {
		repo.ArchivedUnix = timeutil.TimeStamp(0)
	}

	_, err = db.GetEngine(ctx).ID(repo.ID).Cols("is_archived", "archived_unix").NoAutoTime().Update(repo)
	return err
}
