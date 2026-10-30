// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/json"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	actions_service "gitea.dev/services/actions"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsDeploymentRejectsIncompleteRequest(t *testing.T) {
	_, err := CommitActionsArtifact(t.Context(), 1, nil, ActionsDeploymentRequest{})
	require.ErrorIs(t, err, ErrActionsDeploymentRequest)

	_, err = CommitActionsArtifact(t.Context(), 1, &actions_model.ActionTask{ID: 1, RepoID: 1}, ActionsDeploymentRequest{
		ArtifactID: 1, PagesBuildVersion: "sha", OIDCToken: "token", Preview: true,
	})
	require.ErrorIs(t, err, ErrActionsPreview)
}

func TestActionsRestrictedBranch(t *testing.T) {
	site := &pages_model.Site{SourceBranch: "main"}
	assert.True(t, actionsRefAllowed(site, &actionsOIDCClaims{Ref: "refs/heads/main", RefType: "branch"}))
	assert.False(t, actionsRefAllowed(site, &actionsOIDCClaims{Ref: "refs/tags/main", RefType: "tag"}))
	assert.False(t, actionsRefAllowed(site, &actionsOIDCClaims{Ref: "refs/heads/other", RefType: "branch"}))
	site.SourceBranch = ""
	assert.True(t, actionsRefAllowed(site, &actionsOIDCClaims{Ref: "refs/tags/main", RefType: "tag"}))
}

func TestPagesDeploymentRequestJSONContract(t *testing.T) {
	request := api.PagesDeploymentOption{
		ArtifactID:        42,
		PagesBuildVersion: "commit-sha",
		OIDCToken:         "oidc-token",
		Preview:           true,
	}
	payload, err := json.Marshal(request)
	require.NoError(t, err)
	assert.JSONEq(t, `{"artifact_id":42,"pages_build_version":"commit-sha","oidc_token":"oidc-token","preview":true}`, string(payload))
}

func TestConvertPagesArtifactUsesCommonArchiveValidation(t *testing.T) {
	backend, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: t.TempDir()})
	require.NoError(t, err)
	defer test.MockVariableValue(&storage.ActionsArtifacts, backend)()

	valid := storePagesArtifact(t, "valid.zip", []archiveTestEntry{{name: "index.html", body: "page"}})
	candidate, cleanup, err := convertPagesArtifact(valid)
	require.NoError(t, err)
	defer cleanup()
	defer candidate.Close()
	manifest, err := pages_content.ValidateArchive(t.Context(), candidate, pages_content.DefaultLimits())
	require.NoError(t, err)
	assert.EqualValues(t, 4, manifest.SiteBytes)

	unsafe := storePagesArtifact(t, "unsafe.zip", []archiveTestEntry{{name: "../index.html", body: "page"}})
	candidate, cleanup, err = convertPagesArtifact(unsafe)
	require.NoError(t, err, "ZIP conversion only extracts the Actions artifact.tar member")
	defer cleanup()
	defer candidate.Close()
	_, err = pages_content.ValidateArchive(t.Context(), candidate, pages_content.DefaultLimits())
	require.ErrorIs(t, err, pages_content.ErrArchivePath, "the common content commit must enforce the Gitea archive boundary")
}

func TestConvertPagesArtifactRejectsUnexpectedZIPMembers(t *testing.T) {
	backend, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: t.TempDir()})
	require.NoError(t, err)
	defer test.MockVariableValue(&storage.ActionsArtifacts, backend)()

	var payload bytes.Buffer
	zw := zip.NewWriter(&payload)
	for _, name := range []string{"artifact.tar", "extra.txt"} {
		entry, err := zw.Create(name)
		require.NoError(t, err)
		_, err = io.WriteString(entry, "data")
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	_, err = backend.Save("extra.zip", bytes.NewReader(payload.Bytes()), int64(payload.Len()))
	require.NoError(t, err)

	_, _, err = convertPagesArtifact(&actions_model.ActionArtifact{StoragePath: "extra.zip"})
	require.ErrorIs(t, err, ErrActionsArtifact)

	payload.Reset()
	zw = zip.NewWriter(&payload)
	_, err = zw.CreateRaw(&zip.FileHeader{
		Name: "artifact.tar", Method: zip.Store,
		CompressedSize64: 0, UncompressedSize64: uint64(pages_content.DefaultLimits().MaxTarBytes + 1),
	})
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	_, err = backend.Save("oversized.zip", bytes.NewReader(payload.Bytes()), int64(payload.Len()))
	require.NoError(t, err)
	_, _, err = convertPagesArtifact(&actions_model.ActionArtifact{StoragePath: "oversized.zip"})
	require.ErrorIs(t, err, pages_content.ErrArchiveTooLarge)
}

func TestCommitActionsArtifactPublishesValidatedContent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	actionsBackend, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: t.TempDir()})
	require.NoError(t, err)
	pagesBackend, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: t.TempDir()})
	require.NoError(t, err)
	defer test.MockVariableValue(&storage.ActionsArtifacts, actionsBackend)()
	defer test.MockVariableValue(&storage.Pages, pagesBackend)()

	task, err := actions_model.GetTaskByID(t.Context(), 47)
	require.NoError(t, err)
	require.NoError(t, task.LoadJob(t.Context()))
	require.NoError(t, task.Job.LoadAttributes(t.Context()))
	permissions := repo_model.MakeWriteAllActionsTokenPermissions()
	task.Job.TokenPermissions = &permissions
	attempt := &actions_model.ActionRunAttempt{
		RepoID: task.RepoID, RunID: task.Job.RunID, Attempt: 1, TriggerUserID: task.Job.Run.TriggerUserID, Status: actions_model.StatusRunning,
	}
	require.NoError(t, db.Insert(t.Context(), attempt))
	task.Job.RunAttemptID = attempt.ID
	task.Job.Attempt = attempt.Attempt

	artifact := storePagesArtifact(t, "pages-success.zip", []archiveTestEntry{{name: "index.html", body: "published"}})
	artifact.RunID = task.Job.RunID
	artifact.RunAttemptID = attempt.ID
	artifact.RunnerID = task.RunnerID
	artifact.RepoID = task.RepoID
	artifact.OwnerID = task.OwnerID
	artifact.CommitSHA = task.Job.CommitSHA
	artifact.ContentEncodingOrType = actions_model.ContentTypeZip
	artifact.ArtifactName = "github-pages"
	artifact.ArtifactPath = "artifact.tar"
	artifact.Status = actions_model.ArtifactStatusUploadConfirmed
	require.NoError(t, db.Insert(t.Context(), artifact))

	_, err = SaveSiteSettings(t.Context(), task.RepoID, SiteSettings{Enabled: true, SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	repo, err := repo_model.GetRepositoryByID(t.Context(), task.RepoID)
	require.NoError(t, err)
	require.NoError(t, repo.LoadOwner(t.Context()))
	oidcToken := signPagesOIDCToken(t, repo, task, attempt.Attempt)
	run, err := actions_model.GetRunByRepoAndID(t.Context(), task.RepoID, task.Job.RunID)
	require.NoError(t, err)
	require.False(t, run.HasPagesArtifact)
	_, err = CommitActionsArtifact(t.Context(), task.RepoID, task, ActionsDeploymentRequest{
		ArtifactID: artifact.ID, PagesBuildVersion: "wrong-commit-sha", OIDCToken: oidcToken,
	})
	require.ErrorIs(t, err, ErrActionsDeploymentRequest)
	run, err = actions_model.GetRunByRepoAndID(t.Context(), task.RepoID, task.Job.RunID)
	require.NoError(t, err)
	require.False(t, run.HasPagesArtifact)

	result, err := CommitActionsArtifact(t.Context(), task.RepoID, task, ActionsDeploymentRequest{
		ArtifactID: artifact.ID, PagesBuildVersion: task.Job.CommitSHA, OIDCToken: oidcToken,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Digest)

	site, has, err := pages_model.GetSiteByRepoID(t.Context(), task.RepoID)
	require.NoError(t, err)
	require.True(t, has)
	version, has, err := pages_model.GetCurrentVersion(t.Context(), site)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, result.Digest, version.TarGzSHA256)
	assert.Equal(t, pages_model.PublishSourceArtifact, site.SourceType)
	run, err = actions_model.GetRunByRepoAndID(t.Context(), task.RepoID, task.Job.RunID)
	require.NoError(t, err)
	assert.True(t, run.HasPagesArtifact)

	repeated, err := CommitActionsArtifact(t.Context(), task.RepoID, task, ActionsDeploymentRequest{
		ArtifactID: artifact.ID, PagesBuildVersion: task.Job.CommitSHA, OIDCToken: oidcToken,
	})
	require.NoError(t, err)
	assert.Equal(t, result.Digest, repeated.Digest)
}

func TestActionsVersionAndRunMarkerCommitAtomically(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	_, err := SaveSiteSettings(t.Context(), 1, SiteSettings{Enabled: true, SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	runs := []*actions_model.ActionRun{
		{RepoID: 1, Index: 1001, WorkflowID: "pages.yaml", Status: actions_model.StatusSuccess},
		{RepoID: 1, Index: 1002, WorkflowID: "pages.yaml", Status: actions_model.StatusSuccess},
	}
	require.NoError(t, db.Insert(t.Context(), runs[0], runs[1]))

	archive := func(content string) []byte {
		t.Helper()
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte(content), 0o600))
		var output bytes.Buffer
		require.NoError(t, pages_content.PackDirectory(t.Context(), root, &output, pages_content.DefaultLimits()))
		return output.Bytes()
	}
	commit := func(runID int64, input []byte) (*ContentCommitResult, error) {
		return commitContent(t.Context(), 1, bytes.NewReader(input), contentCommitOptions{
			Origin: pages_model.VersionOriginActions, RequiredSource: pages_model.PublishSourceArtifact,
			ActionRunID: runID,
			AcceptVersion: func(ctx context.Context, _ *pages_model.Version) error {
				return actions_model.SetRunHasPagesArtifact(ctx, 1, runID)
			},
		})
	}

	content := archive("same content")
	first, err := commit(runs[0].ID, content)
	require.NoError(t, err)
	second, err := commit(runs[1].ID, content)
	require.NoError(t, err)
	require.Equal(t, first.Version.ID, second.Version.ID)
	versions, total, err := pages_model.FindVersions(t.Context(), &pages_model.FindVersionOptions{ListOptions: db.ListOptionsAll, RepoID: 1})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, runs[0].ID, versions[0].OriginActionRunID)
	for _, run := range runs {
		stored, err := actions_model.GetRunByRepoAndID(t.Context(), 1, run.ID)
		require.NoError(t, err)
		require.True(t, stored.HasPagesArtifact)
	}

	_, err = commit(unittest.NonexistentID, archive("must roll back"))
	require.Error(t, err)
	versions, total, err = pages_model.FindVersions(t.Context(), &pages_model.FindVersionOptions{ListOptions: db.ListOptionsAll, RepoID: 1})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, versions, 1)

	_, err = SaveSiteSettings(t.Context(), 1, SiteSettings{SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	_, err = commit(runs[0].ID, archive("closed site"))
	require.ErrorIs(t, err, ErrSourceChanged)
	_, total, err = pages_model.FindVersions(t.Context(), &pages_model.FindVersionOptions{ListOptions: db.ListOptionsAll, RepoID: 1})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
}

type archiveTestEntry struct {
	name string
	body string
}

func storePagesArtifact(t *testing.T, storagePath string, entries []archiveTestEntry) *actions_model.ActionArtifact {
	t.Helper()
	var tarPayload bytes.Buffer
	tw := tar.NewWriter(&tarPayload)
	for _, entry := range entries {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: entry.name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(entry.body))}))
		_, err := io.WriteString(tw, entry.body)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())

	var payload bytes.Buffer
	zw := zip.NewWriter(&payload)
	artifact, err := zw.Create("artifact.tar")
	require.NoError(t, err)
	_, err = artifact.Write(tarPayload.Bytes())
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	_, err = storage.ActionsArtifacts.Save(storagePath, bytes.NewReader(payload.Bytes()), int64(payload.Len()))
	require.NoError(t, err)
	return &actions_model.ActionArtifact{StoragePath: storagePath}
}

func signPagesOIDCToken(t *testing.T, repo *repo_model.Repository, task *actions_model.ActionTask, attempt int64) string {
	t.Helper()
	now := time.Now()
	claims := &actionsOIDCClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: actions_service.OIDCIssuer(), Audience: jwt.ClaimStrings{setting.AppURL + url.PathEscape(repo.OwnerName)},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)), NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)), IssuedAt: jwt.NewNumericDate(now),
		},
		RepositoryID: strconv.FormatInt(repo.ID, 10), RepositoryOwnerID: strconv.FormatInt(repo.OwnerID, 10),
		RunID: strconv.FormatInt(task.Job.RunID, 10), RunAttempt: strconv.FormatInt(attempt, 10),
		Ref: task.Job.Run.Ref, RefType: "branch", SHA: task.Job.CommitSHA,
	}
	key, err := actions_service.OIDCSigningKey()
	require.NoError(t, err)
	token := jwt.NewWithClaims(key.SigningMethod(), claims)
	key.PreProcessToken(token)
	raw, err := token.SignedString(key.SignKey())
	require.NoError(t, err)
	return raw
}
