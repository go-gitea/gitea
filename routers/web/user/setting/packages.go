// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	chef_module "code.gitea.io/gitea/modules/packages/chef"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
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

func InitializeCargoIndex(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsSettingsPackages"] = true

	shared.InitializeCargoIndex(ctx, ctx.Doer)

	ctx.Redirect(setting.AppSubURL + "/user/settings/packages")
}

func RebuildCargoIndex(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("packages.title")
	ctx.Data["PageIsSettingsPackages"] = true

	shared.RebuildCargoIndex(ctx, ctx.Doer)

	ctx.Redirect(setting.AppSubURL + "/user/settings/packages")
}

func RegenerateChefKeyPair(ctx *context.Context) {
	priv, pub, err := util.GenerateKeyPair(chef_module.KeyBits)
	if err != nil {
		ctx.ServerError("GenerateKeyPair", err)
		return
	}

	if err := user_model.SetUserSetting(ctx, ctx.Doer.ID, chef_module.SettingPublicPem, pub); err != nil {
		ctx.ServerError("SetUserSetting", err)
		return
	}

	ctx.ServeContent(strings.NewReader(priv), &context.ServeHeaderOptions{
		ContentType: "application/x-pem-file",
		Filename:    ctx.Doer.Name + ".priv",
	})
}
