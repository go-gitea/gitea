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

type pagesPublicationState string

const (
	pagesPublicationPublishing pagesPublicationState = "publishing"
	pagesPublicationPublished  pagesPublicationState = "published"
	pagesPublicationFailed     pagesPublicationState = "failed"
)

type pagesAddressView struct {
	URL             string
	InstanceRootURL string
	State           pagesPublicationState
	ErrorMessage    string
}

type pagesPublicationView struct {
	Addresses []*pagesAddressView
	State     pagesPublicationState
	Partial   bool
}

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

func (view *pagesPublicationView) finalizeStatus() {
	failed := 0
	publishing := false
	for _, address := range view.Addresses {
		switch address.State {
		case pagesPublicationFailed:
			failed++
		case pagesPublicationPublishing:
			publishing = true
		}
	}
	if failed > 0 {
		view.State = pagesPublicationFailed
		view.Partial = failed < len(view.Addresses)
	} else if publishing {
		view.State = pagesPublicationPublishing
	} else if len(view.Addresses) > 0 {
		view.State = pagesPublicationPublished
	}
}

func classifyPagesPublication(result *pages_model.InstanceResult, pagesUpdated, rootUpdated int64, requireGiteaTXTRecord, custom bool) (pagesPublicationState, pages_model.InstanceErrorCode) {
	if result == nil || result.AppliedPagesUpdatedUnixNano != pagesUpdated || result.AppliedRootUpdatedUnixNano != rootUpdated ||
		(custom && result.AppliedRequireGiteaTXTRecord != requireGiteaTXTRecord) {
		return pagesPublicationPublishing, ""
	}
	published := result.DefaultPublished
	if custom {
		published = result.CustomPublished
	}
	if published {
		return pagesPublicationPublished, ""
	}
	if result.ErrorCode == "" || result.ErrorCode == pages_model.InstanceErrorCertificatePending {
		return pagesPublicationPublishing, ""
	}
	return pagesPublicationFailed, result.ErrorCode
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
	instances, err := pages_model.ListInstances(ctx)
	if err != nil {
		ctx.ServerError("ListInstances", err)
		return false
	}
	siteResults, err := pages_model.ListInstanceResultsByRepoID(ctx, repoID)
	if err != nil {
		ctx.ServerError("ListInstanceResultsByRepoID", err)
		return false
	}
	resultByInstance := make(map[int64]*pages_model.InstanceResult, len(siteResults))
	for _, result := range siteResults {
		resultByInstance[result.InstanceID] = result
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

	defaultPublication := &pagesPublicationView{Addresses: make([]*pagesAddressView, 0, len(instances))}
	customPublication := &pagesPublicationView{Addresses: make([]*pagesAddressView, 0, len(instances))}
	for _, instance := range instances {
		if instance.RootURL == "" {
			continue
		}
		defaultURL, err := pages_service.DefaultSiteURL(instance.RootURL, ctx.Repo.Owner.LowerName, ctx.Repo.Repository.LowerName)
		if err != nil {
			defaultPublication.Addresses = append(defaultPublication.Addresses, &pagesAddressView{
				URL: instance.RootURL, State: pagesPublicationFailed,
				ErrorMessage: ctx.Locale.TrString("repo.settings.pages.instance.address_error"),
			})
		} else {
			state, errorCode := classifyPagesPublication(resultByInstance[instance.ID], site.PagesUpdatedUnixNano, instance.RootUpdatedUnixNano, false, false)
			address := &pagesAddressView{URL: defaultURL, State: state}
			if errorCode != "" {
				address.ErrorMessage = ctx.Locale.TrString("repo.settings.pages.error." + string(errorCode))
			}
			defaultPublication.Addresses = append(defaultPublication.Addresses, address)
		}

		if activeDomain == nil {
			continue
		}
		state, errorCode := classifyPagesPublication(resultByInstance[instance.ID], site.PagesUpdatedUnixNano, instance.RootUpdatedUnixNano, setting_module.Pages.RequireGiteaTXTRecord, true)
		address := &pagesAddressView{InstanceRootURL: instance.RootURL, State: state}
		if errorCode != "" {
			address.ErrorMessage = ctx.Locale.TrString("repo.settings.pages.error." + string(errorCode))
		}
		customPublication.Addresses = append(customPublication.Addresses, address)
	}
	defaultPublication.finalizeStatus()
	customPublication.finalizeStatus()

	ctx.Data["Title"] = ctx.Tr("repo.settings.pages")
	ctx.Data["PageIsSettingsPages"] = true
	ctx.Data["PageIsSettingsPagesSettings"] = true
	ctx.Data["PagesHasSite"] = hasSite
	ctx.Data["PagesSite"] = site
	ctx.Data["PagesHasContent"] = site.CurrentVersionID != 0
	ctx.Data["PagesLastChangeUnix"] = timeutil.TimeStamp(site.PagesUpdatedUnixNano / int64(time.Second))
	ctx.Data["PagesDefaultPublication"] = defaultPublication
	ctx.Data["PagesCustomPublication"] = customPublication
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
