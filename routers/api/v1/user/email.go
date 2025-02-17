// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"fmt"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	user_service "code.gitea.io/gitea/services/user"
)

// ListEmails list all of the authenticated user's email addresses
// see https://github.com/gogits/go-gogs-client/wiki/Users-Emails#list-email-addresses-for-a-user
func ListEmails(ctx *context.APIContext) {
	// swagger:operation GET /user/emails user userListEmails
	// ---
	// summary: List the authenticated user's email addresses
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/EmailList"

	emails, err := user_model.GetEmailAddresses(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	apiEmails := make([]*api.Email, len(emails))
	for i := range emails {
		apiEmails[i] = convert.ToEmail(emails[i])
	}
	ctx.JSON(http.StatusOK, &apiEmails)
}

// AddEmail add an email address
func AddEmail(ctx *context.APIContext) {
	// swagger:operation POST /user/emails user userAddEmail
	// ---
	// summary: Add email addresses
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateEmailOption"
	// responses:
	//   '201':
	//     "$ref": "#/responses/EmailList"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateEmailOption)
	if len(form.Emails) == 0 {
		ctx.APIError(http.StatusUnprocessableEntity, "Email list empty")
		return
	}

	if err := user_service.AddEmailAddresses(ctx, ctx.Doer, form.Emails); err != nil {
		if user_model.IsErrEmailAlreadyUsed(err) {
			ctx.APIError(http.StatusUnprocessableEntity, "Email address has been used: "+err.(user_model.ErrEmailAlreadyUsed).Email)
		} else if user_model.IsErrEmailCharIsNotSupported(err) || user_model.IsErrEmailInvalid(err) {
			email := ""
			if typedError, ok := err.(user_model.ErrEmailInvalid); ok {
				email = typedError.Email
			}
			if typedError, ok := err.(user_model.ErrEmailCharIsNotSupported); ok {
				email = typedError.Email
			}

			errMsg := fmt.Sprintf("Email address %q invalid", email)
			ctx.APIError(http.StatusUnprocessableEntity, errMsg)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	emails, err := user_model.GetEmailAddresses(ctx, ctx.Doer.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	apiEmails := make([]*api.Email, 0, len(emails))
	for _, email := range emails {
		apiEmails = append(apiEmails, convert.ToEmail(email))
	}
	ctx.JSON(http.StatusCreated, apiEmails)
}

// DeleteEmail delete email
func DeleteEmail(ctx *context.APIContext) {
	// swagger:operation DELETE /user/emails user userDeleteEmail
	// ---
	// summary: Delete email addresses
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/DeleteEmailOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	form := web.GetForm(ctx).(*api.DeleteEmailOption)
	if len(form.Emails) == 0 {
		ctx.Status(http.StatusNoContent)
		return
	}

	if err := user_service.DeleteEmailAddresses(ctx, ctx.Doer, form.Emails); err != nil {
		if user_model.IsErrEmailAddressNotExist(err) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
