// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package components

import (
	"code.gitea.io/gitea/modules/translation"
	g "maragu.dev/gomponents"
	gh "maragu.dev/gomponents/html"
)

func SearchInput(locale translation.Locale, value, placeholder string, disabled bool) g.Node {
	// Corresponds to templates/shared/search/input.tmpl

	if placeholder == "" {
		placeholder = string(locale.Tr("search.search"))
	}

	return gh.Input(
		gh.Type("search"),
		gh.Name("q"),
		gh.MaxLength("255"),
		g.Attr("spellcheck", "false"),
		gh.Value(value),
		gh.Placeholder(placeholder),
		If(disabled, gh.Disabled()),
	)
}
