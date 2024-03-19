// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
)

// RepoArchiveDownloadCount counts all archive downloads for a tag
type RepoArchiveDownloadCount struct { //nolint:revive
	ID        int64           `xorm:"pk autoincr"`
	RepoID    int64           `xorm:"index unique(s)"`
	ReleaseID int64           `xorm:"index unique(s)"`
	Type      git.ArchiveType `xorm:"unique(s)"`
	Count     int64
}

func init() {
	db.RegisterModel(new(RepoArchiveDownloadCount))
}

// CountArchiveDownload adds one download the the given archive
func CountArchiveDownload(ctx context.Context, repoID, releaseID int64, tp git.ArchiveType) error {
	updateCount, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).And("release_id = ?", releaseID).And("`type` = ?", tp).Incr("count").Update(new(RepoArchiveDownloadCount))
	if err != nil {
		return err
	}

	if updateCount != 0 {
		// The count was updated, so we can exit
		return nil
	}

	// The archive does not esxists in the databse, so let's add it
	newCounter := &RepoArchiveDownloadCount{
		RepoID:    repoID,
		ReleaseID: releaseID,
		Type:      tp,
		Count:     1,
	}

	_, err = db.GetEngine(ctx).Insert(newCounter)
	return err
}

// GetArchiveDownloadCount returns the download count of a tag
func GetArchiveDownloadCount(ctx context.Context, repoID, releaseID int64) (*api.TagArchiveDownloadCount, error) {
	downloadCountList := make([]RepoArchiveDownloadCount, 0)
	err := db.GetEngine(ctx).Where("repo_id = ?", repoID).And("release_id = ?", releaseID).Find(&downloadCountList)
	if err != nil {
		return nil, err
	}

	tagCounter := new(api.TagArchiveDownloadCount)

	for _, singleCount := range downloadCountList {
		switch singleCount.Type {
		case git.ZIP:
			tagCounter.Zip = singleCount.Count
		case git.TARGZ:
			tagCounter.TarGz = singleCount.Count
		}
	}

	return tagCounter, nil
}

// GetDownloadCountForTagName returns the download count of a tag with the given name
func GetArchiveDownloadCountForTagName(ctx context.Context, repoID int64, tagName string) (*api.TagArchiveDownloadCount, error) {
	release, err := GetRelease(ctx, repoID, tagName)
	if err != nil {
		return nil, err
	}

	return GetArchiveDownloadCount(ctx, repoID, release.ID)
}

// DeleteArchiveDownloadCountForRelease deletes the release from the repo_archive_download_count table
func DeleteArchiveDownloadCountForRelease(ctx context.Context, releaseID int64) error {
	_, err := db.GetEngine(ctx).Delete(&RepoArchiveDownloadCount{ReleaseID: releaseID})
	return err
}
