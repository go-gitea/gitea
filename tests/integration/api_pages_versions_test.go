// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	pages_content "gitea.dev/modules/pages"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/require"
)

func TestAPIPagesVersions(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	contentStore := pages_content.NewContentStore()
	versions := make([]*pages_model.Version, 0, 2)
	for _, content := range [][]byte{[]byte("first Pages version"), []byte("second Pages version")} {
		digest := sha256.Sum256(content)
		version := &pages_model.Version{
			RepoID: 1, TarGzSHA256: hex.EncodeToString(digest[:]), OriginType: pages_model.VersionOriginUpload,
		}
		require.NoError(t, pages_model.InsertVersion(t.Context(), version))
		require.NoError(t, contentStore.Save(1, version.TarGzSHA256, bytes.NewReader(content), int64(len(content))))
		versions = append(versions, version)
	}
	require.NoError(t, pages_model.InsertSite(t.Context(), &pages_model.Site{
		RepoID: 1, Enabled: true, SourceType: pages_model.PublishSourceArtifact,
		CurrentVersionID: versions[1].ID, PagesUpdatedUnixNano: 1,
	}))

	readToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
	listURL := "/api/v1/repos/user2/repo1/pages/versions?limit=1"
	resp := MakeRequest(t, NewRequest(t, "GET", listURL).AddTokenAuth(readToken), http.StatusOK)
	require.Equal(t, "2", resp.Header().Get("X-Total-Count"))
	listed := *DecodeJSON(t, resp, &[]*api.PagesVersion{})
	require.Len(t, listed, 1)
	require.Equal(t, versions[1].ID, listed[0].ID)
	require.True(t, listed[0].Current)

	versionURL := "/api/v1/repos/user2/repo1/pages/versions/" + strconv.FormatInt(versions[1].ID, 10)
	resp = MakeRequest(t, NewRequest(t, "GET", versionURL).AddTokenAuth(readToken), http.StatusOK)
	version := DecodeJSON(t, resp, &api.PagesVersion{})
	require.Equal(t, versions[1].ID, version.ID)
	require.True(t, version.Current)
	userToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser)
	MakeRequest(t, NewRequest(t, "GET", versionURL).AddTokenAuth(userToken), http.StatusForbidden)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/pages/versions/0").AddTokenAuth(readToken), http.StatusNotFound)
	otherDigest := sha256.Sum256([]byte("another repository version"))
	otherVersion := &pages_model.Version{
		RepoID: 2, TarGzSHA256: hex.EncodeToString(otherDigest[:]), OriginType: pages_model.VersionOriginUpload,
	}
	require.NoError(t, pages_model.InsertVersion(t.Context(), otherVersion))
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/pages/versions/"+strconv.FormatInt(otherVersion.ID, 10)).AddTokenAuth(readToken), http.StatusNotFound)

	for _, repoURL := range []string{"/api/v1/repos/user2/repo1", "/api/v1/repositories/1"} {
		resp = MakeRequest(t, NewRequest(t, "GET", repoURL).AddTokenAuth(readToken), http.StatusOK)
		repository := DecodeJSON(t, resp, &api.Repository{})
		require.NotNil(t, repository.PagesCurrentVersion)
		require.Equal(t, versions[1].ID, repository.PagesCurrentVersion.ID)
		require.True(t, repository.PagesCurrentVersion.Current)
	}
	resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo2").AddTokenAuth(readToken), http.StatusOK)
	require.Nil(t, DecodeJSON(t, resp, &api.Repository{}).PagesCurrentVersion)
	listToken := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser, auth_model.AccessTokenScopeReadRepository)
	resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/user/repos").AddTokenAuth(listToken), http.StatusOK)
	repositories := *DecodeJSON(t, resp, &[]map[string]any{})
	for _, repository := range repositories {
		_, exists := repository["pages_current_version"]
		require.False(t, exists)
	}

	editURL := "/api/v1/repos/user2/repo1/pages/versions/" + strconv.FormatInt(versions[1].ID, 10)
	MakeRequest(t, NewRequestWithJSON(t, "PATCH", editURL, map[string]any{"comment": "release", "keep_forever": true}).AddTokenAuth(readToken), http.StatusForbidden)
	writeToken := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	resp = MakeRequest(t, NewRequestWithJSON(t, "PATCH", editURL, map[string]any{"comment": " release ", "keep_forever": true}).AddTokenAuth(writeToken), http.StatusOK)
	edited := DecodeJSON(t, resp, &api.PagesVersion{})
	require.Equal(t, "release", edited.Comment)
	require.True(t, edited.KeepForever)
	require.True(t, edited.Current)
	MakeRequest(t, NewRequestWithJSON(t, "PATCH", editURL, map[string]any{}).AddTokenAuth(writeToken), http.StatusBadRequest)
	MakeRequest(t, NewRequestWithJSON(t, "PATCH", editURL, map[string]any{"comment": "first\nsecond"}).AddTokenAuth(writeToken), http.StatusBadRequest)

	restoreURL := "/api/v1/repos/user2/repo1/pages/versions/" + strconv.FormatInt(versions[0].ID, 10) + "/restore"
	MakeRequest(t, NewRequest(t, "POST", restoreURL).AddTokenAuth(readToken), http.StatusForbidden)
	resp = MakeRequest(t, NewRequest(t, "POST", restoreURL).AddTokenAuth(writeToken), http.StatusOK)
	restored := DecodeJSON(t, resp, &api.PagesContentCommit{})
	require.Equal(t, versions[0].ID, restored.Version.ID)
	require.True(t, restored.Version.Current)

	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, versions[0].ID, site.CurrentVersionID)
	updated := site.PagesUpdatedUnixNano
	resp = MakeRequest(t, NewRequest(t, "POST", restoreURL).AddTokenAuth(writeToken), http.StatusOK)
	restored = DecodeJSON(t, resp, &api.PagesContentCommit{})
	require.Equal(t, updated, restored.PagesUpdatedUnixNano)
	site, has, err = pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, updated, site.PagesUpdatedUnixNano)

	resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1").AddTokenAuth(readToken), http.StatusOK)
	require.Equal(t, versions[0].ID, DecodeJSON(t, resp, &api.Repository{}).PagesCurrentVersion.ID)
	site.Enabled = false
	require.NoError(t, pages_model.UpdateSite(t.Context(), site))
	resp = MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1").AddTokenAuth(readToken), http.StatusOK)
	require.Nil(t, DecodeJSON(t, resp, &api.Repository{}).PagesCurrentVersion)
	site.Enabled = true
	require.NoError(t, pages_model.UpdateSite(t.Context(), site))

	deleteURL := "/api/v1/repos/user2/repo1/pages/versions/" + strconv.FormatInt(versions[1].ID, 10)
	MakeRequest(t, NewRequest(t, "DELETE", deleteURL).AddTokenAuth(readToken), http.StatusForbidden)
	MakeRequest(t, NewRequest(t, "DELETE", deleteURL).AddTokenAuth(writeToken), http.StatusNoContent)
	currentDeleteURL := "/api/v1/repos/user2/repo1/pages/versions/" + strconv.FormatInt(versions[0].ID, 10)
	MakeRequest(t, NewRequest(t, "DELETE", currentDeleteURL).AddTokenAuth(writeToken), http.StatusNoContent)
	count, err := db.GetEngine(t.Context()).Where("repo_id = ?", 1).Count(new(pages_model.Version))
	require.NoError(t, err)
	require.Zero(t, count)
	site, has, err = pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, site.Enabled)
	require.Zero(t, site.CurrentVersionID)
	require.Greater(t, site.PagesUpdatedUnixNano, int64(1))
}
