// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_28

import (
	"gitea.dev/modelmigration/base"

	"xorm.io/xorm/schemas"
)

type codespace struct {
	UUID                      string `xorm:"pk CHAR(36)"`
	UserID                    int64  `xorm:"NOT NULL DEFAULT 0"`
	RepoID                    int64  `xorm:"NOT NULL DEFAULT 0"`
	RefType                   string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	RefName                   string `xorm:"TEXT NOT NULL"`
	EnvironmentTag            string `xorm:"VARCHAR(64) NOT NULL"`
	CommitSHA                 string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	DevContainerPath          string `xorm:"VARCHAR(512) NOT NULL DEFAULT ''"`
	DevContainerContentSHA256 string `xorm:"dev_container_content_sha256 VARCHAR(64) NOT NULL DEFAULT ''"`
	DevContainerDefaultImage  string `xorm:"VARCHAR(512) NOT NULL DEFAULT ''"`
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
	userUpdated.AddColumn("user_id", "updated_unix", "created_unix")

	repo := schemas.NewIndex("repo", schemas.IndexType)
	repo.AddColumn("repo_id")

	createClaim := schemas.NewIndex("create_claim", schemas.IndexType)
	createClaim.AddColumn("status", "operation_type", "operation_status", "manager_id", "environment_tag", "operation_created_unix", "uuid")

	managerActive := schemas.NewIndex("manager_active", schemas.IndexType)
	managerActive.AddColumn("manager_id", "operation_status", "operation_type", "status", "operation_created_unix", "uuid")

	queuedTimeout := schemas.NewIndex("queued_timeout", schemas.IndexType)
	queuedTimeout.AddColumn("operation_status", "operation_created_unix", "uuid")

	runningTimeout := schemas.NewIndex("running_timeout", schemas.IndexType)
	runningTimeout.AddColumn("operation_status", "operation_deadline_unix", "uuid")

	failedRetention := schemas.NewIndex("failed_retention", schemas.IndexType)
	failedRetention.AddColumn("status", "updated_unix", "uuid")

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

func AddCodespaceTables(x base.EngineMigration) error {
	type codespaceManagerAddress struct {
		ManagerID int64  `xorm:"pk NOT NULL DEFAULT 0"`
		Kind      string `xorm:"pk VARCHAR(16) NOT NULL DEFAULT '' unique(kind_address)"`
		Address   string `xorm:"VARCHAR(512) NOT NULL DEFAULT '' unique(kind_address)"`
	}

	type codespaceManagerToken struct {
		Token  string `xorm:"VARCHAR(64) NOT NULL UNIQUE"`
		UserID int64  `xorm:"pk NOT NULL DEFAULT 0"`
	}

	type codespaceGiteaToken struct {
		CodespaceUUID  string `xorm:"pk CHAR(36)"`
		TokenHash      string `xorm:"VARCHAR(100) NOT NULL UNIQUE"`
		TokenSalt      string `xorm:"VARCHAR(10) NOT NULL"`
		TokenLastEight string `xorm:"VARCHAR(8) NOT NULL index"`
		TokenEncrypted string `xorm:"TEXT NOT NULL"`
	}

	type codespaceSSHKey struct {
		CodespaceUUID string `xorm:"pk CHAR(36)"`
		KeyID         int64  `xorm:"NOT NULL UNIQUE"`
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

	return x.Sync(
		new(codespace),
		new(codespaceManager),
		new(codespaceManagerAddress),
		new(codespaceManagerToken),
		new(codespaceGiteaToken),
		new(codespaceSSHKey),
		new(codespacePermissionAuthorization),
		new(codespacePermissionRepository),
		new(codespaceUserSecret),
		new(codespaceUserSecretRepository),
	)
}
