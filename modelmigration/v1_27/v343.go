// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"

	"xorm.io/xorm/schemas"
)

func codespaceIndexes() []*schemas.Index {
	userStatus := schemas.NewIndex("user_status", schemas.IndexType)
	userStatus.AddColumn("user_id", "status")

	repoStatus := schemas.NewIndex("repo_status", schemas.IndexType)
	repoStatus.AddColumn("repo_id", "status")

	createClaim := schemas.NewIndex("create_claim", schemas.IndexType)
	createClaim.AddColumn("status", "operation_type", "operation_status", "manager_id", "repo_tag", "operation_created_unix", "uuid")

	managerActive := schemas.NewIndex("manager_active", schemas.IndexType)
	managerActive.AddColumn("manager_id", "operation_type", "operation_status", "status", "operation_created_unix", "uuid")

	queuedTimeout := schemas.NewIndex("queued_timeout", schemas.IndexType)
	queuedTimeout.AddColumn("operation_status", "operation_created_unix", "uuid")

	runningTimeout := schemas.NewIndex("running_timeout", schemas.IndexType)
	runningTimeout.AddColumn("operation_status", "operation_deadline_unix", "uuid")

	failedRetention := schemas.NewIndex("failed_retention", schemas.IndexType)
	failedRetention.AddColumn("status", "updated_unix", "uuid")

	return []*schemas.Index{userStatus, repoStatus, createClaim, managerActive, queuedTimeout, runningTimeout, failedRetention}
}

func codespaceManagerIndexes() []*schemas.Index {
	ownerRuntime := schemas.NewIndex("owner_runtime", schemas.IndexType)
	ownerRuntime.AddColumn("owner_id", "runtime_state")

	runtimeOnline := schemas.NewIndex("runtime_online", schemas.IndexType)
	runtimeOnline.AddColumn("runtime_state", "last_online_unix")

	return []*schemas.Index{ownerRuntime, runtimeOnline}
}

func AddCodespaceTables(x base.EngineMigration) error {
	type Codespace struct {
		UUID                   string `xorm:"pk CHAR(36)"`
		UserID                 int64  `xorm:"NOT NULL DEFAULT 0"`
		RepoID                 int64  `xorm:"NOT NULL DEFAULT 0"`
		RefType                string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
		RefName                string `xorm:"TEXT NOT NULL"`
		RepoTag                string `xorm:"VARCHAR(64) NOT NULL DEFAULT 'default'"`
		GitProtocol            string `xorm:"VARCHAR(8) NOT NULL DEFAULT 'http'"`
		CommitSHA              string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
		ManagerID              int64  `xorm:"NOT NULL DEFAULT 0"`
		Status                 string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
		OperationRVersion      int64  `xorm:"NOT NULL DEFAULT 0"`
		OperationType          string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
		OperationStatus        string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
		OperationTrigger       string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
		OperationCreatedUnix   int64  `xorm:"NOT NULL DEFAULT 0"`
		OperationStartedUnix   int64  `xorm:"NOT NULL DEFAULT 0"`
		OperationDeadlineUnix  int64  `xorm:"NOT NULL DEFAULT 0"`
		RuntimeGeneration      int64  `xorm:"NOT NULL DEFAULT 0"`
		LastActiveUnix         int64  `xorm:"NOT NULL DEFAULT 0"`
		AutoStopMode           string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'default'"`
		AutoStopTimeoutSeconds int64  `xorm:"NOT NULL DEFAULT 0"`
		InteractionGeneration  int64  `xorm:"NOT NULL DEFAULT 0"`
		CreatedUnix            int64  `xorm:"NOT NULL DEFAULT 0"`
		UpdatedUnix            int64  `xorm:"NOT NULL DEFAULT 0"`
		StoppedUnix            int64  `xorm:"NOT NULL DEFAULT 0"`
		LogFilename            string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
		LogLineCount           int64  `xorm:"NOT NULL DEFAULT 0"`
		LogSize                int64  `xorm:"NOT NULL DEFAULT 0"`
	}

	type Manager struct {
		ID                  int64
		Name                string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"`
		OwnerID             int64  `xorm:"NOT NULL DEFAULT 0"`
		SecretHash          string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
		SecretSalt          string `xorm:"VARCHAR(32) NOT NULL DEFAULT ''"`
		TagsJSON            string `xorm:"TEXT NOT NULL"`
		RuntimeState        string `xorm:"VARCHAR(16) NOT NULL DEFAULT 'recovering'"`
		LastOnlineUnix      int64  `xorm:"NOT NULL DEFAULT 0"`
		InventoryGeneration int64  `xorm:"NOT NULL DEFAULT 0"`
		CreatedUnix         int64  `xorm:"NOT NULL DEFAULT 0"`
		MetaJSON            string `xorm:"TEXT NOT NULL"`
	}

	type ManagerAddress struct {
		ID        int64
		ManagerID int64  `xorm:"NOT NULL DEFAULT 0 unique(manager_kind)"`
		Kind      string `xorm:"VARCHAR(16) NOT NULL DEFAULT '' unique(manager_kind) unique(kind_address)"`
		Address   string `xorm:"VARCHAR(512) NOT NULL DEFAULT '' unique(kind_address)"`
	}

	type ManagerToken struct {
		ID      int64
		Token   string             `xorm:"VARCHAR(64) NOT NULL UNIQUE"`
		OwnerID int64              `xorm:"NOT NULL DEFAULT 0 UNIQUE"`
		Created timeutil.TimeStamp `xorm:"created"`
		Updated timeutil.TimeStamp `xorm:"updated"`
	}

	type GiteaToken struct {
		CodespaceUUID  string `xorm:"pk CHAR(36)"`
		TokenHash      string `xorm:"VARCHAR(100) NOT NULL UNIQUE"`
		TokenSalt      string `xorm:"VARCHAR(10) NOT NULL"`
		TokenLastEight string `xorm:"VARCHAR(8) NOT NULL index"`
		TokenEncrypted string `xorm:"TEXT NOT NULL"`
		CreatedUnix    int64  `xorm:"NOT NULL DEFAULT 0"`
	}

	type SSHKey struct {
		CodespaceUUID string `xorm:"pk CHAR(36)"`
		KeyID         int64  `xorm:"NOT NULL UNIQUE"`
		CreatedUnix   int64  `xorm:"NOT NULL DEFAULT 0"`
	}

	if err := x.Table("codespace").Sync(new(Codespace)); err != nil {
		return err
	}
	if err := x.Table("codespace_manager").Sync(new(Manager)); err != nil {
		return err
	}
	if err := x.Table("codespace_manager_address").Sync(new(ManagerAddress)); err != nil {
		return err
	}
	if err := x.Table("codespace_manager_token").Sync(new(ManagerToken)); err != nil {
		return err
	}
	if err := x.Table("codespace_gitea_token").Sync(new(GiteaToken)); err != nil {
		return err
	}
	if err := x.Table("codespace_ssh_key").Sync(new(SSHKey)); err != nil {
		return err
	}

	// These indexes match the service query order; field tags cannot express all column orders safely.
	if err := createCodespaceMigrationIndexes(x, "codespace", codespaceIndexes()...); err != nil {
		return err
	}
	return createCodespaceMigrationIndexes(x, "codespace_manager", codespaceManagerIndexes()...)
}

func createCodespaceMigrationIndexes(x base.EngineMigration, tableName string, indexes ...*schemas.Index) error {
	existingIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), tableName)
	if err != nil {
		return err
	}

	for _, index := range indexes {
		if _, ok := existingIndexes[index.Name]; ok {
			continue
		}
		if _, ok := existingIndexes[index.XName(tableName)]; ok {
			continue
		}
		if _, err := x.Exec(x.Dialect().CreateIndexSQL(tableName, index)); err != nil {
			return err
		}
	}
	return nil
}
