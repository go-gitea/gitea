// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package components

import (
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/translation"
	g "maragu.dev/gomponents"
	gh "maragu.dev/gomponents/html"
)

type ExploreNavbarProps struct {
	PageIsExploreRepositories   bool
	UsersPageIsDisabled         bool
	AppSubURL                   string
	PageIsExploreUsers          bool
	PageIsExploreCode           bool
	IsRepoIndexerEnabled        bool
	CodePageIsDisabled          bool
	PageIsExploreOrganizations  bool
	OrganizationsPageIsDisabled bool
	Locale                      translation.Locale
}

func ExploreNavbar(data ExploreNavbarProps) g.Node {
	tr := func(key string) string {
		return string(data.Locale.Tr(key))
	}

	isCodeGlobalDisabled := unit.TypeCode.UnitGlobalDisabled()

	return g.El("overflow-menu",
		gh.Class("ui secondary pointing tabular top attached borderless menu secondary-nav"),
		gh.Div(
			gh.Class("overflow-menu-items tw-justify-center"),
			gh.A(
				gh.Class(classIf(data.PageIsExploreRepositories, "active ")+"item"),
				gh.Href(data.AppSubURL+"/explore/repos"),
				SVG("octicon-repo"),
				g.Text(" "+tr("explore.repos")),
			),
			If(!data.UsersPageIsDisabled,
				gh.A(
					gh.Class(classIf(data.PageIsExploreUsers, "active ")+"item"),
					gh.Href(data.AppSubURL+"/explore/users"),
					SVG("octicon-person"),
					g.Text(" "+tr("explore.users")),
				),
			),
			If(!data.OrganizationsPageIsDisabled,
				gh.A(
					gh.Class(classIf(data.PageIsExploreOrganizations, "active ")+"item"),
					gh.Href(data.AppSubURL+"/explore/organizations"),
					SVG("octicon-organization"),
					g.Text(" "+tr("explore.organizations")),
				),
			),
			If(!isCodeGlobalDisabled && data.IsRepoIndexerEnabled && !data.CodePageIsDisabled,
				gh.A(
					gh.Class(classIf(data.PageIsExploreCode, "active ")+"item"),
					gh.Href(data.AppSubURL+"/explore/code"),
					SVG("octicon-code"),
					g.Text(" "+tr("explore.code")),
				),
			),
		),
	)
}
