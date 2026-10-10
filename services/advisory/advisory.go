// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"

	advisory_model "gitea.dev/models/advisory"
	audit_model "gitea.dev/models/audit"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"
	notify_service "gitea.dev/services/notify"
)

// isBlockedFromAdvisory also checks the reporter because a private report is their content
func isBlockedFromAdvisory(ctx context.Context, a *advisory_model.Advisory, u *user_model.User) bool {
	if a.IsReport {
		return user_model.IsUserBlockedBy(ctx, u, a.Repo.OwnerID, a.ReporterID)
	}
	return user_model.IsUserBlockedBy(ctx, u, a.Repo.OwnerID)
}

// CreateAdvisory creates a draft, only repository admins may call it
func CreateAdvisory(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, opts *ContentOptions) (*advisory_model.Advisory, error) {
	a := &advisory_model.Advisory{RepoID: repo.ID, Repo: repo, State: advisory_model.StateDraft, ReporterID: doer.ID, Reporter: doer}
	if err := applyContent(ctx, doer, a, opts); err != nil {
		return nil, err
	}
	if err := advisory_model.CreateAdvisory(ctx, a); err != nil {
		return nil, err
	}
	notify_service.NewSecurityAdvisory(ctx, doer, a)
	return a, nil
}

func IsPrivateReportingEnabled(ctx context.Context, repo *repo_model.Repository) bool {
	if repo.IsArchived || unit.TypeSecurityAdvisories.UnitGlobalDisabled() {
		return false
	}
	ru, err := repo.GetUnit(ctx, unit.TypeSecurityAdvisories)
	return err == nil && ru.SecurityAdvisoriesConfig().PrivateVulnerabilityReporting
}

// IsPrivateReportingConfigured ignores whether reports are possible at the moment, unlike IsPrivateReportingEnabled
func IsPrivateReportingConfigured(ctx context.Context, repo *repo_model.Repository) bool {
	return repo.MustGetUnit(ctx, unit.TypeSecurityAdvisories).SecurityAdvisoriesConfig().PrivateVulnerabilityReporting
}

// RecordPrivateReportingChange audits a change of the setting, also by the repository settings which update all units at once
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

// ReportVulnerability creates a triage advisory from a private report, the reporter is credited and can't assign a CVE ID
func ReportVulnerability(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, opts *ContentOptions) (*advisory_model.Advisory, error) {
	if !doer.IsIndividual() || !IsPrivateReportingEnabled(ctx, repo) {
		return nil, util.NewPermissionDeniedErrorf("private vulnerability reporting is not enabled")
	}
	if user_model.IsUserBlockedBy(ctx, doer, repo.OwnerID) {
		return nil, user_model.ErrBlockedUser
	}
	opts.CveID, opts.LabelIDs = "", nil
	opts.Credits = []*advisory_model.Credit{{UserID: doer.ID, Type: "reporter"}}
	a := &advisory_model.Advisory{RepoID: repo.ID, Repo: repo, State: advisory_model.StateTriage, IsReport: true, ReporterID: doer.ID, Reporter: doer}
	if err := applyContent(ctx, doer, a, opts); err != nil {
		return nil, err
	}
	if err := advisory_model.CreateAdvisory(ctx, a); err != nil {
		return nil, err
	}
	notify_service.NewSecurityAdvisoryReport(ctx, doer, a)
	return a, nil
}

// DeleteAdvisory deletes an advisory which has never been published, only repository admins may call it
func DeleteAdvisory(ctx context.Context, doer *user_model.User, a *advisory_model.Advisory) error {
	if err := advisory_model.DeleteAdvisory(ctx, a); err != nil {
		return err
	}
	notify_service.DeleteSecurityAdvisory(ctx, doer, a)
	return nil
}
