// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import "gitea.dev/models/db"

// UserSecret stores an encrypted Codespace environment variable owned by one user.
type UserSecret struct {
	ID              int64
	UserID          int64  `xorm:"NOT NULL unique(user_name)"`
	Name            string `xorm:"VARCHAR(255) NOT NULL unique(user_name)"`
	DataEncrypted   string `xorm:"LONGTEXT NOT NULL"`
	DataSize        int64  `xorm:"NOT NULL DEFAULT 0"`
	AllRepositories bool   `xorm:"NOT NULL DEFAULT false"`
	CreatedUnix     int64  `xorm:"NOT NULL DEFAULT 0"`
	UpdatedUnix     int64  `xorm:"NOT NULL DEFAULT 0"`
}

// UserSecretRepository grants one repository access to a user-owned Codespace secret.
type UserSecretRepository struct {
	ID       int64
	SecretID int64 `xorm:"NOT NULL index unique(secret_repo)"`
	RepoID   int64 `xorm:"NOT NULL index unique(secret_repo)"`
}

func (*UserSecret) TableName() string {
	return "codespace_user_secret"
}

func (*UserSecretRepository) TableName() string {
	return "codespace_user_secret_repository"
}

func init() {
	db.RegisterModel(new(UserSecret))
	db.RegisterModel(new(UserSecretRepository))
}
