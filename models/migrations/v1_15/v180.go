// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_15 //nolint

import (
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm"
)

func DeleteMigrationCredentials(x *xorm.Engine) (err error) {
	// Task represents a task
	type Task struct {
		ID             int64
		DoerID         int64 `xorm:"index"` // operator
		OwnerID        int64 `xorm:"index"` // repo owner id, when creating, the repoID maybe zero
		RepoID         int64 `xorm:"index"`
		Type           int
		Status         int `xorm:"index"`
		StartTime      int64
		EndTime        int64
		PayloadContent string `xorm:"TEXT"`
		Errors         string `xorm:"TEXT"` // if task failed, saved the error reason
		Created        int64  `xorm:"created"`
	}

	const TaskTypeMigrateRepo = 0
	const TaskStatusStopped = 2

	const batchSize = 100

	// only match migration tasks, that are not pending or running
	cond := builder.Eq{
		"type": TaskTypeMigrateRepo,
	}.And(builder.Gte{
		"status": TaskStatusStopped,
	})

	sess := x.NewSession()
	defer sess.Close()

	for start := 0; ; start += batchSize {
		tasks := make([]*Task, 0, batchSize)
		if err := sess.Limit(batchSize, start).Where(cond, 0).Find(&tasks); err != nil {
			return err
		}
		if len(tasks) == 0 {
			break
		}
		if err := sess.Begin(); err != nil {
			return err
		}
		for _, t := range tasks {
			if t.PayloadContent, err = removeCredentials(t.PayloadContent); err != nil {
				return err
			}
			if _, err := sess.ID(t.ID).Cols("payload_content").Update(t); err != nil {
				return err
			}
		}
		if err := sess.Commit(); err != nil {
			return err
		}
	}
	return err
}

func removeCredentials(payload string) (string, error) {
	// MigrateOptions defines the way a repository gets migrated
	// this is for internal usage by migrations module and func who interact with it
	type MigrateOptions struct {
		// required: true
		CloneAddr             string `json:"clone_addr" binding:"Required"`
		CloneAddrEncrypted    string `json:"clone_addr_encrypted,omitempty"`
		AuthUsername          string `json:"auth_username"`
		AuthPassword          string `json:"-"`
		AuthPasswordEncrypted string `json:"auth_password_encrypted,omitempty"`
		AuthToken             string `json:"-"`
		AuthTokenEncrypted    string `json:"auth_token_encrypted,omitempty"`
		// required: true
		UID int `json:"uid" binding:"Required"`
		// required: true
		RepoName        string `json:"repo_name" binding:"Required"`
		Mirror          bool   `json:"mirror"`
		LFS             bool   `json:"lfs"`
		LFSEndpoint     string `json:"lfs_endpoint"`
		Private         bool   `json:"private"`
		Description     string `json:"description"`
		OriginalURL     string
		GitServiceType  int
		Wiki            bool
		Issues          bool
		Milestones      bool
		Labels          bool
		Releases        bool
		Comments        bool
		PullRequests    bool
		ReleaseAssets   bool
		MigrateToRepoID int64
		MirrorInterval  string `json:"mirror_interval"`
	}

	var opts MigrateOptions
	err := json.Unmarshal([]byte(payload), &opts)
	if err != nil {
		return "", err
	}

	opts.AuthPassword = ""
	opts.AuthToken = ""
	opts.CloneAddr = util.SanitizeCredentialURLs(opts.CloneAddr)

	confBytes, err := json.Marshal(opts)
	if err != nil {
		return "", err
	}
	return string(confBytes), nil
}
