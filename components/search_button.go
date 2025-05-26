package components

import (
	g "maragu.dev/gomponents"
	gh "maragu.dev/gomponents/html"
)

func SearchButton(disabled bool, tooltip string) g.Node {
	// Corresponds to templates/shared/search/button.tmpl

	class := "ui icon button"
	if disabled {
		class += " disabled"
	}

	btn := gh.Button(
		gh.Type("submit"),
		gh.Class(class),
		SVG("octicon-search", 16),
	)

	if tooltip != "" {
		btn = gh.Button(
			gh.Type("submit"),
			gh.Class(class),
			g.Attr("data-tooltip-content", tooltip),
			SVG("octicon-search", 16),
		)
	}

	return btn
}
