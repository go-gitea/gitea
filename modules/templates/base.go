// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"strings"
	"time"

	"code.gitea.io/gitea/modules/assetfs"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// Vars represents variables to be render in golang templates
type Vars map[string]interface{}

// Merge merges another vars to the current, another Vars will override the current
func (vars Vars) Merge(another map[string]interface{}) Vars {
	for k, v := range another {
		vars[k] = v
	}
	return vars
}

// BaseVars returns all basic vars
func BaseVars() Vars {
	startTime := time.Now()
	return map[string]interface{}{
		"IsLandingPageHome":          setting.LandingPageURL == setting.LandingPageHome,
		"IsLandingPageExplore":       setting.LandingPageURL == setting.LandingPageExplore,
		"IsLandingPageOrganizations": setting.LandingPageURL == setting.LandingPageOrganizations,

		"ShowRegistrationButton":        setting.Service.ShowRegistrationButton,
		"ShowMilestonesDashboardPage":   setting.Service.ShowMilestonesDashboardPage,
		"ShowFooterVersion":             setting.Other.ShowFooterVersion,
		"DisableDownloadSourceArchives": setting.Repository.DisableDownloadSourceArchives,

		"EnableSwagger":      setting.API.EnableSwagger,
		"EnableOpenIDSignIn": setting.Service.EnableOpenIDSignIn,
		"PageStartTime":      startTime,
	}
}

func AssetFS() *assetfs.LayeredFS {
	return assetfs.Layered(CustomAssets(), BuiltinAssets())
}

func CustomAssets() *assetfs.Layer {
	return assetfs.Local("custom", setting.CustomPath, "templates")
}

func ListWebTemplateAssetNames(assets *assetfs.LayeredFS) ([]string, error) {
	files, err := assets.ListAllFiles(".", true)
	if err != nil {
		return nil, err
	}
	return util.SliceRemoveAllFunc(files, func(file string) bool {
		return strings.HasPrefix(file, "mail/") || !strings.HasSuffix(file, ".tmpl")
	}), nil
}

func ListMailTemplateAssetNames(assets *assetfs.LayeredFS) ([]string, error) {
	files, err := assets.ListAllFiles(".", true)
	if err != nil {
		return nil, err
	}
	return util.SliceRemoveAllFunc(files, func(file string) bool {
		return !strings.HasPrefix(file, "mail/") || !strings.HasSuffix(file, ".tmpl")
	}), nil
}
