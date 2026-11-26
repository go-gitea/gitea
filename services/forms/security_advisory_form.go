// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"gitea.dev/modules/web/middleware"
)

type SecurityAdvisoryForm struct {
	middleware.FormDefaultValidator
	Summary        string `binding:"Required"`
	Content        string `binding:"Required"`
	CveID          string `form:"cve_id"`
	Severity       string
	CvssVector     string `form:"cvss_vector"`
	CweIDs         string `form:"cwe_ids"`
	LabelIDs       string `form:"label_ids"`
	ContentVersion int

	Ecosystem              []string
	PackageName            []string
	VulnerableVersionRange []string
	PatchedVersions        []string
	VulnerableFunctions    []string

	CreditUser []string
	CreditType []string
}

type SecurityAdvisoryCommentForm struct {
	middleware.FormDefaultValidator
	Content        string `binding:"Required"`
	IsConfidential bool
}
