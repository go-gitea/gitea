// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This artifact server is inspired by https://github.com/nektos/act/blob/master/pkg/artifacts/server.go.
// It updates url setting and uses ObjectStore to handle artifacts persistence.

package actions

import (
	"context"

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
	RunID            int64 `xorm:"index unique(run-id-name)"` // The run id of the artifact
	RunnerID         int64
	RepoID           int64 `xorm:"index"`
	OwnerID          int64
	CommitSHA        string
	StoragePath      string             // The path to the artifact in the storage
	FileSize         int64              // The size of the artifact in bytes
	FileGzipSize     int64              // The size of the artifact in bytes after gzip compression
	ContentEncnoding string             // The content encoding of the artifact
	ArtifactPath     string             // The path to the artifact when runner uploads it
	ArtifactName     string             `xorm:"UNIQUE(runid_name)"` // The name of the artifact when runner uploads it
	UploadStatus     int64              `xorm:"index"`              // The status of the artifact upload
	Created          timeutil.TimeStamp `xorm:"created"`
	Updated          timeutil.TimeStamp `xorm:"updated index"`
}

// CreateArtifact create a new artifact with task info or get same named artifact in the same run
func CreateArtifact(ctx context.Context, t *ActionTask, artifactName string) (*ActionArtifact, error) {
	if t.Job == nil {
		if err := t.LoadJob(ctx); err != nil {
			return nil, err
		}
	}
	if err := t.Job.LoadRun(ctx); err != nil {
		return nil, err
	}
	artifact, err := GetArtifactByArtifactName(ctx, t.Job.RunID, artifactName)
	if err != nil {
		return nil, err
	}
	if artifact != nil {
		return artifact, nil
	}
	artifact = &ActionArtifact{
		RunID:        t.Job.RunID,
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

// GetArtifactByArtifactName returns an artifact by name
func GetArtifactByArtifactName(ctx context.Context, runID int64, name string) (*ActionArtifact, error) {
	var art ActionArtifact
	has, err := db.GetEngine(ctx).Where("run_id = ? AND artifact_name = ?", runID, name).Get(&art)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return &art, nil
}

// GetArtifactByID returns an artifact by id
func GetArtifactByID(ctx context.Context, id int64) (*ActionArtifact, error) {
	var art ActionArtifact
	has, err := db.GetEngine(ctx).ID(id).Get(&art)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, util.NewNotExistErrorf("no ActionArtifact with id %d exists", id)
	}

	return &art, nil
}

// UpdateArtifactByID updates an artifact by id
func UpdateArtifactByID(ctx context.Context, id int64, art *ActionArtifact) error {
	art.ID = id
	_, err := db.GetEngine(ctx).ID(id).AllCols().Update(art)
	return err
}

// ListArtifactByRunID returns all artifacts of a run
func ListArtifactByRunID(ctx context.Context, runID int64) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, 10)
	return arts, db.GetEngine(ctx).Where("run_id=?", runID).Find(&arts)
}

// ListUploadedArtifactByRunID returns all uploaded artifacts of a run
func ListUploadedArtifactByRunID(ctx context.Context, runID int64) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, 10)
	return arts, db.GetEngine(ctx).Where("run_id=? AND upload_status=?", runID, ArtifactUploadStatusConfirmed).Find(&arts)
}

// ListArtifactsByRepoID returns all artifacts of a repo
func ListArtifactsByRepoID(ctx context.Context, repoID int64) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, 10)
	return arts, db.GetEngine(ctx).Where("repo_id=?", repoID).Find(&arts)
}
