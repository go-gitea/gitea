// Package v1 registers API v1 routes.
// Modified to add Actions permissions endpoints. Modified by LAC | Ludwig investing
package v1

import (
	"code.gitea.io/gitea/routers/api/v1/actions"
	"code.gitea.io/gitea/routers/api/v1/admin"
	"code.gitea.io/gitea/routers/api/v1/misc"
	"code.gitea.io/gitea/routers/api/v1/org"
	"code.gitea.io/gitea/routers/api/v1/repo"
	"code.gitea.io/gitea/routers/api/v1/user"
	"github.com/go-chi/chi/v5"
)

// Routes registers all API v1 routes.
// Modified to add Actions permissions endpoints. Modified by LAC | Ludwig investing
func Routes() *chi.Mux {
	r := chi.NewRouter()

	r.Route("/repos/{owner}/{repo}", func(r chi.Router) {
		// ... existing routes ...
		// Actions permissions endpoints
		// Added to support configurable permissions. Modified by LAC | Ludwig investing
		r.Get("/actions/permissions", repo.GetRepoActionsPermissions)
		r.Put("/actions/permissions", repo.UpdateRepoActionsPermissions)
	})

	r.Route("/orgs/{org}", func(r chi.Router) {
		// ... existing routes ...
		// Actions permissions endpoints
		// Added to support configurable permissions. Modified by LAC | Ludwig investing
		r.Get("/actions/permissions", org.GetOrgActionsPermissions)
		r.Put("/actions/permissions", org.UpdateOrgActionsPermissions)
		r.Get("/actions/permissions/repositories", org.ListCrossRepoAccess)
		r.Put("/actions/permissions/repositories/{repo_id}", org.SetCrossRepoAccess)
	})

	r.Route("/packages/{owner}/{type}/{name}", func(r chi.Router) {
		// ... existing routes ...
		// Package actions access endpoints
		// Added to support package access for Actions. Modified by LAC | Ludwig investing
		r.Get("/-/actions/access", repo.GetPackageActionsAccess)
		r.Put("/-/actions/access", repo.SetPackageActionsAccess)
	})

	return r
}
