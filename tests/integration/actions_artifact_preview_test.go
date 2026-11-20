// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"
	actions_web "gitea.dev/routers/web/repo/actions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setArtifactFile(t *testing.T, artifactID int64, artifactPath string, content []byte) {
	artifact := unittest.AssertExistsAndLoadBean(t, &actions_model.ActionArtifact{ID: artifactID})
	_, err := storage.ActionsArtifacts.Save(artifact.StoragePath, bytes.NewReader(content), int64(len(content)))
	require.NoError(t, err)
	_, err = db.GetEngine(t.Context()).ID(artifactID).Cols("artifact_path", "file_size").Update(&actions_model.ActionArtifact{ArtifactPath: artifactPath, FileSize: int64(len(content))})
	require.NoError(t, err)
}

func TestActionsArtifactPreview(t *testing.T) {
	defer prepareTestEnvActionsArtifacts(t)()
	session := loginUser(t, "user2")

	t.Run("LegacyArtifact", func(t *testing.T) {
		resp := MakeRequest(t, NewRequest(t, "GET", "/user5/repo4/actions/artifacts/19/preview"), http.StatusSeeOther)
		assert.Contains(t, test.RedirectURL(resp), "/user/login")

		resp = session.MakeRequest(t, NewRequest(t, "POST", "/user5/repo4/actions/runs/791"), http.StatusOK)
		previewLinks := map[string]string{}
		for _, artifact := range DecodeJSON(t, resp, &actions_web.ViewResponse{}).Artifacts {
			previewLinks[artifact.Name] = artifact.PreviewLink
		}
		assert.Equal(t, "/user5/repo4/actions/artifacts/19/preview", previewLinks["multi-file-download"])

		resp = session.MakeRequest(t, NewRequest(t, "GET", "/user5/repo4/actions/artifacts/19/preview/missing.txt"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), "The requested file is not present in this artifact.")

		resp = session.MakeRequest(t, NewRequest(t, "GET", "/user5/repo4/actions/artifacts/19/preview/xyz/def.txt"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), `href="/user5/repo4/actions/runs/791"`)
		assert.Contains(t, resp.Body.String(), `href="/user5/repo4/actions/artifacts/19/preview/abc.txt"`)
		rawLink := NewHTMLParser(t, resp.Body).Find("iframe").AttrOr("data-src", "")
		assert.True(t, strings.HasPrefix(rawLink, "/-/actions/artifacts/19/"))

		resp = MakeRequest(t, NewRequest(t, "GET", rawLink), http.StatusOK)
		assert.Equal(t, strings.Repeat("C", 1024), resp.Body.String())
		assert.Equal(t, "text/plain; charset=utf-8", resp.Header().Get("Content-Type"))
		assert.Equal(t, "*", resp.Header().Get("Access-Control-Allow-Origin"))
		assert.Empty(t, resp.Header().Get("Set-Cookie"))

		MakeRequest(t, NewRequest(t, "GET", strings.TrimSuffix(rawLink, "xyz/def.txt")+"missing.txt"), http.StatusNotFound)
		MakeRequest(t, NewRequest(t, "GET", strings.Replace(rawLink, "/19/", "/1/", 1)), http.StatusNotFound)

		resp = MakeRequest(t, NewRequest(t, "GET", rawLink).SetHeader("Sec-Fetch-Dest", "document"), http.StatusSeeOther)
		assert.Equal(t, "/user5/repo4/actions/artifacts/19/preview/xyz/def.txt", test.RedirectURL(resp))

		defer timeutil.MockSet(time.Now().Add(2 * time.Hour))()
		MakeRequest(t, NewRequest(t, "GET", rawLink), http.StatusNotFound)
	})

	t.Run("ContentTypes", func(t *testing.T) {
		var rawLink string
		for _, file := range []struct {
			path, content, contentType, csp string
		}{
			{"report.pdf", "%PDF-1.7\n", "application/pdf", "default-src 'none'; style-src 'unsafe-inline'"},
			{"image.png", "\x89PNG\r\n\x1a\n\x00\x00\x00\x0d", "image/png", "sandbox"},
			{"index.html", "<!DOCTYPE html><html>artifact</html>", "text/html; charset=utf-8", "sandbox"},
		} {
			setArtifactFile(t, 1, file.path, []byte(file.content))
			resp := session.MakeRequest(t, NewRequest(t, "GET", "/user5/repo4/actions/artifacts/1/preview/"+file.path), http.StatusOK)
			rawLink = NewHTMLParser(t, resp.Body).Find("iframe").AttrOr("data-src", "")
			resp = MakeRequest(t, NewRequest(t, "GET", rawLink), http.StatusOK)
			assert.Equal(t, file.contentType, resp.Header().Get("Content-Type"))
			assert.Contains(t, resp.Header().Get("Content-Security-Policy"), file.csp) // CSP is from httplib package, we don't need to test the exact value here
		}

		resp := MakeRequest(t, NewRequest(t, "GET", rawLink), http.StatusOK)
		assert.Regexp(t, `^<!DOCTYPE html><html><head><script crossorigin src="[^"]+/external-render-helper[^"]*"></script></head>artifact</html>$`, resp.Body.String())
	})

	t.Run("V4Zip", func(t *testing.T) {
		setArtifactFile(t, 22, "artifact-v4-download.zip", test.WriteZipArchive(map[string]string{"index.html": "<html>v4</html>", "css/style.css": "body{}"}).Bytes())

		resp := session.MakeRequest(t, NewRequest(t, "GET", "/user5/repo4/actions/artifacts/22/preview/index.html"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), `href="/user5/repo4/actions/artifacts/22/preview/css/style.css"`)
		rawLink := NewHTMLParser(t, resp.Body).Find("iframe").AttrOr("data-src", "")

		resp = MakeRequest(t, NewRequest(t, "GET", strings.TrimSuffix(rawLink, "index.html")+"css/style.css"), http.StatusOK)
		assert.Equal(t, "body{}", resp.Body.String())
		assert.Equal(t, "text/css; charset=utf-8", resp.Header().Get("Content-Type"))
	})

	t.Run("Limits", func(t *testing.T) {
		resp := session.MakeRequest(t, NewRequest(t, "GET", "/user5/repo4/actions/artifacts/19/preview/abc.txt"), http.StatusOK)
		rawLink := NewHTMLParser(t, resp.Body).Find("iframe").AttrOr("data-src", "")

		restore := test.MockVariableValue(&setting.UI.MaxDisplayFileSize, 16)
		MakeRequest(t, NewRequest(t, "GET", rawLink), http.StatusRequestEntityTooLarge)
		restore()

		defer test.MockVariableValue(&setting.Actions.ArtifactPreviewMaxSize, 1)()
		resp = session.MakeRequest(t, NewRequest(t, "GET", "/user5/repo4/actions/artifacts/19/preview/abc.txt"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), "This artifact is too large to preview.")
		assert.NotContains(t, resp.Body.String(), "The requested file is not present")
		MakeRequest(t, NewRequest(t, "GET", rawLink), http.StatusRequestEntityTooLarge)

		setting.Actions.ArtifactPreviewMaxSize = 0
		session.MakeRequest(t, NewRequest(t, "GET", "/user5/repo4/actions/artifacts/19/preview"), http.StatusNotFound)
		MakeRequest(t, NewRequest(t, "GET", rawLink), http.StatusNotFound)
		resp = session.MakeRequest(t, NewRequest(t, "POST", "/user5/repo4/actions/runs/791"), http.StatusOK)
		for _, artifact := range DecodeJSON(t, resp, &actions_web.ViewResponse{}).Artifacts {
			assert.Empty(t, artifact.PreviewLink)
		}
	})

	t.Run("Attempt", func(t *testing.T) {
		attempt := &actions_model.ActionRunAttempt{RepoID: 4, RunID: 791, Attempt: 2, TriggerUserID: 1, Status: actions_model.StatusSuccess}
		require.NoError(t, db.Insert(t.Context(), attempt))
		_, err := db.GetEngine(t.Context()).In("id", 19, 20).Cols("run_attempt_id").Update(&actions_model.ActionArtifact{RunAttemptID: attempt.ID})
		require.NoError(t, err)

		resp := session.MakeRequest(t, NewRequest(t, "GET", "/user5/repo4/actions/artifacts/19/preview"), http.StatusOK)
		assert.Contains(t, resp.Body.String(), `href="/user5/repo4/actions/runs/791/attempts/2"`)
		assert.Contains(t, resp.Body.String(), `href="/user5/repo4/actions/runs/791/artifacts/multi-file-download?attempt=2"`)
	})
}
