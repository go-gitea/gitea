// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package components

import (
	g "maragu.dev/gomponents"
	gh "maragu.dev/gomponents/html"
)

func SearchInput(value, placeholder string, disabled bool) g.Node {
	// Corresponds to templates/shared/search/input.tmpl

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
