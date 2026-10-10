// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"gitea.dev/modules/web/middleware"
)

// SecurityAdvisoryForm is the form for creating, reporting and editing a security advisory,
// the vulnerability and credit fields are repeated once per row
type SecurityAdvisoryForm struct {
	middleware.FormDefaultValidator
	Summary      string `binding:"Required"`
	Content      string `binding:"Required"`
	CveID        string `form:"cve_id"`
	Severity     string
	CvssV3Vector string `form:"cvss_v3_vector"`
	CvssV4Vector string `form:"cvss_v4_vector"`
	CweIDs       string `form:"cwe_ids"`
	LabelIDs     string `form:"label_ids"`
	// ContentVersion detects concurrent edits
	ContentVersion int

	Ecosystem              []string
	PackageName            []string
	VulnerableVersionRange []string
	PatchedVersions        []string
	VulnerableFunctions    []string

	CreditUser []string
	CreditType []string
}

// SecurityAdvisoryCommentForm is the form for a comment on a security advisory
type SecurityAdvisoryCommentForm struct {
	middleware.FormDefaultValidator
	Content    string `binding:"Required"`
	IsInternal bool
}
