// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package components

import (
	"code.gitea.io/gitea/modules/translation"
	g "maragu.dev/gomponents"
	gh "maragu.dev/gomponents/html"
)

func SearchCombo(locale translation.Locale, value, placeholder string) g.Node {
	// Corresponds to templates/shared/search/combo.tmpl

	disabled := false
	return gh.Div(
		gh.Class("ui small fluid action input"),
		SearchInput(value, placeholder, disabled),
		// TODO SearchModeDropdown
		SearchButton(disabled, ""),
	)
}
