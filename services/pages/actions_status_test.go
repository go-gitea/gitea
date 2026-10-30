// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages_test

import (
	"strings"
	"testing"

	pages_model "gitea.dev/models/pages"
	"gitea.dev/models/unittest"
	pages_service "gitea.dev/services/pages"

	"github.com/stretchr/testify/require"
)

func TestGetActionsDeploymentStatusWithoutServiceIsInProgress(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := currentActionsSite(t)

	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, currentActionsVersion(t, site).TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusInProgress, status)
}

func TestGetActionsDeploymentStatusWithoutReportIsInProgress(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := currentActionsSite(t)
	instance, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	_, err = pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, "https://pages.example.test")
	require.NoError(t, err)

	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, currentActionsVersion(t, site).TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusInProgress, status)
}

func TestGetActionsDeploymentStatusRequiresAllInstances(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := currentActionsSite(t)
	first := currentActionsInstance(t, "https://pages-a.example.test")
	second := currentActionsInstance(t, "https://pages-b.example.test")

	currentActionsResult(t, first, site, true, true, "")
	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, currentActionsVersion(t, site).TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusInProgress, status)

	currentActionsResult(t, second, site, true, true, "")
	status, err = pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, currentActionsVersion(t, site).TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusSuccess, status)
}

func TestGetActionsDeploymentStatusReportsCurrentFailure(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := currentActionsSite(t)
	instance := currentActionsInstance(t, "https://pages.example.test")
	currentActionsResult(t, instance, site, false, false, pages_model.InstanceErrorContentDownloadFailed)

	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, currentActionsVersion(t, site).TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusFailure, status)
}

func TestGetActionsDeploymentStatusIgnoresStaleFailure(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := currentActionsSite(t)
	instance := currentActionsInstance(t, "https://pages.example.test")
	currentActionsResult(t, instance, site, false, false, pages_model.InstanceErrorContentDownloadFailed)

	updated, err := pages_service.SaveSiteSettings(t.Context(), site.RepoID, pages_service.SiteSettings{
		Enabled: true, SourceType: pages_model.PublishSourceBranch, SourceBranch: "main",
	})
	require.NoError(t, err)
	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), updated.RepoID, currentActionsVersion(t, updated).TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusInProgress, status)
}

func TestGetActionsDeploymentStatusClosedSiteIsCancelled(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := currentActionsSite(t)
	_, err := pages_service.SaveSiteSettings(t.Context(), site.RepoID, pages_service.SiteSettings{
		SourceType: pages_model.PublishSourceArtifact,
	})
	require.NoError(t, err)

	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, currentActionsVersion(t, site).TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusCancelled, status)
}

func TestGetActionsDeploymentStatusCustomFailureDoesNotBlockDefault(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := currentActionsSite(t)
	instance := currentActionsInstance(t, "https://pages.example.test")
	currentActionsResult(t, instance, site, true, false, pages_model.InstanceErrorACMEFailed)

	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, currentActionsVersion(t, site).TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusSuccess, status)
}

func currentActionsSite(t *testing.T) *pages_model.Site {
	t.Helper()
	site, err := pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{
		Enabled: true, SourceType: pages_model.PublishSourceArtifact,
	})
	require.NoError(t, err)
	version := &pages_model.Version{RepoID: site.RepoID, TarGzSHA256: strings.Repeat("a", 64), OriginType: pages_model.VersionOriginActions}
	require.NoError(t, pages_model.InsertVersion(t.Context(), version))
	site.CurrentVersionID = version.ID
	require.NoError(t, pages_model.UpdateSite(t.Context(), site))
	return site
}

func currentActionsVersion(t *testing.T, site *pages_model.Site) *pages_model.Version {
	t.Helper()
	version, has, err := pages_model.GetCurrentVersion(t.Context(), site)
	require.NoError(t, err)
	require.True(t, has)
	return version
}

func currentActionsInstance(t *testing.T, rootURL string) *pages_model.Instance {
	t.Helper()
	instance, _, err := pages_service.CreateInstance(t.Context())
	require.NoError(t, err)
	instance, err = pages_service.RegisterInstanceRootURL(t.Context(), instance.ID, instance.TokenHash, rootURL)
	require.NoError(t, err)
	return instance
}

func currentActionsResult(t *testing.T, instance *pages_model.Instance, site *pages_model.Site, defaultPublished, customPublished bool, errorCode pages_model.InstanceErrorCode) {
	t.Helper()
	require.NoError(t, pages_model.InsertInstanceResult(t.Context(), &pages_model.InstanceResult{
		InstanceID:                  instance.ID,
		RepoID:                      site.RepoID,
		AppliedPagesUpdatedUnixNano: site.PagesUpdatedUnixNano,
		AppliedRootUpdatedUnixNano:  instance.RootUpdatedUnixNano,
		ConfigApplied:               true,
		ActualTarGzSHA256:           currentActionsVersion(t, site).TarGzSHA256,
		DefaultPublished:            defaultPublished,
		CustomPublished:             customPublished,
		ErrorCode:                   errorCode,
	}))
}
