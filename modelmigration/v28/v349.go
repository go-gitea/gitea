// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"
	"time"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm/schemas"
)

type codespace struct {
	ID                        int64
	UUID                      string `xorm:"CHAR(36) NOT NULL DEFAULT '' index"`
	UserID                    int64  `xorm:"NOT NULL DEFAULT 0"`
	RepoID                    int64  `xorm:"NOT NULL DEFAULT 0"`
	RefType                   string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	RefName                   string `xorm:"TEXT NOT NULL"`
	EnvironmentTag            string `xorm:"VARCHAR(64) NOT NULL"`
	CommitSHA                 string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	DevContainerSource        string `xorm:"VARCHAR(32) NOT NULL DEFAULT ''"`
	DevContainerPath          string `xorm:"VARCHAR(512) NOT NULL DEFAULT ''"`
	DevContainerContent       string `xorm:"TEXT NOT NULL"`
	PermissionAuthorizationID int64  `xorm:"NOT NULL DEFAULT 0 index"`
	ManagerID                 int64  `xorm:"NOT NULL DEFAULT 0"`
	Status                    string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	OperationRVersion         int64  `xorm:"NOT NULL DEFAULT 0"`
	OperationType             string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	OperationStatus           string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	OperationTrigger          string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	OperationCreatedUnix      int64  `xorm:"NOT NULL DEFAULT 0"`
	OperationStartedUnix      int64  `xorm:"NOT NULL DEFAULT 0"`
	OperationDeadlineUnix     int64  `xorm:"NOT NULL DEFAULT 0"`
	RuntimeGeneration         int64  `xorm:"NOT NULL DEFAULT 0"`
	LastActiveUnix            int64  `xorm:"NOT NULL DEFAULT 0"`
	AutoStopMode              string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'default'"`
	AutoStopTimeoutSeconds    int64  `xorm:"NOT NULL DEFAULT 0"`
	InteractionGeneration     int64  `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnix               int64  `xorm:"NOT NULL DEFAULT 0"`
	UpdatedUnix               int64  `xorm:"NOT NULL DEFAULT 0"`
	LogSize                   int64  `xorm:"NOT NULL DEFAULT 0"`
}

func (*codespace) TableName() string {
	return "codespace"
}

func (*codespace) TableIndices() []*schemas.Index {
	userUpdated := schemas.NewIndex("user_updated", schemas.IndexType)
	userUpdated.AddColumn("user_id", "updated_unix", "created_unix", "id")

	repo := schemas.NewIndex("repo", schemas.IndexType)
	repo.AddColumn("repo_id")

	createClaim := schemas.NewIndex("create_claim", schemas.IndexType)
	createClaim.AddColumn("status", "operation_type", "operation_status", "manager_id", "environment_tag", "operation_created_unix", "id")

	managerActive := schemas.NewIndex("manager_active", schemas.IndexType)
	managerActive.AddColumn("manager_id", "operation_status", "operation_created_unix", "id")

	queuedTimeout := schemas.NewIndex("queued_timeout", schemas.IndexType)
	queuedTimeout.AddColumn("operation_status", "operation_created_unix", "id")

	runningTimeout := schemas.NewIndex("running_timeout", schemas.IndexType)
	runningTimeout.AddColumn("operation_status", "operation_deadline_unix", "id")

	failedRetention := schemas.NewIndex("failed_retention", schemas.IndexType)
	failedRetention.AddColumn("status", "updated_unix", "id")

	return []*schemas.Index{userUpdated, repo, createClaim, managerActive, queuedTimeout, runningTimeout, failedRetention}
}

type codespaceManager struct {
	ID                                 int64
	Name                               string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
	UserID                             int64  `xorm:"NOT NULL DEFAULT 0"`
	SecretHash                         string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	SecretSalt                         string `xorm:"VARCHAR(32) NOT NULL DEFAULT ''"`
	TagsJSON                           string `xorm:"TEXT NOT NULL"`
	RuntimeState                       string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'recovering'"`
	LastOnlineUnix                     int64  `xorm:"NOT NULL DEFAULT 0"`
	InventoryGeneration                int64  `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnix                        int64  `xorm:"NOT NULL DEFAULT 0"`
	Version                            string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	GatewaySSHHostKeyAlgorithm         string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	GatewaySSHHostKeyFingerprintSHA256 string `xorm:"gateway_ssh_host_key_fingerprint_sha256 VARCHAR(255) NOT NULL DEFAULT ''"`
	GatewaySSHHostKeyUpdatedUnix       int64  `xorm:"NOT NULL DEFAULT 0"`
}

func (*codespaceManager) TableName() string {
	return "codespace_manager"
}

func (*codespaceManager) TableIndices() []*schemas.Index {
	user := schemas.NewIndex("user", schemas.IndexType)
	user.AddColumn("user_id")
	return []*schemas.Index{user}
}

func AddCodespaceTables(_ context.Context, x base.EngineMigration) error {
	type codespaceManagerAddress struct {
		ManagerID int64  `xorm:"pk NOT NULL DEFAULT 0"`
		Kind      string `xorm:"pk VARCHAR(16) NOT NULL DEFAULT '' index(kind_address)"`
		Address   string `xorm:"VARCHAR(512) NOT NULL DEFAULT '' index(kind_address)"`
	}

	type codespaceGiteaToken struct {
		CodespaceID    int64  `xorm:"pk"`
		TokenHash      string `xorm:"VARCHAR(100) NOT NULL UNIQUE"`
		TokenSalt      string `xorm:"VARCHAR(10) NOT NULL"`
		TokenLastEight string `xorm:"VARCHAR(8) NOT NULL index"`
		TokenEncrypted string `xorm:"TEXT NOT NULL"`
	}

	type codespaceSSHKey struct {
		CodespaceID int64 `xorm:"pk"`
		KeyID       int64 `xorm:"NOT NULL UNIQUE"`
	}

	type codespacePermissionAuthorization struct {
		ID           int64
		UserID       int64  `xorm:"NOT NULL index(user_source_request)"`
		SourceRepoID int64  `xorm:"NOT NULL index(user_source_request)"`
		RequestHash  string `xorm:"CHAR(64) NOT NULL index(user_source_request)"`
		RevokedUnix  int64  `xorm:"NOT NULL DEFAULT 0"`
		CreatedUnix  int64  `xorm:"NOT NULL DEFAULT 0"`
		UpdatedUnix  int64  `xorm:"NOT NULL DEFAULT 0"`
	}

	type codespacePermissionRepository struct {
		AuthorizationID int64 `xorm:"pk NOT NULL"`
		TargetRepoID    int64 `xorm:"pk NOT NULL index"`
		UnitType        int   `xorm:"pk NOT NULL"`
		RequestedMode   int   `xorm:"NOT NULL"`
		GrantedMode     int   `xorm:"NOT NULL"`
	}

	type codespaceUserSecret struct {
		ID              int64
		UserID          int64  `xorm:"NOT NULL unique(user_name)"`
		Name            string `xorm:"VARCHAR(255) NOT NULL unique(user_name)"`
		DataEncrypted   string `xorm:"LONGTEXT NOT NULL"`
		DataSize        int64  `xorm:"NOT NULL DEFAULT 0"`
		AllRepositories bool   `xorm:"NOT NULL DEFAULT false"`
		CreatedUnix     int64  `xorm:"NOT NULL DEFAULT 0"`
		UpdatedUnix     int64  `xorm:"NOT NULL DEFAULT 0"`
	}

	type codespaceUserSecretRepository struct {
		SecretID int64 `xorm:"pk NOT NULL"`
		RepoID   int64 `xorm:"pk NOT NULL index"`
	}

	type codespaceDevContainerTemplate struct {
		ID          int64
		UserID      int64  `xorm:"NOT NULL DEFAULT 0 index"`
		Name        string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
		Content     string `xorm:"TEXT NOT NULL"`
		CreatedUnix int64  `xorm:"NOT NULL DEFAULT 0"`
		UpdatedUnix int64  `xorm:"NOT NULL DEFAULT 0"`
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := sess.Sync(
		new(codespace),
		new(codespaceManager),
		new(codespaceManagerAddress),
		new(codespaceGiteaToken),
		new(codespaceSSHKey),
		new(codespacePermissionAuthorization),
		new(codespacePermissionRepository),
		new(codespaceUserSecret),
		new(codespaceUserSecretRepository),
		new(codespaceDevContainerTemplate),
	); err != nil {
		_ = sess.Rollback()
		return err
	}
	now := time.Now().Unix()
	if _, err := sess.Insert(&codespaceDevContainerTemplate{
		UserID:      0,
		Name:        "Default",
		Content:     `{"image":"mcr.microsoft.com/devcontainers/base:ubuntu"}`,
		CreatedUnix: now,
		UpdatedUnix: now,
	}); err != nil {
		_ = sess.Rollback()
		return err
	}
	return sess.Commit()
}
