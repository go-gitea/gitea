// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import "gitea.dev/modules/web/middleware"

type PackageCleanupRuleForm struct {
	middleware.FormDefaultValidator
	ID            int64
	Enabled       bool
	Type          string `binding:"Required;In(alpine,arch,cargo,chef,composer,conan,conda,container,cran,debian,generic,go,helm,maven,npm,nuget,pub,pypi,rpm,rubygems,swift,terraform,vagrant)"`
	KeepCount     int    `binding:"In(0,1,5,10,25,50,100)"`
	KeepPattern   string `binding:"RegexPattern"`
	RemoveDays    int    `binding:"In(0,7,14,30,60,90,180)"`
	RemovePattern string `binding:"RegexPattern"`
	MatchFullName bool
	Action        string `binding:"Required;In(save,remove)"`
}
