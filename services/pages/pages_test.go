// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages_test

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"
	pages_service "gitea.dev/services/pages"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestPagesContentCommitPreservesSource(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	_, err := pages_service.CommitUploadContent(t.Context(), 1, bytes.NewBufferString("not gzip"))
	require.ErrorIs(t, err, pages_content.ErrInvalidArchive)
	_, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.False(t, has)

	pack := func(content string) []byte {
		t.Helper()
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte(content), 0o600))
		var artifact bytes.Buffer
		require.NoError(t, pages_content.PackDirectory(t.Context(), root, &artifact, pages_content.DefaultLimits()))
		return artifact.Bytes()
	}
	firstArchive := pack("first site")
	first, err := pages_service.CommitUploadContent(t.Context(), 1, bytes.NewReader(firstArchive))
	require.NoError(t, err)
	require.NotEmpty(t, first.Version.TarGzSHA256)

	again, err := pages_service.CommitUploadContent(t.Context(), 1, bytes.NewReader(firstArchive))
	require.NoError(t, err)
	require.Equal(t, first, again, "a repeated content digest should not create an update")
	_, err = pages_service.CommitUploadContent(t.Context(), 1, bytes.NewBufferString("not gzip"))
	require.ErrorIs(t, err, pages_content.ErrInvalidArchive)
	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, site.Enabled, "an initial upload selects the source but keeps the site closed")
	require.Equal(t, first.Version.ID, site.CurrentVersionID)

	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{
		Enabled: true, SourceType: pages_model.PublishSourceBranch, SourceBranch: "master", SourcePath: "docs",
	})
	require.NoError(t, err)
	site, _, err = pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	site.SourceCommitID = strings.Repeat("a", 40)
	site.SourceNextCheckUnix = 123
	require.NoError(t, pages_model.UpdateSite(t.Context(), site))

	uploaded, err := pages_service.CommitUploadContent(t.Context(), 1, bytes.NewReader(pack("manual version")))
	require.NoError(t, err)
	require.Equal(t, pages_model.VersionOriginUpload, uploaded.Version.OriginType)
	site, has, err = pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, uploaded.Version.ID, site.CurrentVersionID)
	require.Equal(t, pages_model.PublishSourceBranch, site.SourceType)
	require.Equal(t, "master", site.SourceBranch)
	require.Equal(t, "docs", site.SourcePath)
	require.Equal(t, strings.Repeat("a", 40), site.SourceCommitID)
	require.Equal(t, int64(123), site.SourceNextCheckUnix)
}

func TestDeletePagesVersion(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	commit := func(content string) *pages_service.ContentCommitResult {
		t.Helper()
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte(content), 0o600))
		var archive bytes.Buffer
		require.NoError(t, pages_content.PackDirectory(t.Context(), root, &archive, pages_content.DefaultLimits()))
		result, err := pages_service.CommitUploadContent(t.Context(), 1, bytes.NewReader(archive.Bytes()))
		require.NoError(t, err)
		return result
	}
	first := commit("first")
	current := commit("current")

	require.ErrorIs(t, pages_service.DeleteVersion(t.Context(), 1, current.Version.ID), pages_service.ErrCurrentVersion)
	_, has, err := pages_model.GetVersionByID(t.Context(), current.Version.ID)
	require.NoError(t, err)
	require.True(t, has)

	require.NoError(t, pages_service.DeleteVersion(t.Context(), 1, first.Version.ID))
	_, has, err = pages_model.GetVersionByID(t.Context(), first.Version.ID)
	require.NoError(t, err)
	require.False(t, has)
	_, err = pages_content.NewContentStore().Stat(1, first.Version.TarGzSHA256)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.ErrorIs(t, pages_service.DeleteVersion(t.Context(), 1, first.Version.ID), util.ErrNotExist)
}

func TestPagesVersionRetentionAndRestore(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Pages.MaxVersionsPerSite, 2)()

	commit := func(content string) *pages_service.ContentCommitResult {
		t.Helper()
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte(content), 0o600))
		var artifact bytes.Buffer
		require.NoError(t, pages_content.PackDirectory(t.Context(), root, &artifact, pages_content.DefaultLimits()))
		result, err := pages_service.CommitUploadContent(t.Context(), 1, bytes.NewReader(artifact.Bytes()))
		require.NoError(t, err)
		return result
	}

	first := commit("first")
	firstTime := first.PagesUpdatedUnixNano
	repeated := commit("first")
	require.Equal(t, first.Version.ID, repeated.Version.ID)
	require.Equal(t, firstTime, repeated.PagesUpdatedUnixNano)
	second := commit("second")
	third := commit("third")
	require.NotEqual(t, second.Version.ID, third.Version.ID)

	versions, total, err := pages_model.FindVersions(t.Context(), &pages_model.FindVersionOptions{ListOptions: db.ListOptionsAll, RepoID: 1})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, versions, 2)
	_, err = pages_content.NewContentStore().Stat(1, first.Version.TarGzSHA256)
	require.ErrorIs(t, err, os.ErrNotExist)

	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{Enabled: false, SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	restored, err := pages_service.RestoreVersion(t.Context(), 1, second.Version.ID)
	require.NoError(t, err)
	require.Equal(t, second.Version.ID, restored.Version.ID)
	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, site.Enabled)
	require.Equal(t, pages_model.PublishSourceArtifact, site.SourceType)
	require.Equal(t, second.Version.ID, site.CurrentVersionID)

	restoredAgain, err := pages_service.RestoreVersion(t.Context(), 1, second.Version.ID)
	require.NoError(t, err)
	require.Equal(t, restored.PagesUpdatedUnixNano, restoredAgain.PagesUpdatedUnixNano)
	require.ErrorIs(t, func() error {
		other := &pages_model.Version{RepoID: 2, TarGzSHA256: strings.Repeat("f", 64), OriginType: pages_model.VersionOriginUpload}
		require.NoError(t, pages_model.InsertVersion(t.Context(), other))
		_, err := pages_service.RestoreVersion(t.Context(), 1, other.ID)
		return err
	}(), util.ErrNotExist)
	require.NoError(t, pages_content.NewContentStore().Delete(1, third.Version.TarGzSHA256))
	_, err = pages_service.RestoreVersion(t.Context(), 1, third.Version.ID)
	require.Error(t, err)
	_, err = db.GetEngine(t.Context()).ID(1).Cols("is_archived").Update(&repo_model.Repository{IsArchived: true})
	require.NoError(t, err)
	_, err = pages_service.RestoreVersion(t.Context(), 1, second.Version.ID)
	require.ErrorContains(t, err, "archived")
}

func TestPagesPrivateRepositoryCanExplicitlyPublishPublicSite(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site, err := pages_service.SaveSiteSettings(t.Context(), 2, pages_service.SiteSettings{Enabled: true, SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	require.True(t, site.Enabled)
	stored, has, err := pages_model.GetSiteByRepoID(t.Context(), 2)
	require.NoError(t, err)
	require.True(t, has)
	require.True(t, stored.Enabled)
}

func TestPagesInstanceRegistrationAndAuthentication(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	instance, token, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	require.Len(t, token, 43)
	require.Len(t, instance.TokenHash, 64)

	authenticated, err := pages_service.AuthenticateInstance(t.Context(), "bEaReR\t"+token)
	require.NoError(t, err)
	require.Equal(t, instance.ID, authenticated.ID)
	_, err = pages_service.AuthenticateInstance(t.Context(), "Bearer wrong")
	require.ErrorIs(t, err, util.ErrNotExist)
	_, err = pages_service.AuthenticateInstance(t.Context(), "Bearer "+token+" extra")
	require.ErrorIs(t, err, util.ErrNotExist)

	registered, err := pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, "https://pages.example.test")
	require.NoError(t, err)
	require.NotZero(t, registered.RootUpdatedUnixNano)
	originalRootTime := registered.RootUpdatedUnixNano
	same, err := pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, "https://pages.example.test")
	require.NoError(t, err)
	require.Equal(t, registered.RootUpdatedUnixNano, same.RootUpdatedUnixNano)

	second, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	_, err = pages_service.RegisterInstanceRootURL(t.Context(), second.ID, second.TokenHash, "https://sub.pages.example.test")
	require.ErrorIs(t, err, pages_service.ErrDomainConflict)

	require.NoError(t, pages_model.InsertInstanceResult(t.Context(), &pages_model.InstanceResult{
		InstanceID: instance.ID, RepoID: 1, AppliedRootUpdatedUnixNano: originalRootTime,
	}))
	nextToken, err := pages_service.ResetInstanceCredential(t.Context(), instance.ID)
	require.NoError(t, err)
	require.NotEqual(t, token, nextToken)
	_, err = pages_service.AuthenticateInstance(t.Context(), "Bearer "+token)
	require.ErrorIs(t, err, util.ErrNotExist)
	resetInstance, err := pages_service.AuthenticateInstance(t.Context(), "Bearer "+nextToken)
	require.NoError(t, err)
	require.Greater(t, resetInstance.RootUpdatedUnixNano, originalRootTime)
	require.Zero(t, resetInstance.LastCommunicationUnix)
	authenticated.LastCommunicationUnix = 0
	require.NoError(t, pages_service.RecordInstanceCommunication(t.Context(), authenticated))
	resetInstance, has, err := pages_model.GetInstanceByID(t.Context(), instance.ID)
	require.NoError(t, err)
	require.True(t, has)
	require.Zero(t, resetInstance.LastCommunicationUnix, "an in-flight request with the old credential must not update the reset instance")
	_, has, err = pages_model.GetInstanceResult(t.Context(), instance.ID, 1)
	require.NoError(t, err)
	require.False(t, has)
	reregistered, err := pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, resetInstance.TokenHash, "https://pages.example.test")
	require.NoError(t, err)
	require.Equal(t, resetInstance.RootUpdatedUnixNano, reregistered.RootUpdatedUnixNano)
}

func TestPagesReservesManagementDNSNamespace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.AppDomain, "Gitea.Example.Test")()

	instance, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	for _, rootURL := range []string{
		"https://gitea.example.test",
		"https://example.test",
		"https://pages.gitea.example.test",
	} {
		_, err := pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, rootURL)
		require.ErrorIs(t, err, pages_service.ErrDomainConflict, rootURL)
	}

	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	for _, host := range []string{"gitea.example.test", "example.test", "docs.gitea.example.test"} {
		_, err := pages_service.AddDomain(t.Context(), 1, host)
		require.ErrorIs(t, err, pages_service.ErrDomainConflict, host)
	}
}

func TestPagesDefaultSiteURL(t *testing.T) {
	for _, test := range []struct{ owner, repo, expected string }{
		{"alice", ".pages", "https://alice.pages.example.test/"},
		{"alice", "docs", "https://alice.pages.example.test/docs/"},
		{"team.docs", "docs", "https://x-25q6w3iheo.pages.example.test/docs/"},
		{"team_docs", "docs", "https://x-25q6w51cps.pages.example.test/docs/"},
		{"x-pages", "docs", "https://x-1xpbv8j4.pages.example.test/docs/"},
	} {
		actual, err := pages_service.DefaultSiteURL("https://pages.example.test", test.owner, test.repo)
		require.NoError(t, err)
		require.Equal(t, test.expected, actual)
	}
}

func TestPagesFullPaginationCatchesConcurrentUpdate(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	instance, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	instance, err = pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, "http://pages.example.test")
	require.NoError(t, err)
	for _, repoID := range []int64{1, 2} {
		_, err := pages_service.SaveSiteSettings(t.Context(), repoID, pages_service.SiteSettings{SourceType: pages_model.PublishSourceArtifact})
		require.NoError(t, err)
	}
	first, err := pages_service.ListSitesForSync(t.Context(), instance, pages_service.SyncOptions{Full: true, Limit: 1})
	require.NoError(t, err)
	require.Equal(t, setting.Pages.MaxTarGzSize, first.ArchiveLimits.MaxTarGzBytes)
	require.Equal(t, setting.Pages.MaxSiteSize, first.ArchiveLimits.MaxSiteBytes)
	require.Equal(t, setting.Pages.MaxFileSize, first.ArchiveLimits.MaxFileBytes)
	require.Equal(t, setting.Pages.MaxEntries, first.ArchiveLimits.MaxEntries)
	require.Equal(t, setting.Pages.RequireGiteaTXTRecord, first.RequireGiteaTXTRecord)
	require.Len(t, first.Sites, 1)
	require.NotNil(t, first.NextCursor)
	require.NotEmpty(t, first.Sites[0].Owner)

	secondSite := int64(2)
	if first.Sites[0].RepoID == 2 {
		secondSite = 1
	}
	_, err = pages_service.SaveSiteSettings(t.Context(), secondSite, pages_service.SiteSettings{SourceType: pages_model.PublishSourceBranch, SourceBranch: "main"})
	require.NoError(t, err)
	second, err := pages_service.ListSitesForSync(t.Context(), instance, pages_service.SyncOptions{
		Full: true, Limit: 1, Until: first.Until,
		AfterTime: first.NextCursor.PagesUpdatedUnixNano, AfterRepo: first.NextCursor.RepoID,
	})
	require.NoError(t, err)
	require.Empty(t, second.Sites)
	updates, err := pages_service.ListSitesForSync(t.Context(), instance, pages_service.SyncOptions{
		Full: false, Since: first.Until, Limit: 100,
	})
	require.NoError(t, err)
	require.Len(t, updates.Sites, 1)
	require.Equal(t, secondSite, updates.Sites[0].RepoID)
	require.Greater(t, updates.Until, first.Until)
}

func TestPagesSyncRejectsMultipleActiveDomains(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	instance, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	instance, err = pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, "http://pages.example.test")
	require.NoError(t, err)
	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	for id, host := range []string{"first.example.test", "second.example.test"} {
		require.NoError(t, pages_model.InsertDomain(t.Context(), &pages_model.Domain{
			RepoID: 1, Host: host, TXTToken: host, State: pages_model.DomainStateActive,
			TXTResult: pages_model.TXTResultValid, CreatedUnixNano: int64(id + 1),
		}))
	}
	_, err = pages_service.ListSitesForSync(t.Context(), instance, pages_service.SyncOptions{Full: true, Limit: 100})
	require.ErrorContains(t, err, "multiple active domains")
}

func TestPagesSyncFiltersSettledDeletionsOnlyFromFullListing(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	instance, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	instance, err = pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, "http://pages.example.test")
	require.NoError(t, err)
	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	base := time.Now().UnixNano()
	require.NoError(t, pages_model.InsertSite(t.Context(), &pages_model.Site{RepoID: 9998, Deleted: true, PagesUpdatedUnixNano: base + 1}))
	require.NoError(t, pages_model.InsertSite(t.Context(), &pages_model.Site{RepoID: 9999, Deleted: true, PagesUpdatedUnixNano: base + 2}))
	require.NoError(t, pages_model.InsertDomain(t.Context(), &pages_model.Domain{
		RepoID: 9998, Host: "old.example.test", TXTToken: "unused", State: pages_model.DomainStateReleasing, TXTResult: pages_model.TXTResultInvalid, CreatedUnixNano: base,
	}))
	full, err := pages_service.ListSitesForSync(t.Context(), instance, pages_service.SyncOptions{Full: true, Limit: 10})
	require.NoError(t, err)
	require.Len(t, full.Sites, 2)
	require.Equal(t, int64(1), full.Sites[0].RepoID)
	require.Equal(t, int64(9998), full.Sites[1].RepoID)
	require.True(t, full.Sites[1].Deleted)
	require.Empty(t, full.Sites[1].Owner)

	updates, err := pages_service.ListSitesForSync(t.Context(), instance, pages_service.SyncOptions{Since: base, Limit: 10})
	require.NoError(t, err)
	require.Len(t, updates.Sites, 2)
	require.Equal(t, int64(9998), updates.Sites[0].RepoID)
	require.Equal(t, int64(9999), updates.Sites[1].RepoID, "an existing instance must receive a default-route deletion")
}

func TestPagesLateResultCannotReplaceCurrentResult(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	instance, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	instance, err = pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, "http://pages.example.test")
	require.NoError(t, err)
	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	result := pages_service.ResultRequest{
		RepoID: 1, AppliedPagesUpdatedUnixNano: site.PagesUpdatedUnixNano,
		AppliedRootUpdatedUnixNano: instance.RootUpdatedUnixNano,
		ConfigApplied:              true, DefaultPublished: false,
	}
	require.NoError(t, pages_service.AcceptInstanceResult(t.Context(), instance.ID, instance.TokenHash, result))
	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{SourceType: pages_model.PublishSourceBranch, SourceBranch: "main"})
	require.NoError(t, err)
	result.DefaultPublished = true
	require.ErrorIs(t, pages_service.AcceptInstanceResult(t.Context(), instance.ID, instance.TokenHash, result), pages_service.ErrStateChanged)
	stored, has, err := pages_model.GetInstanceResult(t.Context(), instance.ID, 1)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, stored.DefaultPublished)
	require.Equal(t, site.PagesUpdatedUnixNano, stored.AppliedPagesUpdatedUnixNano)
}

func TestPagesResultRejectsPublishedDifferentContent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	instance, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	instance, err = pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, "http://pages.example.test")
	require.NoError(t, err)
	site, err := pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{Enabled: true, SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	version := &pages_model.Version{RepoID: site.RepoID, TarGzSHA256: strings.Repeat("a", 64), OriginType: pages_model.VersionOriginUpload}
	require.NoError(t, pages_model.InsertVersion(t.Context(), version))
	site.CurrentVersionID = version.ID
	require.NoError(t, pages_model.UpdateSite(t.Context(), site))

	err = pages_service.AcceptInstanceResult(t.Context(), instance.ID, instance.TokenHash, pages_service.ResultRequest{
		RepoID: 1, AppliedPagesUpdatedUnixNano: site.PagesUpdatedUnixNano,
		AppliedRootUpdatedUnixNano: instance.RootUpdatedUnixNano,
		ConfigApplied:              true, ActualTarGzSHA256: strings.Repeat("b", 64), DefaultPublished: true,
	})
	require.ErrorIs(t, err, pages_service.ErrInvalidResult)
	_, has, err := pages_model.GetInstanceResult(t.Context(), instance.ID, 1)
	require.NoError(t, err)
	require.False(t, has)
}

func TestPagesDomainReleaseWaitsForCurrentInstanceConfiguration(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	oldRequirement := setting.Pages.RequireGiteaTXTRecord
	setting.Pages.RequireGiteaTXTRecord = false
	t.Cleanup(func() { setting.Pages.RequireGiteaTXTRecord = oldRequirement })
	instance, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	instance, err = pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, "http://pages.example.test")
	require.NoError(t, err)
	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{Enabled: true, SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	domain, err := pages_service.AddDomain(t.Context(), 1, "site.example.test")
	require.NoError(t, err)
	require.Equal(t, pages_model.DomainStateActive, domain.State)
	require.NoError(t, pages_service.RemoveDomain(t.Context(), 1, domain.ID))
	stillReserved, has, err := pages_model.GetDomainByHost(t.Context(), "site.example.test")
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, pages_model.DomainStateReleasing, stillReserved.State)

	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	result := pages_service.ResultRequest{
		RepoID: 1, AppliedPagesUpdatedUnixNano: site.PagesUpdatedUnixNano,
		AppliedRootUpdatedUnixNano: instance.RootUpdatedUnixNano, AppliedRequireGiteaTXTRecord: true, ConfigApplied: true,
	}
	require.ErrorIs(t, pages_service.AcceptInstanceResult(t.Context(), instance.ID, instance.TokenHash, result), pages_service.ErrStateChanged)
	result.AppliedRequireGiteaTXTRecord = false
	require.NoError(t, pages_service.AcceptInstanceResult(t.Context(), instance.ID, instance.TokenHash, result))
	_, has, err = pages_model.GetDomainByHost(t.Context(), "site.example.test")
	require.NoError(t, err)
	require.False(t, has)
}

func TestPagesTXTDisabledActivatesPendingDomain(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	oldRequirement := setting.Pages.RequireGiteaTXTRecord
	setting.Pages.RequireGiteaTXTRecord = true
	t.Cleanup(func() { setting.Pages.RequireGiteaTXTRecord = oldRequirement })
	_, err := pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{Enabled: true, SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	domain, err := pages_service.AddDomain(t.Context(), 1, "pending.example.test")
	require.NoError(t, err)
	require.Equal(t, pages_model.DomainStatePending, domain.State)

	setting.Pages.RequireGiteaTXTRecord = false
	require.NoError(t, pages_service.VerifyDueDomains(t.Context(), 1))
	domain, has, err := pages_model.GetDomainByID(t.Context(), domain.ID)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, pages_model.DomainStateActive, domain.State)
}

func TestPagesBranchSourcePreparesCurrentContent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	_, err := pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{
		SourceType: pages_model.PublishSourceBranch, SourceBranch: "master",
	})
	require.NoError(t, err)
	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{
		Enabled: true, SourceType: pages_model.PublishSourceBranch, SourceBranch: "master",
	})
	require.NoError(t, err)
	require.NoError(t, pages_service.UpdateDueBranchSources(t.Context(), 10))
	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.Empty(t, site.SourceErrorCode)
	version, has, err := pages_model.GetCurrentVersion(t.Context(), site)
	require.NoError(t, err)
	require.True(t, has)
	object, size, err := pages_service.OpenCurrentContent(t.Context(), 1, version.TarGzSHA256)
	require.NoError(t, err)
	require.Positive(t, size)
	require.NoError(t, object.Close())
	instance, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	instance, err = pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, "http://pages.example.test")
	require.NoError(t, err)
	syncResult, err := pages_service.ListSitesForSync(t.Context(), instance, pages_service.SyncOptions{Full: true, Limit: 100})
	require.NoError(t, err)
	var synced *pages_service.SiteRecord
	for i := range syncResult.Sites {
		if syncResult.Sites[i].RepoID == site.RepoID {
			synced = &syncResult.Sites[i]
			break
		}
	}
	require.NotNil(t, synced)
	require.Equal(t, version.TarGzSHA256, synced.TarGzSHA256)
}

func TestOpenCurrentContentChecksLazyStorageObject(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site, err := pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{Enabled: true, SourceType: pages_model.PublishSourceArtifact})
	require.NoError(t, err)
	version := &pages_model.Version{RepoID: site.RepoID, TarGzSHA256: strings.Repeat("a", 64), OriginType: pages_model.VersionOriginUpload}
	require.NoError(t, pages_model.InsertVersion(t.Context(), version))
	site.CurrentVersionID = version.ID
	require.NoError(t, pages_model.UpdateSite(t.Context(), site))

	object := &statFailureObject{}
	defer test.MockVariableValue(&storage.Pages, storage.ObjectStorage(statFailureStorage{object: object}))()
	_, _, err = pages_service.OpenCurrentContent(t.Context(), site.RepoID, version.TarGzSHA256)
	require.ErrorContains(t, err, "stat failed")
	require.True(t, object.closed)
}

type statFailureStorage struct {
	object *statFailureObject
}

func (s statFailureStorage) Open(string) (storage.Object, error) { return s.object, nil }
func (statFailureStorage) Save(string, io.Reader, int64) (int64, error) {
	return 0, errors.New("unexpected Save call")
}

func (statFailureStorage) Stat(string) (os.FileInfo, error) {
	return nil, errors.New("unexpected Stat call")
}
func (statFailureStorage) Delete(string) error { return errors.New("unexpected Delete call") }
func (statFailureStorage) ServeDirectURL(string, string, string, *storage.ServeDirectOptions) (*url.URL, error) {
	return nil, errors.New("unexpected ServeDirectURL call")
}

func (statFailureStorage) IterateObjects(string, func(string, storage.Object) error) error {
	return errors.New("unexpected IterateObjects call")
}

type statFailureObject struct {
	closed bool
}

func (*statFailureObject) Read([]byte) (int, error)       { return 0, io.EOF }
func (*statFailureObject) Seek(int64, int) (int64, error) { return 0, nil }
func (o *statFailureObject) Close() error                 { o.closed = true; return nil }
func (*statFailureObject) Stat() (os.FileInfo, error)     { return nil, errors.New("stat failed") }
