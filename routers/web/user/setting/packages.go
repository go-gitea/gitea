// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"net/http"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	shared "code.gitea.io/gitea/routers/web/shared/packages"
)

const (
	tplSettingsPackages            base.TplName = "user/settings/packages"
	tplSettingsPackagesRuleEdit    base.TplName = "user/settings/packages_cleanup_rules_edit"
	tplSettingsPackagesRulePreview base.TplName = "user/settings/packages_cleanup_rules_preview"
)

func Packages(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsSettingsPackages"] = true

	shared.SetPackagesContext(ctx, ctx.Doer)

	ctx.HTML(http.StatusOK, tplSettingsPackages)
}

func PackagesRuleAdd(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsSettingsPackages"] = true

	shared.SetRuleAddContext(ctx)

	ctx.HTML(http.StatusOK, tplSettingsPackagesRuleEdit)
}

func PackagesRuleEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsSettingsPackages"] = true

	shared.SetRuleEditContext(ctx, ctx.Doer)

	ctx.HTML(http.StatusOK, tplSettingsPackagesRuleEdit)
}

func PackagesRuleAddPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsPackages"] = true

	shared.PerformRuleAddPost(
		ctx,
		ctx.Doer,
		setting.AppSubURL+"/user/settings/packages",
		tplSettingsPackagesRuleEdit,
	)
}

func PackagesRuleEditPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsSettingsPackages"] = true

	shared.PerformRuleEditPost(
		ctx,
		ctx.Doer,
		setting.AppSubURL+"/user/settings/packages",
		tplSettingsPackagesRuleEdit,
	)
}

func PackagesRulePreview(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsSettingsPackages"] = true

	shared.SetRulePreviewContext(ctx, ctx.Doer)

	ctx.HTML(http.StatusOK, tplSettingsPackagesRulePreview)
}
