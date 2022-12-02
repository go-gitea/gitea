// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/binding"
)

// EditRunnerForm form for admin to create runner
type EditRunnerForm struct {
	Description  string
	CustomLabels string
}

// Validate validates form fields
func (f *EditRunnerForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// CreateRunnerForm form for admin to create runner
type CreateRunnerForm struct {
	Name string `binding:"Required"`
	Type string
}

// Validate validates form fields
func (f *CreateRunnerForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}
