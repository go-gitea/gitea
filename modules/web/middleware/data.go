// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"context"
	"time"

	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
)

const ContextDataKeySignedUser = "SignedUser"

func GetContextData(c context.Context) reqctx.ContextData {
	if rc := reqctx.GetRequestDataStore(c); rc != nil {
		return rc.GetData()
	}
	return nil
}

func CommonTemplateContextData() reqctx.ContextData {
	return reqctx.ContextData{
		"IsLandingPageOrganizations": setting.LandingPageURL == setting.LandingPageOrganizations,

		"ShowRegistrationButton":        setting.Service.ShowRegistrationButton,
		"ShowMilestonesDashboardPage":   setting.Service.ShowMilestonesDashboardPage,
		"ShowFooterVersion":             setting.Other.ShowFooterVersion,
		"DisableDownloadSourceArchives": setting.Repository.DisableDownloadSourceArchives,

		"EnableSwagger":      setting.API.EnableSwagger,
		"EnableOpenIDSignIn": setting.Service.EnableOpenIDSignIn,
		"PageStartTime":      time.Now(),

		"RunModeIsProd": setting.IsProd,
	}
}
