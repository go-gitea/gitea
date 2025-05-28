// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package components

import (
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/translation"
	g "maragu.dev/gomponents"
	gh "maragu.dev/gomponents/html"
)

type UserNameProps struct {
	User   *user.User
	Locale translation.Locale
}

func UserName(data UserNameProps) g.Node {
	return gh.A(
		gh.Class("text muted"),
		gh.Href(data.User.HomeLink()),
		g.Group([]g.Node{
			g.Text(data.User.Name),
			If(data.User.FullName != "" && data.User.FullName != data.User.Name,
				g.Text(" ("+data.User.FullName+")"),
			),
		}),
	)
}
