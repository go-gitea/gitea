package components

import (
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	templates "code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/services/context"
	g "maragu.dev/gomponents"
	gh "maragu.dev/gomponents/html"
)

type UserListProps struct {
	Users            []*user.User
	IsSigned         bool
	PageIsAdminUsers bool
	Locale           translation.Locale
	Context          *context.Context
}

func UserList(data UserListProps) g.Node {
	tr := func(key string, args ...any) string {
		return string(data.Locale.Tr(key, args...))
	}

	if len(data.Users) == 0 {
		return gh.Div(
			gh.Class("flex-list"),
			gh.Div(
				gh.Class("flex-item"),
				g.Text(tr("search.no_results")),
			),
		)
	}

	return gh.Div(
		gh.Class("flex-list"),
		g.Map(data.Users, func(u *user.User) g.Node {
			utils := templates.NewAvatarUtils(data.Context)

			return gh.Div(
				gh.Class("flex-item tw-items-center"),
				gh.Div(
					gh.Class("flex-item-leading"),
					g.Raw(string(utils.Avatar(u, 48))),
				),
				gh.Div(
					gh.Class("flex-item-main"),
					gh.Div(
						gh.Class("flex-item-title"),
						UserName(UserNameProps{
							Locale: data.Locale,
							User:   u,
						}),
						If(u.Visibility.IsPrivate(),
							gh.Span(
								gh.Class("ui basic tiny label"),
								g.Text(tr("repo.desc.private")),
							),
						),
					),
					gh.Div(
						gh.Class("flex-item-body"),
						If(u.Location != "",
							gh.Span(
								gh.Class("flex-text-inline"),
								SVG("octicon-location", 16),
								g.Text(u.Location),
							),
						),
						If(u.Email != "" && (data.PageIsAdminUsers || (setting.UI.ShowUserEmail && data.IsSigned && !u.KeepEmailPrivate)),
							gh.Span(
								gh.Class("flex-text-inline"),
								SVG("octicon-mail", 16),
								gh.A(
									gh.Href("mailto:"+u.Email),
									g.Text(u.Email),
								),
							),
						),
						gh.Span(
							gh.Class("flex-text-inline"),
							SVG("octicon-calendar", 16),
							g.Raw(tr("user.joined_on", templates.NewDateUtils().AbsoluteShort(u.CreatedUnix))),
						),
					),
				),
			)
		}),
	)
}
