// Package setting registers organization settings routes.
// Modified to add Actions settings page. Modified by LAC | Ludwig investing
package setting

import (
	"code.gitea.io/gitea/modules/context"
	"github.com/go-chi/chi/v5"
)

// Routes registers organization settings routes.
// Modified to add Actions settings page. Modified by LAC | Ludwig investing
func Routes() *chi.Mux {
	r := chi.NewRouter()

	r.Get("/", func(ctx *context.Context) {
		ctx.Redirect(ctx.Org.OrgLink + "/settings/options")
	})
	r.Get("/options", Options)
	r.Post("/options", OptionsPost)
	r.Get("/members", Members)
	r.Post("/members", MembersPost)
	r.Get("/teams", Teams)
	r.Post("/teams", TeamsPost)
	r.Get("/webhooks", Webhooks)
	r.Post("/webhooks", WebhooksPost)
	r.Get("/secrets", Secrets)
	r.Post("/secrets", SecretsPost)
	r.Get("/actions", ActionsSettings)          // Added for Actions permissions. Modified by LAC | Ludwig investing
	r.Post("/actions", ActionsSettingsPost)     // Added for Actions permissions. Modified by LAC | Ludwig investing

	return r
}
