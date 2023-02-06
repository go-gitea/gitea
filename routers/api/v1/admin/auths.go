// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"net/http"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	auth_service "code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/ldap"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/auth/source/pam"
	"code.gitea.io/gitea/services/auth/source/smtp"
	"code.gitea.io/gitea/services/auth/source/sspi"

	xorm "xorm.io/xorm/convert"
)

// ListAuthSources returns list of existing authentication sources
func ListAuthSources(ctx *context.APIContext) {
	// swagger:operation GET /admin/auths admin adminAuthsSourcesList
	// ---
	// summary: List existing authentication sources
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/AuthSourcesList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	sources, err := auth.Sources()
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	result, err := convert.ToAuthSources(sources)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

// GetAuthSource get an authentication source by id
func GetAuthSource(ctx *context.APIContext) {
	// swagger:operation GET /admin/auths/{id} admin adminGetAuthSource
	// ---
	// summary: Get an authentication source
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the authentication source to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AuthSource"

	source, err := auth.GetSourceByID(ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.ServerError("auth.GetSourceByID", err)
		return
	}
	result, err := convert.ToAuthSource(source)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convert.ToAuthSource", err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

// CreateAuthSource creates new authentication source
func CreateAuthSource(ctx *context.APIContext) {
	// swagger:operation POST /admin/auths admin adminCreateAuthSource
	// ---
	// summary: Create new authentication source
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateAuthSource"
	// responses:
	//   "201":
	//     "$ref": "#/responses/AuthSource"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	authSource := web.GetForm(ctx).(*api.CreateAuthSource)
	var config xorm.Conversion
	var loginType auth.Type
	for key, val := range auth.Names {
		if authSource.Type == val {
			loginType = key
			switch key {
			case auth.LDAP:
				config = &ldap.Source{}
			case auth.SMTP:
				config = &smtp.Source{}
			case auth.PAM:
				config = &pam.Source{}
			case auth.DLDAP:
				config = &ldap.Source{}
			case auth.OAuth2:
				config = &oauth2.Source{}
			case auth.SSPI:
				config = &sspi.Source{}
			}
			break
		}
	}
	if loginType == 0 {
		ctx.Error(http.StatusBadRequest, "", "Authentication source type is invalid")
		return
	}
	if err := config.FromDB(authSource.Cfg); err != nil {
		ctx.InternalServerError(err)
		return
	}

	source := &auth.Source{
		Type:          loginType,
		Cfg:           config,
		Name:          authSource.Name,
		IsActive:      authSource.IsActive,
		IsSyncEnabled: authSource.IsSyncEnabled,
		CreatedUnix:   timeutil.TimeStampNow(),
		UpdatedUnix:   timeutil.TimeStampNow(),
	}
	if err := auth.CreateSource(source); err != nil {
		ctx.InternalServerError(err)
		return
	}
	result, err := convert.ToAuthSource(source)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusCreated, result)
}

// DeleteAuthSource delete an authentication source
func DeleteAuthSource(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/auths/{id} admin adminDeleteAuthSource
	// ---
	// summary: Delete an authentication source
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the authentication source to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	source, err := auth.GetSourceByID(ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.ServerError("auth.GetSourceByID", err)
		return
	}

	if err = auth_service.DeleteSource(source); err != nil {
		if auth.IsErrSourceInUse(err) {
			ctx.Error(http.StatusInternalServerError, "auth_service.DeleteSource", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "auth_service.DeleteSource", err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
