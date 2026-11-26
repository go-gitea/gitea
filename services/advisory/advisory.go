// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"

	advisory_model "gitea.dev/models/advisory"
	audit_model "gitea.dev/models/audit"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"
	notify_service "gitea.dev/services/notify"
)

type requestViewerKey struct{}

func RequestViewer(ctx reqctx.RequestContext, doer *user_model.User, repo *repo_model.Repository, perm access_model.Permission, publicOnly bool) advisory_model.Viewer {
	if viewer, ok := ctx.GetContextValue(requestViewerKey{}).(advisory_model.Viewer); ok {
		return viewer
	}
	viewer, err := advisory_model.NewViewer(ctx, doer, repo, perm, publicOnly)
	if err != nil {
		log.Error("NewViewer: %v", err)
	}
	ctx.SetContextValue(requestViewerKey{}, viewer)
	return viewer
}

type requestAdvisoryKey struct{}

type requestAdvisory struct {
	advisory *advisory_model.Advisory
	perms    advisory_model.Permissions
}

func SetRequestAdvisory(ctx reqctx.RequestContext, advisory *advisory_model.Advisory, perms advisory_model.Permissions) {
	ctx.SetContextValue(requestAdvisoryKey{}, &requestAdvisory{advisory: advisory, perms: perms})
}

func RequestAdvisory(ctx context.Context) (*advisory_model.Advisory, advisory_model.Permissions) {
	loaded := reqctx.MustContextValue[*requestAdvisory](ctx, requestAdvisoryKey{})
	return loaded.advisory, loaded.perms
}

// isBlockedFromAdvisory also checks the reporter because a private report is their content
func isBlockedFromAdvisory(ctx context.Context, advisory *advisory_model.Advisory, user *user_model.User) bool {
	if advisory.IsReport {
		return user_model.IsUserBlockedBy(ctx, user, advisory.Repo.OwnerID, advisory.ReporterID)
	}
	return user_model.IsUserBlockedBy(ctx, user, advisory.Repo.OwnerID)
}

// CreateAdvisory requires the caller to check repository admin permission.
func CreateAdvisory(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, opts *ContentOptions) (*advisory_model.Advisory, error) {
	advisory, err := createAdvisory(ctx, doer, repo, advisory_model.StateDraft, opts)
	if err != nil {
		return nil, err
	}
	audit.RecordAs(ctx, doer, audit_model.SecurityAdvisoryCreate, repo, "advisory", advisory.Identifier)
	return advisory, nil
}

func createAdvisory(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, state advisory_model.State, opts *ContentOptions) (*advisory_model.Advisory, error) {
	advisory := &advisory_model.Advisory{
		RepoID: repo.ID, Repo: repo, State: state, IsReport: state == advisory_model.StateTriage, ReporterID: doer.ID, Reporter: doer,
	}
	if err := applyContent(ctx, doer, advisory, opts); err != nil {
		return nil, err
	}
	if err := advisory_model.CreateAdvisory(ctx, advisory); err != nil {
		return nil, err
	}
	return advisory, nil
}

func IsPrivateReportingEnabled(ctx context.Context, repo *repo_model.Repository) bool {
	if repo.IsArchived || unit.TypeSecurityAdvisories.UnitGlobalDisabled() {
		return false
	}
	ru, err := repo.GetUnit(ctx, unit.TypeSecurityAdvisories)
	return err == nil && ru.SecurityAdvisoriesConfig().PrivateVulnerabilityReporting
}

func IsPrivateReportingEnabledForUser(ctx context.Context, repo *repo_model.Repository, user *user_model.User) bool {
	return user.IsIndividual() && IsPrivateReportingEnabled(ctx, repo)
}

func IsPrivateReportingConfigured(ctx context.Context, repo *repo_model.Repository) bool {
	return repo.MustGetUnit(ctx, unit.TypeSecurityAdvisories).SecurityAdvisoriesConfig().PrivateVulnerabilityReporting
}

func RecordPrivateReportingChange(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, wasEnabled, enabled bool) {
	if wasEnabled != enabled {
		audit.RecordAs(ctx, doer, audit_model.SecurityAdvisoryPrivateReporting, repo, "enabled", enabled)
	}
}

func SetPrivateReporting(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, enabled bool) error {
	ru, err := repo.GetUnit(ctx, unit.TypeSecurityAdvisories)
	if err != nil {
		return err
	}
	cfg := ru.SecurityAdvisoriesConfig()
	wasEnabled := cfg.PrivateVulnerabilityReporting
	if wasEnabled == enabled {
		return nil
	}
	cfg.PrivateVulnerabilityReporting = enabled
	if err := repo_model.UpdateRepoUnitConfig(ctx, ru); err != nil {
		return err
	}
	RecordPrivateReportingChange(ctx, doer, repo, wasEnabled, enabled)
	return nil
}

func ReportVulnerability(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, opts *ContentOptions) (*advisory_model.Advisory, error) {
	if !IsPrivateReportingEnabledForUser(ctx, repo, doer) {
		return nil, util.NewPermissionDeniedErrorf("private vulnerability reporting is not enabled")
	}
	if user_model.IsUserBlockedBy(ctx, doer, repo.OwnerID) {
		return nil, user_model.ErrBlockedUser
	}
	opts.CveID, opts.LabelIDs = "", nil
	opts.Credits = []*advisory_model.Credit{{UserID: doer.ID, Type: "reporter"}}
	advisory, err := createAdvisory(ctx, doer, repo, advisory_model.StateTriage, opts)
	if err != nil {
		return nil, err
	}
	notify_service.NewSecurityAdvisoryReport(ctx, doer, advisory)
	return advisory, nil
}

// DeleteAdvisory requires the caller to check repository admin permission.
func DeleteAdvisory(ctx context.Context, doer *user_model.User, advisory *advisory_model.Advisory) error {
	if err := advisory_model.DeleteAdvisory(ctx, advisory); err != nil {
		return err
	}
	audit.RecordAs(ctx, doer, audit_model.SecurityAdvisoryDelete, advisory.Repo, "advisory", advisory.Identifier)
	return nil
}
