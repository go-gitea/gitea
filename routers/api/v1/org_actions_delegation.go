// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1

import (
	"errors"
	"net/http"

	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	"gitea.dev/models/unit"
	"gitea.dev/modules/setting"
	"gitea.dev/services/context"
)

// reqOrgActionsManagement admits an organization's owners, and the members of
// a team its owners have given write access to the Actions unit.
//
// Why this is in the fork rather than in RiaHub, which is where everything
// that can live there does: RiaHub serves the organization's runner surface
// itself, but every call it makes is forwarded here with the Doer's own
// token, and this is what refuses it. RiaHub deciding differently would mean
// RiaHub holding a credential able to register runners and read secrets for
// any organization, used on a non-owner's behalf. The decision has to be made
// where it is enforced.
//
// Why it is not a flag of its own: legacy carried this as a per-team boolean
// added by a migration of its own (its v299, `OrgWorkflowManagement`). A
// column means a migration, a migration means an entry in the version table,
// and an entry in the version table is something every upstream merge has to
// reconcile. Gitea already has the concept -- a team with an access mode on
// the Actions unit -- so this reads what is there instead. Nothing is added
// to the schema and nothing is added to the version table.
//
// The delegation is deliberate on the owners' part either way: a team has no
// access to the Actions unit until somebody gives it some, and write is the
// mode, not read. A team that may only read workflow runs is not thereby
// allowed to register a runner or read a secret.
//
// Scope is the organization's own Actions settings -- its runners, secrets
// and variables, which are the three things `addActionsRoutes` guards with
// this. It does not widen anything else: ownership of the organization, its
// members, its teams and its repositories are all still owner-only.
func reqOrgActionsManagement() func(ctx *context.APIContext) {
	return func(ctx *context.APIContext) {
		if ctx.IsUserSiteAdmin() {
			return
		}

		if ctx.Org.Organization == nil {
			setting.PanicInDevOrTesting("reqOrgActionsManagement: unprepared context")
			ctx.APIErrorInternal(errors.New("reqOrgActionsManagement: unprepared context"))
			return
		}
		orgID := ctx.Org.Organization.ID

		isOwner, err := organization.IsOrganizationOwner(ctx, orgID, ctx.Doer.ID)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		if isOwner {
			return
		}

		teams, err := organization.GetUserOrgTeams(ctx, orgID, ctx.Doer.ID)
		if err != nil {
			// Fail closed. An owner is already past this point, so the only
			// caller reaching here is one whose delegation could not be read,
			// and admitting them on a database error would make an outage
			// into a widening.
			ctx.APIErrorInternal(err)
			return
		}
		for _, team := range teams {
			if team.UnitAccessMode(ctx, unit.TypeActions) >= perm.AccessModeWrite {
				return
			}
		}

		ctx.APIError(http.StatusForbidden, "Must be an organization owner, or a member of a team with write access to Actions")
	}
}
