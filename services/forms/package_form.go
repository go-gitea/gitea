// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"net/http"

	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/context"

	"gitea.com/go-chi/binding"
)

type PackageCleanupRuleForm struct {
	ID            int64
	Enabled       bool
	Type          string `binding:"Required;In(alpine,arch,cargo,chef,composer,conan,conda,container,cran,debian,generic,go,helm,maven,npm,nuget,pub,pypi,rpm,rubygems,swift,vagrant)"`
	KeepCount     int    `binding:"In(0,1,5,10,25,50,100)"`
	KeepPattern   string `binding:"RegexPattern"`
	RemoveDays    int    `binding:"In(0,7,14,30,60,90,180)"`
	RemovePattern string `binding:"RegexPattern"`
	MatchFullName bool
	Action        string `binding:"Required;In(save,remove)"`
}

func (f *PackageCleanupRuleForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}
