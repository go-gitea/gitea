// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This artifact server is inspired by the Gitea runner artifact server implementation.
// It updates url setting and uses ObjectStore to handle artifacts persistence.

package actions

import (
	"context"
	"errors"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// ArtifactStatus is the status of an artifact, uploading, expired or need-delete
type ArtifactStatus int64

const (
	ArtifactStatusUploadPending   ArtifactStatus = iota + 1 // 1， ArtifactStatusUploadPending is the status of an artifact upload that is pending
	ArtifactStatusUploadConfirmed                           // 2， ArtifactStatusUploadConfirmed is the status of an artifact upload that is confirmed
	ArtifactStatusUploadError                               // 3， ArtifactStatusUploadError is the status of an artifact upload that is errored
	ArtifactStatusExpired                                   // 4, ArtifactStatusExpired is the status of an artifact that is expired
	ArtifactStatusPendingDeletion                           // 5, ArtifactStatusPendingDeletion is the status of an artifact that is pending deletion
	ArtifactStatusDeleted                                   // 6, ArtifactStatusDeleted is the status of an artifact that is deleted
)

func (status ArtifactStatus) ToString() string {
	switch status {
	case ArtifactStatusUploadPending:
		return "upload is not yet completed"
	case ArtifactStatusUploadConfirmed:
		return "upload is completed"
	case ArtifactStatusUploadError:
		return "upload failed"
	case ArtifactStatusExpired:
		return "expired"
	case ArtifactStatusPendingDeletion:
		return "pending deletion"
	case ArtifactStatusDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

func init() {
	db.RegisterModel(new(ActionArtifact))
}

const (
	ContentEncodingV3Gzip = "gzip"
	ContentTypeZip        = "application/zip"
)

// ActionArtifact is a file that is stored in the artifact storage.
type ActionArtifact struct {
	ID                 int64 `xorm:"pk autoincr"`
	RunID              int64 `xorm:"index unique(runid_attempt_name_path)"` // The run id of the artifact
	RunAttemptID       int64 `xorm:"index unique(runid_attempt_name_path) NOT NULL DEFAULT 0"`
	RunnerID           int64
	RepoID             int64 `xorm:"index"`
	OwnerID            int64
	CommitSHA          string
	StoragePath        string // The path to the artifact in the storage
	FileSize           int64  // The size of the artifact in bytes
	FileCompressedSize int64  // The size of the artifact in bytes after gzip compression

	// The content encoding or content type of the artifact
	// * empty or null: legacy (v3) uncompressed content
	// * magic string "gzip" (ContentEncodingV3Gzip): v3 gzip compressed content
	//   * requires gzip decoding before storing in a zip for download
	//   * requires gzip content-encoding header when downloaded single files within a workflow
	// * mime type for "Content-Type":
	//   * "application/zip" (ContentTypeZip), seems to be an abuse, fortunately there is no conflict, and it won't cause problems?
	//   * "application/pdf", "text/html", etc.: real content type of the artifact
	ContentEncodingOrType string `xorm:"content_encoding"`

	ArtifactPath string             `xorm:"index unique(runid_attempt_name_path)"` // The path to the artifact when runner uploads it
	ArtifactName string             `xorm:"index unique(runid_attempt_name_path)"` // The name of the artifact when runner uploads it
	Status       ArtifactStatus     `xorm:"index"`                                 // The status of the artifact, uploading, expired or need-delete
	CreatedUnix  timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix  timeutil.TimeStamp `xorm:"updated index"`
	ExpiredUnix  timeutil.TimeStamp `xorm:"index"` // The time when the artifact will be expired
}

func CreateArtifact(ctx context.Context, t *ActionTask, artifactName, artifactPath string, expiredDays int64) (*ActionArtifact, error) {
	if err := t.LoadJob(ctx); err != nil {
		return nil, err
	}
	artifact, err := getArtifactByNameAndPath(ctx, t.Job.RunID, t.Job.RunAttemptID, artifactName, artifactPath)
	if errors.Is(err, util.ErrNotExist) {
		artifact := &ActionArtifact{
			ArtifactName: artifactName,
			ArtifactPath: artifactPath,
			RunID:        t.Job.RunID,
			RunAttemptID: t.Job.RunAttemptID,
			RunnerID:     t.RunnerID,
			RepoID:       t.RepoID,
			OwnerID:      t.OwnerID,
			CommitSHA:    t.CommitSHA,
			Status:       ArtifactStatusUploadPending,
			ExpiredUnix:  timeutil.TimeStamp(time.Now().Unix() + timeutil.Day*expiredDays),
		}
		if _, err := db.GetEngine(ctx).Insert(artifact); err != nil {
			return nil, err
		}
		return artifact, nil
	} else if err != nil {
		return nil, err
	}

	if _, err := db.GetEngine(ctx).ID(artifact.ID).Cols("expired_unix").Update(&ActionArtifact{
		ExpiredUnix: timeutil.TimeStamp(time.Now().Unix() + timeutil.Day*expiredDays),
	}); err != nil {
		return nil, err
	}

	return artifact, nil
}

func getArtifactByNameAndPath(ctx context.Context, runID, runAttemptID int64, name, fpath string) (*ActionArtifact, error) {
	var art ActionArtifact
	has, err := db.GetEngine(ctx).Where("run_id = ? AND run_attempt_id = ? AND artifact_name = ? AND artifact_path = ?", runID, runAttemptID, name, fpath).Get(&art)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, util.ErrNotExist
	}
	return &art, nil
}

// UpdateArtifactByID updates an artifact by id
func UpdateArtifactByID(ctx context.Context, id int64, art *ActionArtifact) error {
	art.ID = id
	_, err := db.GetEngine(ctx).ID(id).AllCols().Update(art)
	return err
}

type FindArtifactsOptions struct {
	db.ListOptions
	RepoID               int64
	RunID                int64
	RunAttemptID         optional.Option[int64] // use optional to allow filtering by zero (legacy artifacts have run_attempt_id=0)
	ArtifactName         string
	Status               int
	FinalizedArtifactsV4 bool
}

func (opts FindArtifactsOptions) ToOrders() string {
	return "id"
}

var _ db.FindOptionsOrder = (*FindArtifactsOptions)(nil)

func (opts FindArtifactsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.RunID > 0 {
		cond = cond.And(builder.Eq{"run_id": opts.RunID})
	}
	if opts.RunAttemptID.Has() {
		cond = cond.And(builder.Eq{"run_attempt_id": opts.RunAttemptID.Value()})
	}
	if opts.ArtifactName != "" {
		cond = cond.And(builder.Eq{"artifact_name": opts.ArtifactName})
	}
	if opts.Status > 0 {
		cond = cond.And(builder.Eq{"status": opts.Status})
	}
	if opts.FinalizedArtifactsV4 {
		cond = cond.And(builder.Eq{"status": ArtifactStatusUploadConfirmed}.Or(builder.Eq{"status": ArtifactStatusExpired}))
		// see the comment of ActionArtifact.ContentEncodingOrType: "*/*" means the field is a content type
		cond = cond.And(builder.Like{"content_encoding", "%/%"})
	}

	return cond
}

// ActionArtifactMeta is the meta-data of an artifact
type ActionArtifactMeta struct {
	ArtifactName string
	FileSize     int64
	Status       ArtifactStatus
	ExpiredUnix  timeutil.TimeStamp
}

// ListUploadedArtifactsMetaByRunAttempt returns uploaded artifacts meta scoped to a specific run and attempt.
// Pass runAttemptID=0 to target legacy artifacts (pre-v331) belonging to the run.
func ListUploadedArtifactsMetaByRunAttempt(ctx context.Context, repoID, runID, runAttemptID int64) ([]*ActionArtifactMeta, error) {
	arts := make([]*ActionArtifactMeta, 0, 10)
	return arts, db.GetEngine(ctx).Table("action_artifact").
		Where("repo_id=? AND run_id=? AND run_attempt_id=? AND (status=? OR status=?)", repoID, runID, runAttemptID, ArtifactStatusUploadConfirmed, ArtifactStatusExpired).
		GroupBy("artifact_name").
		Select("artifact_name, sum(file_size) as file_size, max(status) as status, max(expired_unix) as expired_unix").
		Find(&arts)
}

// ListNeedExpiredArtifacts returns all need expired artifacts but not deleted
func ListNeedExpiredArtifacts(ctx context.Context) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, 10)
	return arts, db.GetEngine(ctx).
		Where("expired_unix < ? AND status = ?", timeutil.TimeStamp(time.Now().Unix()), ArtifactStatusUploadConfirmed).Find(&arts)
}

// ListPendingDeleteArtifacts returns all artifacts in pending-delete status.
// limit is the max number of artifacts to return.
func ListPendingDeleteArtifacts(ctx context.Context, limit int) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, limit)
	return arts, db.GetEngine(ctx).
		Where("status = ?", ArtifactStatusPendingDeletion).Limit(limit).Find(&arts)
}

// SetArtifactExpired sets an artifact to expired
func SetArtifactExpired(ctx context.Context, artifactID int64) error {
	_, err := db.GetEngine(ctx).Where("id=? AND status = ?", artifactID, ArtifactStatusUploadConfirmed).Cols("status").Update(&ActionArtifact{Status: ArtifactStatusExpired})
	return err
}

// SetArtifactNeedDeleteByID sets an artifact to need-delete by ID, cron job will delete it.
func SetArtifactNeedDeleteByID(ctx context.Context, artifactID int64) error {
	_, err := db.GetEngine(ctx).Where("id=? AND status = ?", artifactID, ArtifactStatusUploadConfirmed).Cols("status").Update(&ActionArtifact{Status: ArtifactStatusPendingDeletion})
	return err
}

// SetArtifactNeedDeleteByRunAttempt sets an artifact to need-delete in a run attempt, cron job will delete it.
// runAttemptID may be 0 for legacy artifacts created before ActionRunAttempt existed.
func SetArtifactNeedDeleteByRunAttempt(ctx context.Context, runID, runAttemptID int64, name string) error {
	_, err := db.GetEngine(ctx).Where("run_id=? AND run_attempt_id=? AND artifact_name=? AND status = ?", runID, runAttemptID, name, ArtifactStatusUploadConfirmed).Cols("status").Update(&ActionArtifact{Status: ArtifactStatusPendingDeletion})
	return err
}

// GetArtifactsByRunAttemptAndName returns all artifacts with the given name in the specified run attempt.
// This supports both attempt-scoped data and legacy artifacts with run_attempt_id=0.
func GetArtifactsByRunAttemptAndName(ctx context.Context, runID, runAttemptID int64, artifactName string) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0)
	return arts, db.GetEngine(ctx).
		Where("run_id = ? AND run_attempt_id = ? AND artifact_name = ?", runID, runAttemptID, artifactName).
		OrderBy("id").
		Find(&arts)
}

// SetArtifactDeleted sets an artifact to deleted
func SetArtifactDeleted(ctx context.Context, artifactID int64) error {
	_, err := db.GetEngine(ctx).ID(artifactID).Cols("status").Update(&ActionArtifact{Status: ArtifactStatusDeleted})
	return err
}
