// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func overwriteArtifactStorageContent(t *testing.T, artifactID int64, content []byte) {
	t.Helper()
	artifact := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionArtifact{ID: artifactID})
	_, err := storage.ActionsArtifacts.Save(artifact.StoragePath, bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)
}

func TestActionsArtifactPreviewSingleFile(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, "user2")

	req := NewRequestf(t, "GET", "/%s/actions/runs/187/artifacts/artifact-download/preview", repo.FullName())
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "abc.txt")
	assert.Contains(t, resp.Body.String(), "/preview/raw?path=abc.txt")

	req = NewRequestf(t, "GET", "/%s/actions/runs/187/artifacts/artifact-download/preview/raw", repo.FullName())
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, strings.Repeat("A", 1024), resp.Body.String())
	assert.Contains(t, resp.Header().Get("Content-Type"), "text/plain")
}

func TestActionsArtifactPreviewMultiFile(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, "user2")

	req := NewRequestf(t, "GET", "/%s/actions/runs/187/artifacts/multi-file-download/preview", repo.FullName())
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "abc.txt")
	assert.Contains(t, resp.Body.String(), "xyz/def.txt")

	req = NewRequestf(t, "GET", "/%s/actions/runs/187/artifacts/multi-file-download/preview/raw?path=%s", repo.FullName(), url.QueryEscape("xyz/def.txt"))
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, strings.Repeat("C", 1024), resp.Body.String())
}

func TestActionsArtifactPreviewUnsupportedType(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	overwriteArtifactStorageContent(t, 1, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d})

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, "user2")
	req := NewRequestf(t, "GET", "/%s/actions/runs/187/artifacts/artifact-download/preview/raw", repo.FullName())
	session.MakeRequest(t, req, http.StatusUnsupportedMediaType)
}

func TestActionsArtifactPreviewHTMLSandboxCSP(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	overwriteArtifactStorageContent(t, 1, []byte("<html><body><h1>artifact</h1></body></html>"))

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, "user2")
	req := NewRequestf(t, "GET", "/%s/actions/runs/187/artifacts/artifact-download/preview/raw", repo.FullName())
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Header().Get("Content-Security-Policy"), "sandbox")
	assert.Contains(t, resp.Header().Get("Content-Type"), "text/html")
}

func TestActionsArtifactDownloadViewUnchanged(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, "user2")
	req := NewRequestf(t, "GET", "/%s/actions/runs/187/artifacts/artifact-download", repo.FullName())
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Header().Get("Content-Disposition"), "attachment; filename=artifact-download.zip")
}
