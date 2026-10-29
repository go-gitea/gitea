// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pages_model "gitea.dev/models/pages"
	"gitea.dev/models/unittest"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"
	pages_service "gitea.dev/services/pages"

	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestPagesContentCommitAndSourceChange(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("first site"), 0o600))
	var artifact bytes.Buffer
	require.NoError(t, pages_content.PackDirectory(t.Context(), root, &artifact, pages_content.DefaultLimits()))
	first, err := pages_service.CommitUploadContent(t.Context(), 1, bytes.NewReader(artifact.Bytes()))
	require.NoError(t, err)
	require.NotEmpty(t, first.TarGzSHA256)

	again, err := pages_service.CommitUploadContent(t.Context(), 1, bytes.NewReader(artifact.Bytes()))
	require.NoError(t, err)
	require.Equal(t, first, again, "a repeated content digest should not create an update")
	_, err = pages_service.CommitUploadContent(t.Context(), 1, bytes.NewBufferString("not gzip"))
	require.ErrorIs(t, err, pages_content.ErrInvalidArchive)
	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, site.Enabled, "an initial upload selects the source but keeps the site closed")
	require.Equal(t, first.TarGzSHA256, site.TarGzSHA256)

	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{Enabled: true, SourceType: pages_model.SourceTypeActions})
	require.NoError(t, err)
	_, err = pages_service.CommitUploadContent(t.Context(), 1, bytes.NewReader(artifact.Bytes()))
	require.ErrorIs(t, err, pages_service.ErrSourceChanged)
	site, has, err = pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, first.TarGzSHA256, site.TarGzSHA256, "switching sources keeps the last good content")
}

func TestPagesPrivateRepositoryCanExplicitlyPublishPublicSite(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site, err := pages_service.SaveSiteSettings(t.Context(), 2, pages_service.SiteSettings{Enabled: true, SourceType: pages_model.SourceTypeUpload})
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

	authenticated, err := pages_service.AuthenticateInstance(t.Context(), "Bearer "+token)
	require.NoError(t, err)
	require.Equal(t, instance.ID, authenticated.ID)
	_, err = pages_service.AuthenticateInstance(t.Context(), "Bearer wrong")
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

	require.NoError(t, pages_model.UpsertInstanceResult(t.Context(), &pages_model.InstanceResult{
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
	_, has, err := pages_model.GetInstanceResult(t.Context(), instance.ID, 1)
	require.NoError(t, err)
	require.False(t, has)
	reregistered, err := pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, resetInstance.TokenHash, "https://pages.example.test")
	require.NoError(t, err)
	require.Equal(t, resetInstance.RootUpdatedUnixNano, reregistered.RootUpdatedUnixNano)
}

func TestPagesReservesManagementDNSNamespace(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.AppDomain, "gitea.example.test")()

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

	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{SourceType: pages_model.SourceTypeUpload})
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
		_, err := pages_service.SaveSiteSettings(t.Context(), repoID, pages_service.SiteSettings{SourceType: pages_model.SourceTypeUpload})
		require.NoError(t, err)
	}
	first, err := pages_service.ListSitesForSync(t.Context(), instance, pages_service.SyncOptions{Full: true, Limit: 1})
	require.NoError(t, err)
	require.Len(t, first.Sites, 1)
	require.NotNil(t, first.NextCursor)
	require.NotEmpty(t, first.Sites[0].Owner)

	secondSite := int64(2)
	if first.Sites[0].RepoID == 2 {
		secondSite = 1
	}
	_, err = pages_service.SaveSiteSettings(t.Context(), secondSite, pages_service.SiteSettings{SourceType: pages_model.SourceTypeBranch, SourceBranch: "main"})
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
	require.Greater(t, updates.NextSince, first.Until)
}

func TestPagesSyncFiltersSettledDeletionsOnlyFromFullListing(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	instance, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	instance, err = pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, "http://pages.example.test")
	require.NoError(t, err)
	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{SourceType: pages_model.SourceTypeUpload})
	require.NoError(t, err)
	base := time.Now().UnixNano()
	require.NoError(t, pages_model.UpsertSite(t.Context(), &pages_model.Site{RepoID: 9998, Deleted: true, PagesUpdatedUnixNano: base + 1}))
	require.NoError(t, pages_model.UpsertSite(t.Context(), &pages_model.Site{RepoID: 9999, Deleted: true, PagesUpdatedUnixNano: base + 2}))
	require.NoError(t, pages_model.UpsertDomain(t.Context(), &pages_model.Domain{
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
	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{SourceType: pages_model.SourceTypeUpload})
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
	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{SourceType: pages_model.SourceTypeBranch, SourceBranch: "main"})
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
	site, err := pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{Enabled: true, SourceType: pages_model.SourceTypeUpload})
	require.NoError(t, err)
	site.TarGzSHA256 = strings.Repeat("a", 64)
	require.NoError(t, pages_model.UpsertSite(t.Context(), site))

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
	require.NoError(t, pages_service.SetTXTVerificationPolicy(t.Context(), false))
	instance, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	instance, err = pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, "http://pages.example.test")
	require.NoError(t, err)
	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{Enabled: true, SourceType: pages_model.SourceTypeUpload})
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
	require.NoError(t, pages_service.AcceptInstanceResult(t.Context(), instance.ID, instance.TokenHash, pages_service.ResultRequest{
		RepoID: 1, AppliedPagesUpdatedUnixNano: site.PagesUpdatedUnixNano,
		AppliedRootUpdatedUnixNano: instance.RootUpdatedUnixNano, ConfigApplied: true,
	}))
	_, has, err = pages_model.GetDomainByHost(t.Context(), "site.example.test")
	require.NoError(t, err)
	require.False(t, has)
}

func TestPagesBranchSourcePreparesCurrentContent(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	_, err := pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{
		SourceType: pages_model.SourceTypeBranch, SourceBranch: "master",
	})
	require.NoError(t, err)
	_, err = pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{
		Enabled: true, SourceType: pages_model.SourceTypeBranch, SourceBranch: "master",
	})
	require.NoError(t, err)
	require.NoError(t, pages_service.UpdateDueBranchSources(t.Context(), 10))
	site, has, err := pages_model.GetSiteByRepoID(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, has)
	require.Empty(t, site.SourceErrorCode)
	require.NotEmpty(t, site.TarGzSHA256)
	object, err := pages_service.OpenCurrentContent(t.Context(), 1, site.TarGzSHA256)
	require.NoError(t, err)
	require.NoError(t, object.Close())
}
