// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"time"

	pages_model "gitea.dev/models/pages"
	setting_module "gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/timeutil"
	"gitea.dev/services/context"
	pages_service "gitea.dev/services/pages"
)

const tplPages templates.TplName = "repo/settings/pages"

type pagesDomainView struct {
	ID                 int64
	Host               string
	TXTName            string
	TXTToken           string
	TXTResult          pages_model.TXTResult
	TXTText            string
	TXTLastSuccessUnix timeutil.TimeStamp
	TXTLastCheckUnix   timeutil.TimeStamp
}

func loadPagesSettingsData(ctx *context.Context) bool {
	repoID := ctx.Repo.Repository.ID
	site, hasSite, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		ctx.ServerError("GetSiteByRepoID", err)
		return false
	}
	if !hasSite {
		site = &pages_model.Site{RepoID: repoID}
	}
	domains, err := pages_model.ListDomainsByRepoID(ctx, repoID)
	if err != nil {
		ctx.ServerError("ListDomainsByRepoID", err)
		return false
	}
	var activeDomain, pendingDomain, releasingDomain *pagesDomainView
	for _, domain := range domains {
		view := &pagesDomainView{
			ID: domain.ID, Host: domain.Host, TXTName: "_gitea-pages." + domain.Host, TXTToken: domain.TXTToken,
			TXTResult:          domain.TXTResult,
			TXTText:            ctx.Locale.TrString("repo.settings.pages.domain.txt." + string(domain.TXTResult)),
			TXTLastSuccessUnix: timeutil.TimeStamp(domain.TXTLastSuccessUnixNano / int64(time.Second)),
			TXTLastCheckUnix:   timeutil.TimeStamp(domain.TXTLastCheckUnix),
		}
		switch domain.State {
		case pages_model.DomainStateActive:
			activeDomain = view
		case pages_model.DomainStatePending:
			pendingDomain = view
		case pages_model.DomainStateReleasing:
			releasingDomain = view
		}
	}

	ctx.Data["Title"] = ctx.Tr("repo.settings.pages")
	ctx.Data["PageIsSettingsPages"] = true
	ctx.Data["PageIsSettingsPagesSettings"] = true
	ctx.Data["PagesHasSite"] = hasSite
	ctx.Data["PagesSite"] = site
	ctx.Data["PagesActiveDomain"] = activeDomain
	ctx.Data["PagesPendingDomain"] = pendingDomain
	ctx.Data["PagesReleasingDomain"] = releasingDomain
	ctx.Data["PagesRequireGiteaTXTRecord"] = setting_module.Pages.RequireGiteaTXTRecord
	if site.SourceErrorCode != "" {
		ctx.Data["PagesSourceError"] = ctx.Locale.TrString("repo.settings.pages.error." + string(site.SourceErrorCode))
	}
	return true
}

// Pages shows the Pages settings for a repository.
func Pages(ctx *context.Context) {
	if !loadPagesSettingsData(ctx) {
		return
	}
	ctx.HTML(http.StatusOK, tplPages)
}

// PagesPost updates the repository's Pages source or custom-domain bindings.
func PagesPost(ctx *context.Context) {
	repoID := ctx.Repo.Repository.ID
	switch ctx.FormString("action") {
	case "save":
		_, err := pages_service.SaveSiteSettings(ctx, repoID, pages_service.SiteSettings{
			Enabled:      ctx.FormBool("enabled"),
			SourceType:   pages_model.PublishSource(ctx.FormTrim("source_type")),
			SourceBranch: ctx.FormTrim("source_branch"),
			SourcePath:   ctx.FormTrim("source_path"),
		})
		if err != nil {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.save_error"))
		} else {
			ctx.Flash.Success(ctx.Tr("repo.settings.pages.save_success"))
		}
	case "add-domain":
		if _, err := pages_service.AddDomain(ctx, repoID, ctx.FormTrim("host")); err != nil {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.domain_add_error"))
		} else {
			ctx.Flash.Success(ctx.Tr("repo.settings.pages.domain_add_success"))
		}
	case "remove-domain":
		if err := pages_service.RemoveDomain(ctx, repoID, ctx.FormInt64("domain_id")); err != nil {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.domain_remove_error"))
		} else {
			ctx.Flash.Success(ctx.Tr("repo.settings.pages.domain_remove_success"))
		}
	default:
		ctx.Flash.Error(ctx.Tr("repo.settings.pages.invalid_action"))
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/pages")
}
