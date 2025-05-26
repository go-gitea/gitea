package components

import (
	"fmt"
	"net/url"

	g "maragu.dev/gomponents"
	gh "maragu.dev/gomponents/html"
)

func SortOption(data ExploreUsersPageProps, sortType, label string) g.Node {
	active := ""
	if data.SortType == sortType {
		active = "active "
	}
	return gh.A(
		gh.Class(active+"item"),
		gh.Href(fmt.Sprintf("?sort=%s&q=%s", sortType, url.QueryEscape(data.Keyword))),
		g.Text(label),
	)
}
