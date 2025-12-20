package org

import (
	"net/http"

	group_model "code.gitea.io/gitea/models/group"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
)

func GetOrgGroups(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/groups organization orgListGroups
	// ---
	// summary: List an organization's root-level groups
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
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
	//     "$ref": "#/responses/GroupList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	var doerID int64
	if ctx.Doer != nil {
		doerID = ctx.Doer.ID
	}
	org, err := organization.GetOrgByName(ctx, ctx.PathParam("org"))
	if err != nil {
		if organization.IsErrOrgNotExist(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if !organization.HasOrgOrUserVisible(ctx, org.AsUser(), ctx.Doer) {
		ctx.APIErrorNotFound("HasOrgOrUserVisible", nil)
		return
	}
	groups, err := group_model.FindGroupsByCond(ctx, &group_model.FindGroupsOptions{
		ParentGroupID: 0,
		ActorID:       doerID,
		OwnerID:       org.ID,
	}, group_model.
		AccessibleGroupCondition(ctx.Doer, unit.TypeInvalid, perm.AccessModeRead))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	apiGroups := make([]*api.Group, len(groups))
	for i, group := range groups {
		apiGroups[i], err = convert.ToAPIGroup(ctx, group, ctx.Doer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}
	ctx.SetTotalCountHeader(int64(len(groups)))
	ctx.JSON(http.StatusOK, apiGroups)
}
