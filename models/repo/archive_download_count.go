// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
)

// RepoArchiveDownloadCount counts all archive downloads for a tag
type RepoArchiveDownloadCount struct {
	ID     int64           `xorm:"pk autoincr"`
	RepoID int64           `xorm:"index unique(s)"`
	Type   git.ArchiveType `xorm:"unique(s)"`
	Tag    string          `xorm:"index unique(s)"`
	Count  int64
}

func init() {
	db.RegisterModel(new(RepoArchiveDownloadCount))
}

// CountArchiveDownload adds one download the the given archive
func CountArchiveDownload(ctx context.Context, repoID int64, tp git.ArchiveType, tag string) error {
	var counter RepoArchiveDownloadCount
	has, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).And("`type` = ?", tp).And("tag = ?", tag).Get(&counter)
	if err != nil {
		return err
	}

	if has {
		// The archive already exists in the database, so let's add to the the counter
		counter.Count += 1
		_, err = db.GetEngine(ctx).ID(counter.ID).Update(counter)
		return err
	}

	// The archive does not esxists in the databse, so let's add it
	newCounter := &RepoArchiveDownloadCount{
		RepoID: repoID,
		Type:   tp,
		Tag:    tag,
		Count:  1,
	}

	_, err = db.GetEngine(ctx).Insert(newCounter)
	return err
}

// GetTagDownloadCount returns the download count of a tag
func GetTagDownloadCount(ctx context.Context, repoID int64, tag string) (*api.TagArchiveDownloadCount, error) {
	tagCounter := new(api.TagArchiveDownloadCount)

	var zipCounter RepoArchiveDownloadCount
	has, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).And("`type` = ?", git.ZIP).And("tag = ?", tag).Get(&zipCounter)
	if err != nil {
		return nil, err
	}
	if has {
		tagCounter.Zip = zipCounter.Count
	}

	var targzCounter RepoArchiveDownloadCount
	has, err = db.GetEngine(ctx).Where("repo_id = ?", repoID).And("`type` = ?", git.TARGZ).And("tag = ?", tag).Get(&targzCounter)
	if err != nil {
		return nil, err
	}
	if has {
		tagCounter.TarGz = targzCounter.Count
	}

	return tagCounter, nil
}
