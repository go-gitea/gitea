// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"net/url"

	pages_model "gitea.dev/models/pages"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	pages_service "gitea.dev/services/pages"
)

const tplPages templates.TplName = "repo/settings/pages"

type pagesInstanceResultView struct {
	Instance     *pages_model.Instance
	Result       *pages_model.InstanceResult
	DefaultURL   string
	CustomURL    string
	MatchesState bool
	ErrorMessage string
}

func loadPagesSettingsData(ctx *context.Context) bool {
	repoID := ctx.Repo.Repository.ID
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		ctx.ServerError("GetSiteByRepoID", err)
		return false
	}
	if !has {
		site = &pages_model.Site{RepoID: repoID, SourceType: pages_model.SourceTypeUpload}
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
	_, policyTime, err := pages_service.GetTXTVerificationPolicy(ctx)
	if err != nil {
		ctx.ServerError("GetPagesTXTPolicy", err)
		return false
	}
	var activeHost string
	for _, domain := range domains {
		if domain.State == pages_model.DomainStateActive {
			activeHost = domain.Host
			break
		}
	}
	results := make([]*pagesInstanceResultView, 0, len(instances))
	for _, instance := range instances {
		view := &pagesInstanceResultView{Instance: instance, Result: resultByInstance[instance.ID]}
		if instance.RootURL != "" {
			view.DefaultURL, _ = pages_service.DefaultSiteURL(instance.RootURL, ctx.Repo.Owner.LowerName, ctx.Repo.Repository.LowerName)
			if activeHost != "" {
				root, err := url.Parse(instance.RootURL)
				if err == nil {
					view.CustomURL = root.Scheme + "://" + activeHost + "/"
				}
			}
		}
		if result := view.Result; result != nil {
			view.MatchesState = result.AppliedPagesUpdatedUnixNano == site.PagesUpdatedUnixNano &&
				result.AppliedRootUpdatedUnixNano == instance.RootUpdatedUnixNano &&
				(activeHost == "" || result.AppliedTXTPolicyUpdatedUnixNano == policyTime)
			if result.ErrorCode != "" {
				view.ErrorMessage = ctx.Locale.TrString("repo.settings.pages.error." + string(result.ErrorCode))
			}
		}
		results = append(results, view)
	}

	ctx.Data["Title"] = ctx.Tr("repo.settings.pages")
	ctx.Data["PageIsSettingsPages"] = true
	ctx.Data["PagesSite"] = site
	if site.SourceErrorCode != "" {
		ctx.Data["PagesSourceError"] = ctx.Locale.TrString("repo.settings.pages.error." + string(site.SourceErrorCode))
	}
	ctx.Data["PagesDomains"] = domains
	ctx.Data["PagesInstanceResults"] = results
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
	case "upload":
		file, _, err := ctx.Req.FormFile("archive")
		if err != nil {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.upload_error"))
			break
		}
		defer file.Close()
		if _, err := pages_service.CommitUploadContent(ctx, repoID, file); err != nil {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.upload_error"))
		} else {
			ctx.Flash.Success(ctx.Tr("repo.settings.pages.upload_success"))
		}
	case "save":
		_, err := pages_service.SaveSiteSettings(ctx, repoID, pages_service.SiteSettings{
			Enabled:      ctx.FormBool("enabled"),
			SourceType:   pages_model.SourceType(ctx.FormTrim("source_type")),
			SourceBranch: ctx.FormTrim("source_branch"),
			SourcePath:   ctx.FormTrim("source_path"),
		})
		if err != nil {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.save_error"))
		} else {
			ctx.Flash.Success(ctx.Tr("repo.settings.pages.save_success"))
		}
	case "close":
		if err := pages_service.DisableSite(ctx, repoID); err != nil {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.close_error"))
		} else {
			ctx.Flash.Success(ctx.Tr("repo.settings.pages.close_success"))
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
