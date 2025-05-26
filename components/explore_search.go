package components

import (
	g "maragu.dev/gomponents"
	gh "maragu.dev/gomponents/html"
)

func ExploreSearchMenu(data ExploreUsersPageProps, pageIsExploreUsers bool) g.Node {
	// Corresponds to templates/explore/search.tmpl

	tr := func(key string) string {
		return string(data.Locale.Tr(key))
	}

	return g.Group([]g.Node{
		gh.Div(
			gh.Class("ui small secondary filter menu tw-items-center tw-mx-0"),
			gh.Form(
				gh.Class("ui form ignore-dirty tw-flex-1"),
				If(pageIsExploreUsers,
					SearchCombo(data.Locale, data.Keyword, tr("search.user_kind")),
				),
				If(!pageIsExploreUsers,
					SearchCombo(data.Locale, data.Keyword, tr("search.org_kind")),
				),
			),
			gh.Div(
				gh.Class("ui small dropdown type jump item tw-mr-0"),
				gh.Span(
					gh.Class("text"),
					g.Text(tr("repo.issues.filter_sort")),
				),
				SVG("octicon-triangle-down", 14, "dropdown icon"),
				gh.Div(
					gh.Class("menu"),
					SortOption(data, "newest", tr("repo.issues.filter_sort.latest")),
					SortOption(data, "oldest", tr("repo.issues.filter_sort.oldest")),
					SortOption(data, "alphabetically", tr("repo.issues.label.filter_sort.alphabetically")),
					SortOption(data, "reversealphabetically", tr("repo.issues.label.filter_sort.reverse_alphabetically")),
				),
			),
		),
		gh.Div(gh.Class("divider")),
	})
}
