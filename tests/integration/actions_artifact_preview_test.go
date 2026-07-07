// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"testing"

	actions_model "gitea.dev/models/actions"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildArtifactZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := w.Create(name)
		require.NoError(t, err)
		_, err = fw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

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

	req := NewRequestf(t, "GET", "/%s/actions/runs/791/artifacts/artifact-download/preview", repo.FullName())
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "abc.txt")
	assert.Contains(t, resp.Body.String(), "/preview/raw/abc.txt")
	assert.Contains(t, resp.Body.String(), `target="_blank"`)
	assert.Contains(t, resp.Body.String(), `rel="noopener noreferrer"`)
	assert.NotContains(t, resp.Body.String(), "<iframe")

	req = NewRequestf(t, "GET", "/%s/actions/runs/791/artifacts/artifact-download/preview/raw", repo.FullName())
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, strings.Repeat("A", 1024), resp.Body.String())
	assert.Contains(t, resp.Header().Get("Content-Type"), "text/plain")
}

func TestActionsArtifactPreviewMultiFile(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, "user2")

	req := NewRequestf(t, "GET", "/%s/actions/runs/791/artifacts/multi-file-download/preview", repo.FullName())
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "abc.txt")
	assert.Contains(t, resp.Body.String(), "xyz/def.txt")

	req = NewRequestf(t, "GET", "/%s/actions/runs/791/artifacts/multi-file-download/preview?path=%s", repo.FullName(), url.QueryEscape("xyz/def.txt"))
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "/preview/raw/xyz/def.txt")
	assert.Contains(t, resp.Body.String(), `target="_blank"`)

	req = NewRequestf(t, "GET", "/%s/actions/runs/791/artifacts/multi-file-download/preview/raw/xyz/def.txt", repo.FullName())
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, strings.Repeat("C", 1024), resp.Body.String())
}

func TestActionsArtifactPreviewPathTraversal(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, "user2")

	// A traversal-style path normalizes to a path that doesn't match any file in
	// the artifact. The page still renders with the file list, but no preview is selected.
	req := NewRequestf(t, "GET", "/%s/actions/runs/791/artifacts/multi-file-download/preview?path=%s", repo.FullName(), url.QueryEscape("../../../etc/passwd"))
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "abc.txt")
	assert.NotContains(t, resp.Body.String(), "etc/passwd")
	// The artifact browser is still rendered, but no embedded preview is used.
	assert.NotContains(t, resp.Body.String(), "<iframe")

	// URL-encoded so the router doesn't collapse the segments before the handler sees them.
	req = NewRequestf(t, "GET", "/%s/actions/runs/791/artifacts/multi-file-download/preview/raw/%s", repo.FullName(), "%2E%2E%2F%2E%2E%2Fetc%2Fpasswd")
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestActionsArtifactPreviewUnsupportedType(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	overwriteArtifactStorageContent(t, 1, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d})

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, "user2")
	req := NewRequestf(t, "GET", "/%s/actions/runs/791/artifacts/artifact-download/preview/raw", repo.FullName())
	session.MakeRequest(t, req, http.StatusUnsupportedMediaType)
}

func TestActionsArtifactPreviewHTMLSandboxCSP(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	overwriteArtifactStorageContent(t, 1, []byte("<html><body><h1>artifact</h1></body></html>"))

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, "user2")
	req := NewRequestf(t, "GET", "/%s/actions/runs/791/artifacts/artifact-download/preview/raw", repo.FullName())
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Header().Get("Content-Security-Policy"), "sandbox")
	assert.Contains(t, resp.Header().Get("Content-Security-Policy"), "default-src 'none'")
	assert.Contains(t, resp.Header().Get("Content-Security-Policy"), "connect-src 'none'")
	assert.NotContains(t, resp.Header().Get("Content-Security-Policy"), "allow-same-origin")
	assert.Contains(t, resp.Header().Get("Content-Type"), "text/html")
}

func TestActionsArtifactPreviewArtifactTooLarge(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()
	oldMaxSize := setting.Actions.ArtifactPreviewMaxSize
	defer func() {
		setting.Actions.ArtifactPreviewMaxSize = oldMaxSize
	}()
	setting.Actions.ArtifactPreviewMaxSize = 1

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, "user2")

	req := NewRequestf(t, "GET", "/%s/actions/runs/791/artifacts/artifact-download/preview", repo.FullName())
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Body.String(), "This artifact is too large to preview")
	assert.NotContains(t, resp.Body.String(), "/preview/raw/abc.txt")

	req = NewRequestf(t, "GET", "/%s/actions/runs/791/artifacts/artifact-download/preview/raw/abc.txt", repo.FullName())
	session.MakeRequest(t, req, http.StatusRequestEntityTooLarge)
}

func TestActionsArtifactPreviewV4Zip(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	zipBytes := buildArtifactZip(t, map[string]string{
		"index.html":      "<html><body><h1>v4 zip</h1></body></html>",
		"style.css":       "body{color:red}",
		"sort.js":         "document.body.dataset.sorted = 'true';",
		"logs/output.txt": "v4 log output",
	})
	overwriteArtifactStorageContent(t, 22, zipBytes)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, "user2")

	// /preview lists files extracted from the zip's central directory.
	req := NewRequestf(t, "GET", "/%s/actions/runs/792/artifacts/artifact-v4-download/preview", repo.FullName())
	resp := session.MakeRequest(t, req, http.StatusOK)
	body := resp.Body.String()
	assert.Contains(t, body, "index.html")
	assert.Contains(t, body, "logs/output.txt")
	assert.Contains(t, body, "/preview/raw/index.html")
	assert.Contains(t, body, `target="_blank"`)
	assert.Contains(t, body, `rel="noopener noreferrer"`)
	assert.NotContains(t, body, "<iframe")

	req = NewRequestf(t, "GET", "/%s/actions/runs/792/artifacts/artifact-v4-download/preview/raw/index.html", repo.FullName())
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "<html><body><h1>v4 zip</h1></body></html>", resp.Body.String())
	assert.Contains(t, resp.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, resp.Header().Get("Content-Disposition"), "inline")
	assert.Contains(t, resp.Header().Get("Content-Security-Policy"), "sandbox")

	req = NewRequestf(t, "GET", "/%s/actions/runs/792/artifacts/artifact-v4-download/preview/raw/style.css", repo.FullName())
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "body{color:red}", resp.Body.String())
	assert.Contains(t, resp.Header().Get("Content-Type"), "text/css")

	req = NewRequestf(t, "GET", "/%s/actions/runs/792/artifacts/artifact-v4-download/preview/raw/sort.js", repo.FullName())
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "document.body.dataset.sorted = 'true';", resp.Body.String())
	assert.Contains(t, resp.Header().Get("Content-Type"), "javascript")

	req = NewRequestf(t, "GET", "/%s/actions/runs/792/artifacts/artifact-v4-download/preview/raw/logs/output.txt", repo.FullName())
	resp = session.MakeRequest(t, req, http.StatusOK)
	assert.Equal(t, "v4 log output", resp.Body.String())

	// Unknown path inside the zip returns 404 instead of falling back.
	req = NewRequestf(t, "GET", "/%s/actions/runs/792/artifacts/artifact-v4-download/preview/raw/missing.txt", repo.FullName())
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestActionsArtifactDownloadViewUnchanged(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	session := loginUser(t, "user2")
	req := NewRequestf(t, "GET", "/%s/actions/runs/791/artifacts/artifact-download", repo.FullName())
	resp := session.MakeRequest(t, req, http.StatusOK)
	assert.Contains(t, resp.Header().Get("Content-Disposition"), "attachment; filename=artifact-download.zip")
}
