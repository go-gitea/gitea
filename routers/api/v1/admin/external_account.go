// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	stdCtx "context"
	"net/http"
	"slices"
	"strconv"

	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	"gitea.dev/services/externalaccount"
)

func ListUserExternalAccounts(ctx *context.APIContext) {
	// swagger:operation GET /admin/users/{username}/external-accounts admin adminListUserExternalAccounts
	// ---
	// summary: List the external accounts whose migrated content is attributed to a user
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/ExternalAccountList"
	//   default:
	//     "$ref": "#/responses/error"

	listOptions := utils.GetListOptions(ctx)
	links, total, err := db.FindAndCount[user_model.ExternalAccountLink](ctx, user_model.FindExternalAccountLinksOptions{
		ListOptions: listOptions,
		UserID:      ctx.ContextUser.ID,
	})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	accounts := make([]api.ExternalAccount, len(links))
	for i, link := range links {
		accounts[i] = api.ExternalAccount{Service: link.ServiceType.Name(), ExternalID: link.ExternalID}
	}
	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, accounts)
}

func LinkUserExternalAccount(ctx *context.APIContext) {
	// swagger:operation PUT /admin/users/{username}/external-accounts/{service}/{external_id} admin adminLinkUserExternalAccount
	// ---
	// summary: Attribute content migrated from an external account to a user
	// description: Attributes migrated issues, pull requests, comments, reviews, reactions and releases of the
	//   external account that are not yet attributed to a user, now and in future migrations. The account is
	//   matched on every instance of the service, and services that do not record user IDs, like CodeCommit,
	//   have nothing to attribute. Linking an account that is linked to another user returns 409.
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// - name: service
	//   in: path
	//   description: name of the git service, e.g. github
	//   type: string
	//   required: true
	// - name: external_id
	//   in: path
	//   description: the user's ID on the git service
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   default:
	//     "$ref": "#/responses/error"

	updateUserExternalAccount(ctx, externalaccount.LinkMigratedAccount)
}

func UnlinkUserExternalAccount(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/users/{username}/external-accounts/{service}/{external_id} admin adminUnlinkUserExternalAccount
	// ---
	// summary: Stop attributing content migrated from an external account to a user
	// description: Only affects future migrations, already attributed content keeps the user. Future
	//   migrations still attribute the account to a user who signed in with it.
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// - name: service
	//   in: path
	//   description: name of the git service, e.g. github
	//   type: string
	//   required: true
	// - name: external_id
	//   in: path
	//   description: the user's ID on the git service
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   default:
	//     "$ref": "#/responses/error"

	updateUserExternalAccount(ctx, externalaccount.UnlinkMigratedAccount)
}

func updateUserExternalAccount(ctx *context.APIContext, update func(stdCtx.Context, *user_model.User, api.GitServiceType, int64) error) {
	tp := convert.ToGitServiceType(ctx.PathParam("service"))
	externalID, err := strconv.ParseInt(ctx.PathParam("external_id"), 10, 64)
	if !slices.Contains(api.SupportedFullGitService, tp) || err != nil || externalID <= 0 {
		ctx.APIError(http.StatusUnprocessableEntity, "the service must support migrations and the external ID must be positive")
		return
	}
	if err := update(ctx, ctx.ContextUser, tp, externalID); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
