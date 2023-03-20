// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This artifact server is inspired by https://github.com/nektos/act/blob/master/pkg/artifacts/server.go.
// It updates url setting and uses ObjectStore to handle artifacts persistence.

package actions

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

const (
	// ArtifactUploadStatusPending is the status of an artifact upload that is pending
	ArtifactUploadStatusPending = 1
	// ArtifactUploadStatusConfirmed is the status of an artifact upload that is confirmed
	ArtifactUploadStatusConfirmed = 2
)

func init() {
	db.RegisterModel(new(ActionArtifact))
}

// ActionArtifact is a file that is stored in the artifact storage.
type ActionArtifact struct {
	ID               int64 `xorm:"pk autoincr"`
	JobID            int64 `xorm:"index"`
	RunnerID         int64
	RepoID           int64 `xorm:"index"`
	OwnerID          int64
	CommitSHA        string
	StoragePath      string             // The path to the artifact in the storage
	FileSize         int64              // The size of the artifact in bytes
	FileGzipSize     int64              // The size of the artifact in bytes after gzip compression
	ContentEncnoding string             // The content encoding of the artifact
	ArtifactPath     string             // The path to the artifact when runner uploads it
	ArtifactName     string             // The name of the artifact when runner uploads it
	UploadStatus     int64              `xorm:"index"` // The status of the artifact upload
	Created          timeutil.TimeStamp `xorm:"created"`
	Updated          timeutil.TimeStamp `xorm:"updated index"`
}

// CreateArtifact creates a new artifact with task info
func CreateArtifact(ctx context.Context, t *ActionTask) (*ActionArtifact, error) {
	artifact := &ActionArtifact{
		JobID:        t.JobID,
		RunnerID:     t.RunnerID,
		RepoID:       t.RepoID,
		OwnerID:      t.OwnerID,
		CommitSHA:    t.CommitSHA,
		UploadStatus: ArtifactUploadStatusPending,
	}
	if _, err := db.GetEngine(ctx).Insert(artifact); err != nil {
		return nil, err
	}
	return artifact, nil
}

// GetArtifactByID returns an artifact by id
func GetArtifactByID(ctx context.Context, id int64) (*ActionArtifact, error) {
	var art ActionArtifact
	has, err := db.GetEngine(ctx).ID(id).Get(&art)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, fmt.Errorf("task with id %d: %w", id, util.ErrNotExist)
	}

	return &art, nil
}

// UpdateArtifactByID updates an artifact by id
func UpdateArtifactByID(ctx context.Context, id int64, art *ActionArtifact) error {
	art.ID = id
	_, err := db.GetEngine(ctx).ID(id).AllCols().Update(art)
	return err
}

// ListArtifactByJobID returns all artifacts of a job
func ListArtifactByJobID(ctx context.Context, jobID int64) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, 10)
	return arts, db.GetEngine(ctx).Where("job_id=?", jobID).Find(&arts)
}

// ListArtifactsByRepoID returns all artifacts of a repo
func ListArtifactsByRepoID(ctx context.Context, repoID int64) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, 10)
	return arts, db.GetEngine(ctx).Where("repo_id=?", repoID).Find(&arts)
}
