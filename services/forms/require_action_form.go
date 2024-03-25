// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// WIP RequireAction
package forms

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/binding"
)

// EditRequireActionForm form for admin to create Require action
type EditRequireActionForm struct {
	RepoID int64
	OrgID  int64
	Link   string
	Data   string
}

// Validate validates form fields
func (f *EditRequireActionForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}
