// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// This artifact server is inspired by the Gitea runner artifact server implementation.
// It updates url setting and uses ObjectStore to handle artifacts persistence.

package actions

import (
	"context"
	"slices"

	"gitea.dev/models/db"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

// ArtifactStatus is the status of an artifact, uploading, expired or need-delete
type ArtifactStatus int64

const (
	ArtifactStatusUploadPending ArtifactStatus = iota + 1
	ArtifactStatusUploadConfirmed
	ArtifactStatusUploadError // unused, kept so the numbering below stays stable
	ArtifactStatusExpired
	ArtifactStatusPendingDeletion
	ArtifactStatusDeleted
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
	ExpiredUnix  timeutil.TimeStamp `xorm:"index"` // 0 means the artifact is kept forever
}

const artifactKeepForever timeutil.TimeStamp = 0

func artifactExpiry(requested optional.Option[timeutil.TimeStamp]) timeutil.TimeStamp {
	if requested.Has() {
		return max(requested.Value(), artifactKeepForever+1)
	}
	if setting.Actions.ArtifactRetentionDays <= 0 {
		return artifactKeepForever
	}
	return timeutil.TimeStampNow().Add(timeutil.Day * setting.Actions.ArtifactRetentionDays)
}

// CreateArtifact returns the artifact for the name and path, creating it on first upload and refreshing its expiry either way.
func CreateArtifact(ctx context.Context, t *ActionTask, artifactName, artifactPath string, expiry optional.Option[timeutil.TimeStamp]) (*ActionArtifact, error) {
	if err := t.LoadJob(ctx); err != nil {
		return nil, err
	}
	expiredUnix := artifactExpiry(expiry)

	artifact, exist, err := db.Get[ActionArtifact](ctx, builder.Eq{
		"run_id": t.Job.RunID, "run_attempt_id": t.Job.RunAttemptID,
		"artifact_name": artifactName, "artifact_path": artifactPath,
	})
	if err != nil {
		return nil, err
	}
	if !exist {
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
			ExpiredUnix:  expiredUnix,
		}
		if _, err := db.GetEngine(ctx).Insert(artifact); err != nil {
			return nil, err
		}
		return artifact, nil
	}

	artifact.ExpiredUnix = expiredUnix
	if err := UpdateArtifact(ctx, artifact, "expired_unix"); err != nil {
		return nil, err
	}

	return artifact, nil
}

func UpdateArtifact(ctx context.Context, art *ActionArtifact, cols ...string) error {
	_, err := db.GetEngine(ctx).ID(art.ID).Cols(cols...).Update(art)
	return err
}

type FindArtifactsOptions struct {
	db.ListOptions
	RepoID               int64
	RunID                int64
	RunAttemptIDs        []int64 // empty means every attempt; pass 0 to target legacy artifacts, which have run_attempt_id=0
	ArtifactName         string
	Status               ArtifactStatus
	FinalizedArtifactsV4 bool
}

func (opts FindArtifactsOptions) ToOrders() string {
	return "id"
}

var _ db.FindOptions = (*FindArtifactsOptions)(nil)

func (opts FindArtifactsOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"repo_id": opts.RepoID})
	}
	if opts.RunID > 0 {
		cond = cond.And(builder.Eq{"run_id": opts.RunID})
	}
	if len(opts.RunAttemptIDs) > 0 {
		cond = cond.And(builder.In("run_attempt_id", opts.RunAttemptIDs))
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

// FindReadableArtifacts returns the artifacts of opts.RunAttemptIDs, only keeps the ones from a newer attempt.
func FindReadableArtifacts(ctx context.Context, opts FindArtifactsOptions) ([]*ActionArtifact, error) {
	arts, err := db.Find[ActionArtifact](ctx, opts)
	if err != nil || len(opts.RunAttemptIDs) <= 1 {
		return arts, err
	}
	return keepLatestAttemptArtifacts(arts), nil
}

// keepLatestAttemptArtifacts keeps, per name, only the artifacts of the newest attempt that has it.
// A v3 artifact is one row per uploaded file, so the whole group of the winning attempt is kept.
func keepLatestAttemptArtifacts(arts []*ActionArtifact) []*ActionArtifact {
	latest := make(map[string]int64)
	for _, art := range arts {
		latest[art.ArtifactName] = max(latest[art.ArtifactName], art.RunAttemptID)
	}
	return slices.DeleteFunc(arts, func(art *ActionArtifact) bool {
		return art.RunAttemptID != latest[art.ArtifactName]
	})
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
		Where("expired_unix > ? AND expired_unix < ? AND status = ?", artifactKeepForever, timeutil.TimeStampNow(), ArtifactStatusUploadConfirmed).Find(&arts)
}

// ListPendingDeleteArtifacts returns all artifacts in pending-delete status.
// limit is the max number of artifacts to return.
func ListPendingDeleteArtifacts(ctx context.Context, limit int) ([]*ActionArtifact, error) {
	arts := make([]*ActionArtifact, 0, limit)
	return arts, db.GetEngine(ctx).
		Where("status = ?", ArtifactStatusPendingDeletion).Limit(limit).Find(&arts)
}

func setConfirmedArtifactsStatus(ctx context.Context, status ArtifactStatus, cond builder.Cond) error {
	_, err := db.GetEngine(ctx).Where(cond).And(builder.Eq{"status": ArtifactStatusUploadConfirmed}).
		Cols("status").Update(&ActionArtifact{Status: status})
	return err
}

func SetArtifactExpired(ctx context.Context, artifactID int64) error {
	return setConfirmedArtifactsStatus(ctx, ArtifactStatusExpired, builder.Eq{"id": artifactID})
}

func SetArtifactNeedDeleteByID(ctx context.Context, artifactID int64) error {
	return setConfirmedArtifactsStatus(ctx, ArtifactStatusPendingDeletion, builder.Eq{"id": artifactID})
}

// runAttemptID may be 0 for legacy artifacts created before ActionRunAttempt existed.
func SetArtifactNeedDeleteByRunAttempt(ctx context.Context, runID, runAttemptID int64, name string) error {
	return setConfirmedArtifactsStatus(ctx, ArtifactStatusPendingDeletion,
		builder.Eq{"run_id": runID, "run_attempt_id": runAttemptID, "artifact_name": name})
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

func SetArtifactDeleted(ctx context.Context, artifactID int64) error {
	return UpdateArtifact(ctx, &ActionArtifact{ID: artifactID, Status: ArtifactStatusDeleted}, "status")
}
