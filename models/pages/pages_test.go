// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages_test

import (
	"testing"

	pages_model "gitea.dev/models/pages"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestPagesModels(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	site := &pages_model.Site{
		RepoID:               101,
		Enabled:              true,
		SourceType:           pages_model.PublishSourceArtifact,
		PagesUpdatedUnixNano: 10,
	}
	require.NoError(t, pages_model.InsertSite(t.Context(), site))
	require.Equal(t, pages_model.DefaultVersionRetentionCount, site.VersionRetentionCount)
	version := &pages_model.Version{
		RepoID: site.RepoID, TarGzSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		OriginType: pages_model.VersionOriginUpload,
	}
	require.NoError(t, pages_model.InsertVersion(t.Context(), version))
	site.CurrentVersionID = version.ID
	require.NoError(t, pages_model.UpdateSite(t.Context(), site))
	loaded, has, err := pages_model.GetSiteByRepoID(t.Context(), site.RepoID)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, version.ID, loaded.CurrentVersionID)
	current, has, err := pages_model.GetCurrentVersion(t.Context(), loaded)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, version.TarGzSHA256, current.TarGzSHA256)
	versions, total, err := pages_model.FindVersions(t.Context(), &pages_model.FindVersionOptions{RepoID: site.RepoID})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, version.ID, versions[0].ID)
	require.Error(t, pages_model.InsertVersion(t.Context(), &pages_model.Version{RepoID: site.RepoID, TarGzSHA256: version.TarGzSHA256, OriginType: pages_model.VersionOriginBranch}))

	site.Deleted = true
	site.Enabled = false
	site.SourceType = ""
	site.CurrentVersionID = 0
	site.PagesUpdatedUnixNano = 11
	require.NoError(t, pages_model.UpdateSite(t.Context(), site))
	loaded, has, err = pages_model.GetSiteByRepoID(t.Context(), site.RepoID)
	require.NoError(t, err)
	require.True(t, has)
	require.True(t, loaded.Deleted)
	require.Zero(t, loaded.CurrentVersionID)

	instance := &pages_model.Instance{
		RootURL:             "https://pages.example.test",
		RootUpdatedUnixNano: 20,
		TokenHash:           "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	require.NoError(t, pages_model.InsertInstance(t.Context(), instance))
	require.NotZero(t, instance.ID)

	domain := &pages_model.Domain{
		RepoID:                 site.RepoID,
		Host:                   "docs.example.test",
		TXTToken:               "token",
		State:                  pages_model.DomainStateActive,
		TXTResult:              pages_model.TXTResultValid,
		TXTLastSuccessUnixNano: 30,
		TXTLastCheckUnix:       30,
		CreatedUnixNano:        30,
	}
	require.NoError(t, pages_model.InsertDomain(t.Context(), domain))
	domains, err := pages_model.ListDomainsByRepoID(t.Context(), site.RepoID)
	require.NoError(t, err)
	require.Len(t, domains, 1)

	result := &pages_model.InstanceResult{
		InstanceID:                  instance.ID,
		RepoID:                      site.RepoID,
		AppliedPagesUpdatedUnixNano: 11,
		AppliedRootUpdatedUnixNano:  20,
		ConfigApplied:               true,
		DefaultPublished:            true,
	}
	require.NoError(t, pages_model.InsertInstanceResult(t.Context(), result))
	result.ConfigApplied = false
	result.ErrorCode = pages_model.InstanceErrorContentInstallFailed
	require.NoError(t, pages_model.UpdateInstanceResult(t.Context(), result))
	loadedResult, has, err := pages_model.GetInstanceResult(t.Context(), instance.ID, site.RepoID)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, loadedResult.ConfigApplied)
	require.Equal(t, pages_model.InstanceErrorContentInstallFailed, loadedResult.ErrorCode)
}
