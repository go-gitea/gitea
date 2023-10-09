// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"fmt"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/convert"
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
		ctx.Error(http.StatusInternalServerError, "GetEmailAddresses", err)
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
		ctx.Error(http.StatusUnprocessableEntity, "", "Email list empty")
		return
	}

	emails := make([]*user_model.EmailAddress, len(form.Emails))
	for i := range form.Emails {
		emails[i] = &user_model.EmailAddress{
			UID:         ctx.Doer.ID,
			Email:       form.Emails[i],
			IsActivated: !setting.Service.RegisterEmailConfirm,
		}
	}

	if err := user_model.AddEmailAddresses(ctx, emails); err != nil {
		if user_model.IsErrEmailAlreadyUsed(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", "Email address has been used: "+err.(user_model.ErrEmailAlreadyUsed).Email)
		} else if user_model.IsErrEmailCharIsNotSupported(err) || user_model.IsErrEmailInvalid(err) {
			email := ""
			if typedError, ok := err.(user_model.ErrEmailInvalid); ok {
				email = typedError.Email
			}
			if typedError, ok := err.(user_model.ErrEmailCharIsNotSupported); ok {
				email = typedError.Email
			}

			errMsg := fmt.Sprintf("Email address %q invalid", email)
			ctx.Error(http.StatusUnprocessableEntity, "", errMsg)
		} else {
			ctx.Error(http.StatusInternalServerError, "AddEmailAddresses", err)
		}
		return
	}

	apiEmails := make([]*api.Email, len(emails))
	for i := range emails {
		apiEmails[i] = convert.ToEmail(emails[i])
	}
	ctx.JSON(http.StatusCreated, &apiEmails)
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

	emails := make([]*user_model.EmailAddress, len(form.Emails))
	for i := range form.Emails {
		emails[i] = &user_model.EmailAddress{
			Email: form.Emails[i],
			UID:   ctx.Doer.ID,
		}
	}

	if err := user_model.DeleteEmailAddresses(ctx, emails); err != nil {
		if user_model.IsErrEmailAddressNotExist(err) {
			ctx.Error(http.StatusNotFound, "DeleteEmailAddresses", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "DeleteEmailAddresses", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
