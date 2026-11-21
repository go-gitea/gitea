// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	disclosure_model "gitea.dev/models/disclosure"
	"gitea.dev/modules/validation"
	"gitea.dev/modules/web/middleware"
)

func validateDisclosureSeverity(severity string, ctx *middleware.ValidateContext, errs validation.BindingErrors) validation.BindingErrors {
	if _, err := disclosure_model.ParseSeverity(severity); err != nil {
		errs = validation.AddValidationError(errs, "Severity", ctx.Locale.TrString("repo.security.invalid_severity"))
	}
	return errs
}

// NewDisclosureForm is the form for creating a security disclosure
type NewDisclosureForm struct {
	middleware.FormDefaultValidator
	Title            string `binding:"Required;MaxSize(255)"`
	Summary          string `binding:"MaxSize(4096)"`
	Content          string
	Severity         string `binding:"Required"`
	CVSSVector       string `binding:"MaxSize(128)"`
	CweIds           string
	CveID            string `binding:"MaxSize(64)"`
	References       string
	AffectedVersions string
	FixedVersions    string
	Publish          bool
}

// Validate implements the Form interface
func (f *NewDisclosureForm) Validate(ctx *middleware.ValidateContext, errs validation.BindingErrors) validation.BindingErrors {
	return validateDisclosureSeverity(f.Severity, ctx, errs)
}

// EditDisclosureForm is the form for editing a security disclosure
type EditDisclosureForm struct {
	middleware.FormDefaultValidator
	Title            string `binding:"Required;MaxSize(255)"`
	Summary          string `binding:"MaxSize(4096)"`
	Content          string
	Severity         string `binding:"Required"`
	CVSSVector       string `binding:"MaxSize(128)"`
	CweIds           string
	CveID            string `binding:"MaxSize(64)"`
	References       string
	AffectedVersions string
	FixedVersions    string
	Status           string `binding:"Required"`
}

// Validate implements the Form interface
func (f *EditDisclosureForm) Validate(ctx *middleware.ValidateContext, errs validation.BindingErrors) validation.BindingErrors {
	errs = validateDisclosureSeverity(f.Severity, ctx, errs)
	if _, err := disclosure_model.ParseStatus(f.Status); err != nil {
		errs = validation.AddValidationError(errs, "Status", ctx.Locale.TrString("repo.security.invalid_status"))
	}
	return errs
}
