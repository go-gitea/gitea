// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/zip"
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"testing"

	render_model "code.gitea.io/gitea/models/render"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/renderplugin"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/tests"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderPluginLifecycle(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	require.NoError(t, storage.Clean(renderplugin.Storage()))
	t.Cleanup(func() {
		_ = storage.Clean(renderplugin.Storage())
	})

	const pluginID = "itest-plugin"

	session := loginUser(t, "user1")

	uploadArchive(t, session, "/-/admin/render-plugins/upload", buildRenderPluginArchive(t, pluginID, "Integration Plugin", "1.0.0"))
	flash := expectFlashSuccess(t, session)
	assert.Contains(t, flash.SuccessMsg, "installed")
	row := requireRenderPluginRow(t, session, pluginID)
	assert.Equal(t, "1.0.0", row.Version)
	assert.False(t, row.Enabled)

	postPluginAction(t, session, fmt.Sprintf("/-/admin/render-plugins/%d/enable", row.ID))
	flash = expectFlashSuccess(t, session)
	assert.Contains(t, flash.SuccessMsg, "enabled")
	row = requireRenderPluginRow(t, session, pluginID)
	assert.True(t, row.Enabled)

	metas := fetchRenderPluginMetadata(t)
	require.Len(t, metas, 1)
	assert.Equal(t, pluginID, metas[0].ID)
	assert.Contains(t, metas[0].EntryURL, "render.js")
	MakeRequest(t, NewRequest(t, "GET", metas[0].EntryURL), http.StatusOK)

	uploadArchive(t, session, fmt.Sprintf("/-/admin/render-plugins/%d/upgrade", row.ID), buildRenderPluginArchive(t, pluginID, "Integration Plugin", "2.0.0"))
	flash = expectFlashSuccess(t, session)
	assert.Contains(t, flash.SuccessMsg, "upgraded")
	row = requireRenderPluginRow(t, session, pluginID)
	assert.Equal(t, "2.0.0", row.Version)

	postPluginAction(t, session, fmt.Sprintf("/-/admin/render-plugins/%d/disable", row.ID))
	flash = expectFlashSuccess(t, session)
	assert.Contains(t, flash.SuccessMsg, "disabled")
	row = requireRenderPluginRow(t, session, pluginID)
	assert.False(t, row.Enabled)
	require.Empty(t, fetchRenderPluginMetadata(t))

	postPluginAction(t, session, fmt.Sprintf("/-/admin/render-plugins/%d/delete", row.ID))
	flash = expectFlashSuccess(t, session)
	assert.Contains(t, flash.SuccessMsg, "deleted")
	unittest.AssertNotExistsBean(t, &render_model.Plugin{Identifier: pluginID})
	_, err := renderplugin.Storage().Stat(renderplugin.ObjectPath(pluginID, "render.js"))
	assert.Error(t, err)
	require.Nil(t, findRenderPluginRow(t, session, pluginID))
}

func postPluginAction(t *testing.T, session *TestSession, path string) {
	req := NewRequestWithValues(t, "POST", path, map[string]string{
		"_csrf": GetUserCSRFToken(t, session),
	})
	session.MakeRequest(t, req, http.StatusSeeOther)
}

func uploadArchive(t *testing.T, session *TestSession, path string, archive []byte) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("_csrf", GetUserCSRFToken(t, session)))
	part, err := writer.CreateFormFile("plugin", "plugin.zip")
	require.NoError(t, err)
	_, err = part.Write(archive)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := NewRequestWithBody(t, "POST", path, bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp := session.MakeRequest(t, req, http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	token := doc.GetInputValueByName("token")
	require.NotEmpty(t, token, "pending upload token not found")
	confirmReq := NewRequestWithValues(t, "POST", path+"/confirm", map[string]string{
		"_csrf": GetUserCSRFToken(t, session),
		"token": token,
	})
	session.MakeRequest(t, confirmReq, http.StatusSeeOther)
}

func buildRenderPluginArchive(t *testing.T, id, name, version string) []byte {
	manifest := fmt.Sprintf(`{
	"schemaVersion": 1,
	"id": %q,
	"name": %q,
	"version": %q,
	"description": "integration test plugin",
	"entry": "render.js",
	"filePatterns": ["*.itest"]
}`, id, name, version)

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	file, err := zipWriter.Create("manifest.json")
	require.NoError(t, err)
	_, err = file.Write([]byte(manifest))
	require.NoError(t, err)

	file, err = zipWriter.Create("render.js")
	require.NoError(t, err)
	_, err = file.Write([]byte("export default {render(){}};"))
	require.NoError(t, err)
	require.NoError(t, zipWriter.Close())
	return buf.Bytes()
}

func fetchRenderPluginMetadata(t *testing.T) []renderplugin.Metadata {
	resp := MakeRequest(t, NewRequest(t, "GET", "/assets/render-plugins/index.json"), http.StatusOK)
	var metas []renderplugin.Metadata
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &metas))
	return metas
}

func expectFlashSuccess(t *testing.T, session *TestSession) *middleware.Flash {
	flash := session.GetCookieFlashMessage()
	require.NotNil(t, flash, "expected flash message")
	require.Empty(t, flash.ErrorMsg)
	return flash
}

type renderPluginRow struct {
	ID         int64
	Identifier string
	Version    string
	Enabled    bool
}

func requireRenderPluginRow(t *testing.T, session *TestSession, identifier string) *renderPluginRow {
	row := findRenderPluginRow(t, session, identifier)
	require.NotNil(t, row, "plugin %s not found", identifier)
	return row
}

func findRenderPluginRow(t *testing.T, session *TestSession, identifier string) *renderPluginRow {
	resp := session.MakeRequest(t, NewRequest(t, "GET", "/-/admin/render-plugins"), http.StatusOK)
	doc := NewHTMLParser(t, resp.Body)
	var result *renderPluginRow
	doc.Find("table tbody tr").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		cols := s.Find("td")
		if cols.Length() < 6 {
			return true
		}
		idText := strings.TrimSpace(cols.Eq(1).Text())
		if idText != identifier {
			return true
		}
		link := cols.Eq(5).Find("a[href]").First()
		href, _ := link.Attr("href")
		id, err := strconv.ParseInt(path.Base(href), 10, 64)
		if err != nil {
			return true
		}
		version := strings.TrimSpace(cols.Eq(2).Text())
		enabled := cols.Eq(4).Find(".ui.green").Length() > 0
		result = &renderPluginRow{
			ID:         id,
			Identifier: idText,
			Version:    version,
			Enabled:    enabled,
		}
		return false
	})
	return result
}
