// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// ServerVersion
// swagger:response ServerVersion
type swaggerResponseServerVersion struct {
	// in:body
	Body api.ServerVersion `json:"body"`
}

// GitignoreTemplateList
// swagger:response GitignoreTemplateList
type swaggerResponseGitignoreTemplateList struct {
	// in:body
	Body []string `json:"body"`
}

// GitignoreTemplateInfo
// swagger:response GitignoreTemplateInfo
type swaggerResponseGitignoreTemplateInfo struct {
	// in:body
	Body api.GitignoreTemplateInfo `json:"body"`
}

// LicenseTemplateList
// swagger:response LicenseTemplateList
type swaggerResponseLicensesTemplateList struct {
	// in:body
	Body []api.LicensesTemplateListEntry `json:"body"`
}

// LicenseTemplateInfo
// swagger:response LicenseTemplateInfo
type swaggerResponseLicenseTemplateInfo struct {
	// in:body
	Body api.LicenseTemplateInfo `json:"body"`
}

// StringSlice
// swagger:response StringSlice
type swaggerResponseStringSlice struct {
	// in:body
	Body []string `json:"body"`
}
