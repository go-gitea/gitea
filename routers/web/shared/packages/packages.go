// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"fmt"
	"net/http"
	"time"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	cargo_service "code.gitea.io/gitea/services/packages/cargo"
	container_service "code.gitea.io/gitea/services/packages/container"
)

func SetPackagesContext(ctx *context.Context, owner *user_model.User) {
	pcrs, err := packages_model.GetCleanupRulesByOwner(ctx, owner.ID)
	if err != nil {
		ctx.ServerError("GetCleanupRulesByOwner", err)
		return
	}

	ctx.Data["CleanupRules"] = pcrs
}

func SetRuleAddContext(ctx *context.Context) {
	setRuleEditContext(ctx, nil)
}

func SetRuleEditContext(ctx *context.Context, owner *user_model.User) {
	pcr := getCleanupRuleByContext(ctx, owner)
	if pcr == nil {
		return
	}

	setRuleEditContext(ctx, pcr)
}

func setRuleEditContext(ctx *context.Context, pcr *packages_model.PackageCleanupRule) {
	ctx.Data["IsEditRule"] = pcr != nil

	if pcr == nil {
		pcr = &packages_model.PackageCleanupRule{}
	}
	ctx.Data["CleanupRule"] = pcr
	ctx.Data["AvailableTypes"] = packages_model.TypeList
}

func PerformRuleAddPost(ctx *context.Context, owner *user_model.User, redirectURL string, template templates.TplName) {
	performRuleEditPost(ctx, owner, nil, redirectURL, template)
}

func PerformRuleEditPost(ctx *context.Context, owner *user_model.User, redirectURL string, template templates.TplName) {
	pcr := getCleanupRuleByContext(ctx, owner)
	if pcr == nil {
		return
	}

	form := web.GetForm(ctx).(*forms.PackageCleanupRuleForm)

	if form.Action == "remove" {
		if err := packages_model.DeleteCleanupRuleByID(ctx, pcr.ID); err != nil {
			ctx.ServerError("DeleteCleanupRuleByID", err)
			return
		}

		ctx.Flash.Success(ctx.Tr("packages.owner.settings.cleanuprules.success.delete"))
		ctx.Redirect(redirectURL)
	} else {
		performRuleEditPost(ctx, owner, pcr, redirectURL, template)
	}
}

func performRuleEditPost(ctx *context.Context, owner *user_model.User, pcr *packages_model.PackageCleanupRule, redirectURL string, template templates.TplName) {
	isEditRule := pcr != nil

	if pcr == nil {
		pcr = &packages_model.PackageCleanupRule{}
	}

	form := web.GetForm(ctx).(*forms.PackageCleanupRuleForm)

	pcr.Enabled = form.Enabled
	pcr.OwnerID = owner.ID
	pcr.KeepCount = form.KeepCount
	pcr.KeepPattern = form.KeepPattern
	pcr.RemoveDays = form.RemoveDays
	pcr.RemovePattern = form.RemovePattern
	pcr.MatchFullName = form.MatchFullName

	ctx.Data["IsEditRule"] = isEditRule
	ctx.Data["CleanupRule"] = pcr
	ctx.Data["AvailableTypes"] = packages_model.TypeList

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, template)
		return
	}

	if isEditRule {
		if err := packages_model.UpdateCleanupRule(ctx, pcr); err != nil {
			ctx.ServerError("UpdateCleanupRule", err)
			return
		}
	} else {
		pcr.Type = packages_model.Type(form.Type)

		if has, err := packages_model.HasOwnerCleanupRuleForPackageType(ctx, owner.ID, pcr.Type); err != nil {
			ctx.ServerError("HasOwnerCleanupRuleForPackageType", err)
			return
		} else if has {
			ctx.Data["Err_Type"] = true
			ctx.HTML(http.StatusOK, template)
			return
		}

		var err error
		if pcr, err = packages_model.InsertCleanupRule(ctx, pcr); err != nil {
			ctx.ServerError("InsertCleanupRule", err)
			return
		}
	}

	ctx.Flash.Success(ctx.Tr("packages.owner.settings.cleanuprules.success.update"))
	ctx.Redirect(fmt.Sprintf("%s/rules/%d", redirectURL, pcr.ID))
}

func SetRulePreviewContext(ctx *context.Context, owner *user_model.User) {
	pcr := getCleanupRuleByContext(ctx, owner)
	if pcr == nil {
		return
	}

	if err := pcr.CompiledPattern(); err != nil {
		ctx.ServerError("CompiledPattern", err)
		return
	}

	olderThan := time.Now().AddDate(0, 0, -pcr.RemoveDays)

	packages, err := packages_model.GetPackagesByType(ctx, pcr.OwnerID, pcr.Type)
	if err != nil {
		ctx.ServerError("GetPackagesByType", err)
		return
	}

	versionsToRemove := make([]*packages_model.PackageDescriptor, 0, 10)

	for _, p := range packages {
		pvs, _, err := packages_model.SearchVersions(ctx, &packages_model.PackageSearchOptions{
			PackageID:  p.ID,
			IsInternal: optional.Some(false),
			Sort:       packages_model.SortCreatedDesc,
			Paginator:  db.NewAbsoluteListOptions(pcr.KeepCount, 200),
		})
		if err != nil {
			ctx.ServerError("SearchVersions", err)
			return
		}
		for _, pv := range pvs {
			if skip, err := container_service.ShouldBeSkipped(ctx, pcr, p, pv); err != nil {
				ctx.ServerError("ShouldBeSkipped", err)
				return
			} else if skip {
				continue
			}

			toMatch := pv.LowerVersion
			if pcr.MatchFullName {
				toMatch = p.LowerName + "/" + pv.LowerVersion
			}

			if pcr.KeepPatternMatcher != nil && pcr.KeepPatternMatcher.MatchString(toMatch) {
				continue
			}
			if pv.CreatedUnix.AsLocalTime().After(olderThan) {
				continue
			}
			if pcr.RemovePatternMatcher != nil && !pcr.RemovePatternMatcher.MatchString(toMatch) {
				continue
			}

			pd, err := packages_model.GetPackageDescriptor(ctx, pv)
			if err != nil {
				ctx.ServerError("GetPackageDescriptor", err)
				return
			}
			versionsToRemove = append(versionsToRemove, pd)
		}
	}

	ctx.Data["CleanupRule"] = pcr
	ctx.Data["VersionsToRemove"] = versionsToRemove
}

func getCleanupRuleByContext(ctx *context.Context, owner *user_model.User) *packages_model.PackageCleanupRule {
	id := ctx.FormInt64("id")
	if id == 0 {
		id = ctx.PathParamInt64("id")
	}

	pcr, err := packages_model.GetCleanupRuleByID(ctx, id)
	if err != nil {
		if err == packages_model.ErrPackageCleanupRuleNotExist {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetCleanupRuleByID", err)
		}
		return nil
	}

	if pcr != nil && pcr.OwnerID == owner.ID {
		return pcr
	}

	ctx.NotFound(fmt.Errorf("PackageCleanupRule[%v] not associated to owner %v", id, owner))

	return nil
}

func InitializeCargoIndex(ctx *context.Context, owner *user_model.User) {
	err := cargo_service.InitializeIndexRepository(ctx, owner, owner)
	if err != nil {
		log.Error("InitializeIndexRepository failed: %v", err)
		ctx.Flash.Error(ctx.Tr("packages.owner.settings.cargo.initialize.error", err))
	} else {
		ctx.Flash.Success(ctx.Tr("packages.owner.settings.cargo.initialize.success"))
	}
}

func RebuildCargoIndex(ctx *context.Context, owner *user_model.User) {
	err := cargo_service.RebuildIndex(ctx, owner, owner)
	if err != nil {
		log.Error("RebuildIndex failed: %v", err)
		ctx.Flash.Error(ctx.Tr("packages.owner.settings.cargo.rebuild.error", err))
	} else {
		ctx.Flash.Success(ctx.Tr("packages.owner.settings.cargo.rebuild.success"))
	}
}
