// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"time"

	"code.gitea.io/gitea/modules/setting"
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
	var startTime = time.Now()
	return map[string]interface{}{
		"IsLandingPageHome": setting.LandingPageURL == setting.LandingPageHome,
		"IsLandingPageExplore": setting.LandingPageURL == setting.LandingPageExplore,
		"IsLandingPageOrganizations": setting.LandingPageURL == setting.LandingPageOrganizations,

		"ShowRegistrationButton": setting.Service.ShowRegistrationButton,
		"ShowMilestonesDashboardPage": setting.Service.ShowMilestonesDashboardPage,
		"ShowFooterBranding": setting.ShowFooterBranding,
		"ShowFooterVersion": setting.ShowFooterVersion,

		"EnableSwagger": setting.API.EnableSwagger,
		"EnableOpenIDSignIn": setting.Service.EnableOpenIDSignIn,
		"PageStartTime": startTime,
		"TmplLoadTimes": func() string {
			return time.Since(startTime).String()
		},
	}
}