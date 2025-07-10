// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
)

func init() {
	db.RegisterModel(new(RepoLicense))
}

type RepoLicense struct { //revive:disable-line:exported
	ID          int64 `xorm:"pk autoincr"`
	RepoID      int64 `xorm:"UNIQUE(s) NOT NULL"`
	CommitID    string
	License     string             `xorm:"VARCHAR(255) UNIQUE(s) NOT NULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX CREATED"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX UPDATED"`
}

// RepoLicenseList defines a list of repo licenses
type RepoLicenseList []*RepoLicense //revive:disable-line:exported

func (rll RepoLicenseList) StringList() []string {
	var licenses []string
	for _, rl := range rll {
		licenses = append(licenses, rl.License)
	}
	return licenses
}

// GetRepoLicenses returns the license statistics for a repository
func GetRepoLicenses(ctx context.Context, repo *Repository) (RepoLicenseList, error) {
	licenses := make(RepoLicenseList, 0)
	if err := db.GetEngine(ctx).Where("`repo_id` = ?", repo.ID).Asc("`license`").Find(&licenses); err != nil {
		return nil, err
	}
	return licenses, nil
}

// UpdateRepoLicenses updates the license statistics for repository
func UpdateRepoLicenses(ctx context.Context, repo *Repository, commitID string, licenses []string) error {
	oldLicenses, err := GetRepoLicenses(ctx, repo)
	if err != nil {
		return err
	}
	for _, license := range licenses {
		upd := false
		for _, o := range oldLicenses {
			// Update already existing license
			if o.License == license {
				o.CommitID = commitID
				if _, err := db.GetEngine(ctx).ID(o.ID).Cols("`commit_id`").Update(o); err != nil {
					return err
				}
				upd = true
				break
			}
		}
		// Insert new license
		if !upd {
			if err := db.Insert(ctx, &RepoLicense{
				RepoID:   repo.ID,
				CommitID: commitID,
				License:  license,
			}); err != nil {
				return err
			}
		}
	}
	// Delete old licenses
	licenseToDelete := make([]int64, 0, len(oldLicenses))
	for _, o := range oldLicenses {
		if o.CommitID != commitID {
			licenseToDelete = append(licenseToDelete, o.ID)
		}
	}
	if len(licenseToDelete) > 0 {
		if _, err := db.GetEngine(ctx).In("`id`", licenseToDelete).Delete(&RepoLicense{}); err != nil {
			return err
		}
	}

	return nil
}

// CopyLicense Copy originalRepo license information to destRepo (use for forked repo)
func CopyLicense(ctx context.Context, originalRepo, destRepo *Repository) error {
	repoLicenses, err := GetRepoLicenses(ctx, originalRepo)
	if err != nil {
		return err
	}
	if len(repoLicenses) > 0 {
		newRepoLicenses := make(RepoLicenseList, 0, len(repoLicenses))

		for _, rl := range repoLicenses {
			newRepoLicense := &RepoLicense{
				RepoID:   destRepo.ID,
				CommitID: rl.CommitID,
				License:  rl.License,
			}
			newRepoLicenses = append(newRepoLicenses, newRepoLicense)
		}
		if err := db.Insert(ctx, &newRepoLicenses); err != nil {
			return err
		}
	}
	return nil
}

// CleanRepoLicenses will remove all license record of the repo
func CleanRepoLicenses(ctx context.Context, repo *Repository) error {
	return db.DeleteBeans(ctx, &RepoLicense{
		RepoID: repo.ID,
	})
}
