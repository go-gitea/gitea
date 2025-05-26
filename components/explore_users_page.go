package components

import (
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/services/context"
	g "maragu.dev/gomponents"
	gh "maragu.dev/gomponents/html"
)

type ExploreUsersPageProps struct {
	Title    string
	Locale   translation.Locale
	Keyword  string
	SortType string
	Users    []*user.User
	// ContextUser          *user.User
	Context  *context.Context
	IsSigned bool
}

func ExploreUsersPage(data ExploreUsersPageProps) g.Node {
	// pageIsExplore := true
	pageIsExploreUsers := true

	head, err := data.Context.HTMLPartial(200, "base/head")
	if err != nil {
		panic("could not render head")
	}

	footer, err := data.Context.HTMLPartial(200, "base/footer")
	if err != nil {
		panic("could not render footer")
	}

	return g.Group([]g.Node{
		g.Raw(head),
		gh.Div(
			gh.Role("main"),
			gh.Aria("label", data.Title),
			gh.Class("page-content explore users"),
			ExploreNavbar(ExploreNavbarProps{
				Locale:             data.Locale,
				PageIsExploreUsers: pageIsExploreUsers,
			}),
			gh.Div(
				gh.Class("ui container"),
				ExploreSearchMenu(data, true),
				UserList(UserListProps{
					// ContextUser:      data.ContextUser,
					Context:          data.Context,
					Users:            data.Users,
					IsSigned:         data.IsSigned,
					Locale:           data.Locale,
					PageIsAdminUsers: false,
				}),
				// Pagination(data),
			),
		),
		g.Raw(footer),
	})
}
