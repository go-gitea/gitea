// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/util"

	jsoniter "github.com/json-iterator/go"
	"xorm.io/builder"
	"xorm.io/xorm"
)

func deleteMigrationCredentials(x *xorm.Engine) (err error) {
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
		if err = sess.Limit(batchSize, start).Where(cond, 0).Find(&tasks); err != nil {
			return
		}
		if len(tasks) == 0 {
			break
		}
		if err = sess.Begin(); err != nil {
			return
		}
		for _, t := range tasks {
			if t.PayloadContent, err = removeCredentials(t.PayloadContent); err != nil {
				return
			}
			if _, err = sess.ID(t.ID).Cols("payload_content").Update(t); err != nil {
				return
			}
		}
		if err = sess.Commit(); err != nil {
			return
		}
	}
	return
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
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	err := json.Unmarshal([]byte(payload), &opts)
	if err != nil {
		return "", err
	}

	opts.AuthPassword = ""
	opts.AuthToken = ""
	opts.CloneAddr = util.NewStringURLSanitizer(opts.CloneAddr, true).Replace(opts.CloneAddr)

	confBytes, err := json.Marshal(opts)
	if err != nil {
		return "", err
	}
	return string(confBytes), nil
}
