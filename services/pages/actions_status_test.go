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

	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, site.TarGzSHA256)
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

	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, site.TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusInProgress, status)
}

func TestGetActionsDeploymentStatusRequiresAllInstances(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := currentActionsSite(t)
	first := currentActionsInstance(t, "https://pages-a.example.test")
	second := currentActionsInstance(t, "https://pages-b.example.test")

	currentActionsResult(t, first, site, true, true, "")
	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, site.TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusInProgress, status)

	currentActionsResult(t, second, site, true, true, "")
	status, err = pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, site.TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusSuccess, status)
}

func TestGetActionsDeploymentStatusReportsCurrentFailure(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := currentActionsSite(t)
	instance := currentActionsInstance(t, "https://pages.example.test")
	currentActionsResult(t, instance, site, false, false, pages_model.InstanceErrorContentDownloadFailed)

	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, site.TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusFailure, status)
}

func TestGetActionsDeploymentStatusIgnoresStaleFailure(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := currentActionsSite(t)
	instance := currentActionsInstance(t, "https://pages.example.test")
	currentActionsResult(t, instance, site, false, false, pages_model.InstanceErrorContentDownloadFailed)

	updated, err := pages_service.SaveSiteSettings(t.Context(), site.RepoID, pages_service.SiteSettings{
		Enabled: true, SourceType: pages_model.SourceTypeBranch, SourceBranch: "main",
	})
	require.NoError(t, err)
	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), updated.RepoID, updated.TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusInProgress, status)
}

func TestGetActionsDeploymentStatusClosedSiteIsCancelled(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := currentActionsSite(t)
	require.NoError(t, pages_service.DisableSite(t.Context(), site.RepoID))

	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, site.TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusCancelled, status)
}

func TestGetActionsDeploymentStatusCustomFailureDoesNotBlockDefault(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := currentActionsSite(t)
	instance := currentActionsInstance(t, "https://pages.example.test")
	currentActionsResult(t, instance, site, true, false, pages_model.InstanceErrorACMEFailed)

	status, err := pages_service.GetActionsDeploymentStatus(t.Context(), site.RepoID, site.TarGzSHA256)
	require.NoError(t, err)
	require.Equal(t, pages_service.ActionsDeploymentStatusSuccess, status)
}

func currentActionsSite(t *testing.T) *pages_model.Site {
	t.Helper()
	site, err := pages_service.SaveSiteSettings(t.Context(), 1, pages_service.SiteSettings{
		Enabled: true, SourceType: pages_model.SourceTypeUpload,
	})
	require.NoError(t, err)
	site.TarGzSHA256 = strings.Repeat("a", 64)
	require.NoError(t, pages_model.UpsertSite(t.Context(), site))
	return site
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
	require.NoError(t, pages_model.UpsertInstanceResult(t.Context(), &pages_model.InstanceResult{
		InstanceID:                  instance.ID,
		RepoID:                      site.RepoID,
		AppliedPagesUpdatedUnixNano: site.PagesUpdatedUnixNano,
		AppliedRootUpdatedUnixNano:  instance.RootUpdatedUnixNano,
		ConfigApplied:               true,
		ActualTarGzSHA256:           site.TarGzSHA256,
		DefaultPublished:            defaultPublished,
		CustomPublished:             customPublished,
		ErrorCode:                   errorCode,
	}))
}
