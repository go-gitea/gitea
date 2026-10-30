// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
	actions_service "gitea.dev/services/actions"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrActionsDeploymentRequest = errors.New("invalid Pages Actions deployment request")
	ErrActionsDeploymentAuth    = errors.New("Pages Actions deployment is not authorized")
	ErrActionsArtifact          = errors.New("Pages Actions artifact is unavailable")
	ErrActionsPreview           = errors.New("Pages preview deployments are not supported")
)

// ActionsDeploymentRequest is kept in the service package so callers cannot
// bypass the checks performed before an artifact is converted.
type ActionsDeploymentRequest struct {
	ArtifactID        int64
	PagesBuildVersion string
	OIDCToken         string
	Preview           bool
}

type ActionsDeploymentResult struct {
	Digest  string
	PageURL string
}

type ActionsDeploymentStatus string

const (
	ActionsDeploymentStatusInProgress ActionsDeploymentStatus = "in_progress"
	ActionsDeploymentStatusSuccess    ActionsDeploymentStatus = "success"
	ActionsDeploymentStatusFailure    ActionsDeploymentStatus = "failure"
	ActionsDeploymentStatusCancelled  ActionsDeploymentStatus = "cancelled"
)

type actionsOIDCClaims struct {
	jwt.RegisteredClaims
	RepositoryID      string `json:"repository_id"`
	RepositoryOwnerID string `json:"repository_owner_id"`
	RunID             string `json:"run_id"`
	RunAttempt        string `json:"run_attempt"`
	Ref               string `json:"ref"`
	RefType           string `json:"ref_type"`
	SHA               string `json:"sha"`
}

// CommitActionsArtifact validates the authenticated Actions task and converts
// its single artifact.tar ZIP member to the common Pages TAR.GZ object.
func CommitActionsArtifact(ctx context.Context, repoID int64, task *actions_model.ActionTask, req ActionsDeploymentRequest) (*ActionsDeploymentResult, error) {
	if repoID <= 0 || task == nil || task.ID <= 0 || req.ArtifactID <= 0 || req.PagesBuildVersion == "" || req.OIDCToken == "" {
		return nil, ErrActionsDeploymentRequest
	}
	if req.Preview {
		return nil, ErrActionsPreview
	}
	if task.RepoID != repoID || task.Status != actions_model.StatusRunning {
		return nil, ErrActionsDeploymentAuth
	}
	if err := task.LoadJob(ctx); err != nil {
		return nil, fmt.Errorf("CommitActionsArtifact: load job: %w", err)
	}
	if task.Job == nil || task.Job.RepoID != repoID {
		return nil, ErrActionsDeploymentAuth
	}
	if err := task.Job.LoadAttributes(ctx); err != nil {
		return nil, fmt.Errorf("CommitActionsArtifact: load job attributes: %w", err)
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			return nil, ErrActionsDeploymentAuth
		}
		return nil, fmt.Errorf("CommitActionsArtifact: load repository: %w", err)
	}
	if repo.IsArchived || repo.IsEmpty {
		return nil, ErrActionsDeploymentAuth
	}
	if err := repo.LoadOwner(ctx); err != nil {
		return nil, fmt.Errorf("CommitActionsArtifact: load repository owner: %w", err)
	}
	perms, err := actions_model.ComputeTaskTokenPermissions(ctx, task, repo)
	if err != nil {
		return nil, fmt.Errorf("CommitActionsArtifact: compute task permissions: %w", err)
	}
	if !perms.Pages || !perms.IDToken {
		return nil, ErrActionsDeploymentAuth
	}
	claims, err := verifyActionsOIDC(ctx, req.OIDCToken, repo, task)
	if err != nil {
		return nil, fmt.Errorf("CommitActionsArtifact: %w: %v", ErrActionsDeploymentAuth, err)
	}
	if req.PagesBuildVersion != task.Job.CommitSHA {
		return nil, ErrActionsDeploymentRequest
	}
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return nil, err
	}
	if !has || site.Deleted || site.SourceType != pages_model.PublishSourceArtifact {
		return nil, ErrSourceChanged
	}
	if !actionsRefAllowed(site, claims) {
		return nil, ErrActionsDeploymentAuth
	}
	artifact, err := getPagesArtifactForTask(ctx, task, req.ArtifactID)
	if err != nil {
		return nil, err
	}
	if artifact.CommitSHA != req.PagesBuildVersion {
		return nil, ErrActionsArtifact
	}
	candidate, cleanup, err := convertPagesArtifact(artifact)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer candidate.Close()
	result, err := commitContent(ctx, repoID, candidate, contentCommitOptions{
		Origin: pages_model.VersionOriginActions, RequiredSource: pages_model.PublishSourceArtifact,
		ActionRunID: task.Job.RunID,
		ValidateSource: func(_ context.Context, site *pages_model.Site) error {
			if !actionsRefAllowed(site, claims) {
				return ErrActionsDeploymentAuth
			}
			return nil
		},
		AcceptVersion: func(ctx context.Context, _ *pages_model.Version) error {
			return actions_model.SetRunHasPagesArtifact(ctx, repoID, task.Job.RunID)
		},
	})
	if err != nil {
		return nil, err
	}
	return &ActionsDeploymentResult{
		Digest: result.Version.TarGzSHA256, PageURL: FirstDefaultSiteURL(ctx, repo),
	}, nil
}

func actionsRefAllowed(site *pages_model.Site, claims *actionsOIDCClaims) bool {
	return site.SourceBranch == "" || (claims.RefType == "branch" && claims.Ref == "refs/heads/"+site.SourceBranch)
}

func getPagesArtifactForTask(ctx context.Context, task *actions_model.ActionTask, artifactID int64) (*actions_model.ActionArtifact, error) {
	if task == nil || task.Job == nil {
		return nil, ErrActionsArtifact
	}
	attempts, err := actions_model.GetArtifactAttemptIDs(ctx, task.Job)
	if err != nil {
		return nil, err
	}
	artifact, has, err := db.GetByID[actions_model.ActionArtifact](ctx, artifactID)
	if err != nil {
		return nil, fmt.Errorf("getPagesArtifactForTask: load artifact: %w", err)
	}
	if !has {
		return nil, ErrActionsArtifact
	}
	if artifact.RepoID != task.RepoID || artifact.RunID != task.Job.RunID ||
		!slices.Contains(attempts, artifact.RunAttemptID) || artifact.Status != actions_model.ArtifactStatusUploadConfirmed {
		return nil, ErrActionsArtifact
	}
	if artifact.ExpiredUnix != 0 && artifact.ExpiredUnix <= timeutil.TimeStampNow() {
		return nil, ErrActionsArtifact
	}
	if artifact.StoragePath == "" || artifact.ContentEncodingOrType != actions_model.ContentTypeZip {
		return nil, ErrActionsArtifact
	}
	return artifact, nil
}

func convertPagesArtifact(artifact *actions_model.ActionArtifact) (*os.File, func(), error) {
	limits := archiveLimits()
	maxArtifactZipSize := limits.MaxCompressedBytes + 1024*1024
	object, err := storage.ActionsArtifacts.Open(artifact.StoragePath)
	if err != nil {
		return nil, nil, fmt.Errorf("convertPagesArtifact: open Actions artifact: %w", err)
	}
	defer object.Close()
	info, err := storage.ActionsArtifacts.Stat(artifact.StoragePath)
	if err != nil || info.Size() < 0 || info.Size() > maxArtifactZipSize {
		return nil, nil, ErrActionsArtifact
	}
	work, cleanup, err := setting.AppDataTempDir("pages").MkdirTempRandom("actions-*")
	if err != nil {
		return nil, nil, err
	}
	complete := false
	defer func() {
		if !complete {
			cleanup()
		}
	}()
	zipPath := filepath.Join(work, "artifact.zip")
	zipFile, err := os.OpenFile(zipPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, err
	}
	_, copyErr := io.Copy(zipFile, io.LimitReader(object, maxArtifactZipSize+1))
	closeErr := zipFile.Close()
	if copyErr != nil || closeErr != nil {
		return nil, nil, errors.Join(copyErr, closeErr)
	}
	zipStat, err := os.Stat(zipPath)
	if err != nil {
		return nil, nil, err
	}
	if zipStat.Size() > maxArtifactZipSize {
		return nil, nil, pages_content.ErrArchiveTooLarge
	}
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, fmt.Errorf("convertPagesArtifact: read Actions artifact ZIP: %w", err)
	}
	defer zipReader.Close()
	if len(zipReader.File) != 1 || zipReader.File[0].Name != "artifact.tar" || !zipReader.File[0].FileInfo().Mode().IsRegular() {
		return nil, nil, ErrActionsArtifact
	}
	entry := zipReader.File[0]
	if entry.UncompressedSize64 > uint64(limits.MaxTarBytes) {
		return nil, nil, pages_content.ErrArchiveTooLarge
	}
	in, err := entry.Open()
	if err != nil {
		return nil, nil, err
	}
	defer in.Close()
	candidatePath := filepath.Join(work, "pages.tar.gz")
	candidate, err := os.OpenFile(candidatePath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, nil, err
	}
	gz := gzip.NewWriter(candidate)
	gz.Header.ModTime = time.Unix(0, 0)
	written, copyErr := io.Copy(gz, io.LimitReader(in, limits.MaxTarBytes+1))
	gzCloseErr := gz.Close()
	if copyErr != nil || gzCloseErr != nil {
		_ = candidate.Close()
		return nil, nil, errors.Join(copyErr, gzCloseErr)
	}
	if written != int64(entry.UncompressedSize64) || written > limits.MaxTarBytes {
		_ = candidate.Close()
		return nil, nil, pages_content.ErrArchiveTooLarge
	}
	if _, err := candidate.Seek(0, io.SeekStart); err != nil {
		_ = candidate.Close()
		return nil, nil, err
	}
	complete = true
	return candidate, cleanup, nil
}

func verifyActionsOIDC(ctx context.Context, raw string, repo *repo_model.Repository, task *actions_model.ActionTask) (*actionsOIDCClaims, error) {
	key, err := actions_service.OIDCSigningKey()
	if err != nil {
		return nil, err
	}
	claims := new(actionsOIDCClaims)
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != actions_service.OIDCSigningAlgorithm {
			return nil, errors.New("unexpected OIDC signing algorithm")
		}
		return key.VerifyKey(), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid OIDC token")
	}
	if claims.Issuer != actions_service.OIDCIssuer() || !slices.Contains(claims.Audience, setting.AppURL+url.PathEscape(repo.OwnerName)) {
		return nil, errors.New("OIDC issuer or audience mismatch")
	}
	if claims.RepositoryID != strconv.FormatInt(repo.ID, 10) || claims.RepositoryOwnerID != strconv.FormatInt(repo.OwnerID, 10) ||
		claims.RunID != strconv.FormatInt(task.Job.RunID, 10) || claims.SHA != task.Job.CommitSHA || claims.Ref != task.Job.Run.Ref {
		return nil, errors.New("OIDC repository or run mismatch")
	}
	attempt, err := actions_model.GetRunAttemptByRepoAndID(ctx, repo.ID, task.Job.RunAttemptID)
	if err != nil || claims.RunAttempt != strconv.FormatInt(attempt.Attempt, 10) {
		return nil, errors.New("OIDC run attempt mismatch")
	}
	return claims, nil
}

// GetActionsDeploymentStatus reports the flat current-content status used by
// actions/deploy-pages polling. It intentionally has no deployment history.
func GetActionsDeploymentStatus(ctx context.Context, repoID int64, digest string) (ActionsDeploymentStatus, error) {
	if _, err := pages_content.ContentRelativePath(repoID, digest); err != nil {
		return "", ErrActionsDeploymentRequest
	}
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return "", err
	}
	if !has || site.Deleted {
		return "", util.ErrNotExist
	}
	version, has, err := pages_model.GetCurrentVersion(ctx, site)
	if err != nil {
		return "", err
	}
	if !site.Enabled || !has || version.TarGzSHA256 != digest {
		return ActionsDeploymentStatusCancelled, nil
	}
	instances, err := pages_model.ListInstances(ctx)
	if err != nil {
		return "", err
	}
	serviceCount := 0
	allPublished := true
	results, err := pages_model.ListInstanceResultsByRepoID(ctx, repoID)
	if err != nil {
		return "", err
	}
	resultByInstance := make(map[int64]*pages_model.InstanceResult, len(results))
	for _, result := range results {
		resultByInstance[result.InstanceID] = result
	}
	for _, instance := range instances {
		if instance.RootURL == "" {
			continue
		}
		serviceCount++
		result := resultByInstance[instance.ID]
		if result == nil || result.AppliedPagesUpdatedUnixNano != site.PagesUpdatedUnixNano ||
			result.AppliedRootUpdatedUnixNano != instance.RootUpdatedUnixNano {
			allPublished = false
			continue
		}
		// A result with the current target and an error means the instance
		// explicitly rejected this deployment. A default address can still be
		// successful while a custom domain reports its own certificate or
		// entry error, so that error does not block the default deployment.
		if result.ErrorCode != "" && !result.DefaultPublished {
			return ActionsDeploymentStatusFailure, nil
		}
		if !result.ConfigApplied || result.ActualTarGzSHA256 != digest || !result.DefaultPublished {
			allPublished = false
			continue
		}
	}
	if serviceCount > 0 && allPublished {
		return ActionsDeploymentStatusSuccess, nil
	}
	return ActionsDeploymentStatusInProgress, nil
}
