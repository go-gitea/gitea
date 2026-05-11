// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// AppCredential represents a GitHub App credential for migrations
type AppCredential struct {
	ID                  int64              `xorm:"pk autoincr"`
	OwnerID             int64              `xorm:"INDEX NOT NULL"`
	Name                string             `xorm:"NOT NULL"`
	ClientID            string             `xorm:"NOT NULL"`
	InstallationID      int64              `xorm:"NOT NULL"`
	PrivateKeyEncrypted string             `xorm:"TEXT NOT NULL"`
	BaseURL             string             `xorm:"VARCHAR(255) NOT NULL DEFAULT 'https://api.github.com'"`
	CreatedUnix         timeutil.TimeStamp `xorm:"created"`
	LastUsedUnix        timeutil.TimeStamp `xorm:"last_used_unix"`
}

// HasRecentActivity returns true if this credential was used recently
func (g *AppCredential) HasRecentActivity() bool {
	// Consider activity within the last 7 days as recent
	return g.LastUsedUnix > 0 && timeutil.TimeStampNow()-g.LastUsedUnix < 7*24*3600
}

// HasUsed returns true if this credential has been used (token exchange occurred)
func (g *AppCredential) HasUsed() bool {
	return g.LastUsedUnix > 0
}

func init() {
	db.RegisterModel(new(AppCredential))
}

// TableName returns the table name for GithubAppCredential
func (g *AppCredential) TableName() string {
	return "github_app_credential"
}

// CreateGithubAppCredential creates a new GitHub App credential
func CreateGithubAppCredential(ctx context.Context, cred *AppCredential) error {
	_, err := db.GetEngine(ctx).Insert(cred)
	return err
}

// GetGithubAppCredentialByID gets a GitHub App credential by ID
func GetGithubAppCredentialByID(ctx context.Context, id int64) (*AppCredential, error) {
	cred := &AppCredential{}
	has, err := db.GetEngine(ctx).ID(id).Get(cred)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, util.ErrNotExist
	}
	return cred, nil
}

// UpdateGithubAppCredentialLastUsed updates the timestamp of when it was last used on successful exchange
func UpdateGithubAppCredentialLastUsed(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Cols("last_used_unix").Update(&AppCredential{
		LastUsedUnix: timeutil.TimeStampNow(),
	})
	return err
}

// GetGithubAppCredentialsByOwnerID gets all GitHub App credentials for an owner
func GetGithubAppCredentialsByOwnerID(ctx context.Context, ownerID int64) ([]*AppCredential, error) {
	creds := make([]*AppCredential, 0, 5)
	return creds, db.GetEngine(ctx).
		Where("owner_id = ?", ownerID).
		Find(&creds)
}

// CountMirrorsByCredentialID returns the number of mirrors using the given credential
func CountMirrorsByCredentialID(ctx context.Context, credID int64) (int64, error) {
	type Mirror struct {
		GithubAppCredentialID int64 `xorm:"github_app_credential_id"`
	}
	return db.GetEngine(ctx).
		Table("mirror").
		Where("github_app_credential_id = ?", credID).
		Count(new(Mirror))
}

// DeleteGithubAppCredential deletes a GitHub App credential.
// It returns an error if any mirrors still reference the credential.
func DeleteGithubAppCredential(ctx context.Context, id int64) error {
	// Prevent deletion while mirrors still reference this credential
	count, err := CountMirrorsByCredentialID(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("cannot delete credential: %d mirror(s) still reference it", count)
	}

	_, err = db.GetEngine(ctx).ID(id).Delete(&AppCredential{})
	return err
}

// CheckGithubAppCredentialOwnership checks if a user owns a GitHub App credential
func CheckGithubAppCredentialOwnership(ctx context.Context, credID, ownerID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where(builder.Eq{"id": credID, "owner_id": ownerID}).
		Exist(&AppCredential{})
}

// MirrorWithRepo holds mirror information along with the associated repository details
type MirrorWithRepo struct {
	MirrorID  int64  `xorm:"mirror_id"`
	RepoID    int64  `xorm:"repo_id"`
	RepoName  string `xorm:"repo_name"`
	OwnerName string `xorm:"owner_name"`
	OwnerID   int64  `xorm:"owner_id"`
}

// GetMirrorsWithRepoByCredentialID returns all mirrors using a given credential ID,
// along with repository and owner details for display purposes
func GetMirrorsWithRepoByCredentialID(ctx context.Context, credID int64) ([]*MirrorWithRepo, error) {
	mirrors := make([]*MirrorWithRepo, 0, 5)
	err := db.GetEngine(ctx).
		Table("mirror").
		Select("mirror.id AS mirror_id, mirror.repo_id, repository.name AS repo_name, `user`.name AS owner_name, repository.owner_id").
		Join("INNER", "repository", "repository.id = mirror.repo_id").
		Join("INNER", "`user`", "`user`.id = repository.owner_id").
		Where("mirror.github_app_credential_id = ?", credID).
		Find(&mirrors)
	return mirrors, err
}
