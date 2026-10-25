// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"net/http"

	quota_model "gitea.dev/models/quota"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/quota"
)

func GetStorageQuota(ctx *context.APIContext) {
	// swagger:operation GET /admin/quotas/{subject_type}/{id} admin adminGetStorageQuota
	// ---
	// summary: Get storage quota policy, usage and reservations
	// produces:
	// - application/json
	// parameters:
	// - name: subject_type
	//   in: path
	//   type: string
	//   enum: [repository, user, organization]
	//   required: true
	// - name: id
	//   in: path
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/StorageQuota"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "500":
	//     "$ref": "#/responses/error"
	status, err := quota.Status(ctx, quota_model.Subject(ctx.PathParam("subject_type")), ctx.PathParamInt64("id"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusOK, status)
}

func SetStorageQuota(ctx *context.APIContext) {
	// swagger:operation PUT /admin/quotas/{subject_type}/{id} admin adminSetStorageQuota
	// ---
	// summary: Replace storage quota overrides (null or omitted limits inherit defaults)
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: subject_type
	//   in: path
	//   type: string
	//   enum: [repository, user, organization]
	//   required: true
	// - name: id
	//   in: path
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/StorageQuotaOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/StorageQuota"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/conflict"
	//   "500":
	//     "$ref": "#/responses/error"
	subject, id := quota_model.Subject(ctx.PathParam("subject_type")), ctx.PathParamInt64("id")
	if _, err := quota.Status(ctx, subject, id); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	options := web.GetForm[*api.StorageQuotaOptions](ctx)
	if err := quota.UpdatePolicy(ctx, subject, id, *options); err != nil {
		if quota.IsQuotaError(err) {
			ctx.APIError(http.StatusConflict, err.Error())
		} else {
			ctx.APIErrorAuto(err)
		}
		return
	}
	GetStorageQuota(ctx)
}
