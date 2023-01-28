// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package org

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	shared "code.gitea.io/gitea/routers/web/shared/packages"
)

const (
	tplSettingsPackages            base.TplName = "org/settings/packages"
	tplSettingsPackagesRuleEdit    base.TplName = "org/settings/packages_cleanup_rules_edit"
	tplSettingsPackagesRulePreview base.TplName = "org/settings/packages_cleanup_rules_preview"
)

func Packages(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsPackages"] = true

	shared.SetPackagesContext(ctx, ctx.ContextUser)

	ctx.HTML(http.StatusOK, tplSettingsPackages)
}

func PackagesRuleAdd(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsPackages"] = true

	shared.SetRuleAddContext(ctx)

	ctx.HTML(http.StatusOK, tplSettingsPackagesRuleEdit)
}

func PackagesRuleEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsPackages"] = true

	shared.SetRuleEditContext(ctx, ctx.ContextUser)

	ctx.HTML(http.StatusOK, tplSettingsPackagesRuleEdit)
}

func PackagesRuleAddPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsPackages"] = true

	shared.PerformRuleAddPost(
		ctx,
		ctx.ContextUser,
		fmt.Sprintf("%s/org/%s/settings/packages", setting.AppSubURL, ctx.ContextUser.Name),
		tplSettingsPackagesRuleEdit,
	)
}

func PackagesRuleEditPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsPackages"] = true

	shared.PerformRuleEditPost(
		ctx,
		ctx.ContextUser,
		fmt.Sprintf("%s/org/%s/settings/packages", setting.AppSubURL, ctx.ContextUser.Name),
		tplSettingsPackagesRuleEdit,
	)
}

func PackagesRulePreview(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsOrgSettings"] = true
	ctx.Data["PageIsSettingsPackages"] = true

	shared.SetRulePreviewContext(ctx, ctx.ContextUser)

	ctx.HTML(http.StatusOK, tplSettingsPackagesRulePreview)
}
