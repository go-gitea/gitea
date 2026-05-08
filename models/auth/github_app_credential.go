// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// GithubAppCredential represents a GitHub App credential for migrations
type GithubAppCredential struct {
	ID                  int64              `xorm:"pk autoincr"`
	OwnerID             int64              `xorm:"INDEX NOT NULL"`
	Name                string             `xorm:"NOT NULL"`
	AppID               int64              `xorm:"NOT NULL"`
	InstallationID      int64              `xorm:"NOT NULL"`
	PrivateKeyEncrypted string             `xorm:"TEXT NOT NULL"`
	BaseURL             string             `xorm:"VARCHAR(255) NOT NULL DEFAULT 'https://api.github.com'"`
	CreatedUnix         timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix         timeutil.TimeStamp `xorm:"updated"`
}

// HasRecentActivity returns true if this credential was used recently
func (g *GithubAppCredential) HasRecentActivity() bool {
	// Consider activity within the last 7 days as recent
	return timeutil.TimeStampNow()-g.UpdatedUnix < 7*24*3600
}

// HasUsed returns true if this credential has been used (updated after creation)
func (g *GithubAppCredential) HasUsed() bool {
	return g.UpdatedUnix > g.CreatedUnix
}

func init() {
	db.RegisterModel(new(GithubAppCredential))
}

// TableName returns the table name for GithubAppCredential
func (g *GithubAppCredential) TableName() string {
	return "github_app_credential"
}

// CreateGithubAppCredential creates a new GitHub App credential
func CreateGithubAppCredential(ctx context.Context, cred *GithubAppCredential) error {
	_, err := db.GetEngine(ctx).Insert(cred)
	return err
}

// GetGithubAppCredentialByID gets a GitHub App credential by ID
func GetGithubAppCredentialByID(ctx context.Context, id int64) (*GithubAppCredential, error) {
	cred := &GithubAppCredential{}
	has, err := db.GetEngine(ctx).ID(id).Get(cred)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, util.ErrNotExist
	}
	return cred, nil
}

// GetGithubAppCredentialsByOwnerID gets all GitHub App credentials for an owner
func GetGithubAppCredentialsByOwnerID(ctx context.Context, ownerID int64) ([]*GithubAppCredential, error) {
	creds := make([]*GithubAppCredential, 0, 5)
	return creds, db.GetEngine(ctx).
		Where("owner_id = ?", ownerID).
		Find(&creds)
}

// UpdateGithubAppCredential updates a GitHub App credential
func UpdateGithubAppCredential(ctx context.Context, cred *GithubAppCredential) error {
	_, err := db.GetEngine(ctx).ID(cred.ID).AllCols().Update(cred)
	return err
}

// DeleteGithubAppCredential deletes a GitHub App credential
func DeleteGithubAppCredential(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Delete(&GithubAppCredential{})
	return err
}

// CheckGithubAppCredentialOwnership checks if a user owns a GitHub App credential
func CheckGithubAppCredentialOwnership(ctx context.Context, credID, ownerID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where(builder.Eq{"id": credID, "owner_id": ownerID}).
		Exist(&GithubAppCredential{})
}
