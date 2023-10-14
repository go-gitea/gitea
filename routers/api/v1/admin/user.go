// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/user"
	"code.gitea.io/gitea/routers/api/v1/utils"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/mailer"
	user_service "code.gitea.io/gitea/services/user"
)

func parseAuthSource(ctx *context.APIContext, u *user_model.User, sourceID int64, loginName string) {
	if sourceID == 0 {
		return
	}

	source, err := auth.GetSourceByID(ctx, sourceID)
	if err != nil {
		if auth.IsErrSourceNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "auth.GetSourceByID", err)
		}
		return
	}

	u.LoginType = source.Type
	u.LoginSource = source.ID
	u.LoginName = loginName
}

// CreateUser create a user
func CreateUser(ctx *context.APIContext) {
	// swagger:operation POST /admin/users admin adminCreateUser
	// ---
	// summary: Create a user
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateUserOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/User"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateUserOption)

	u := &user_model.User{
		Name:               form.Username,
		FullName:           form.FullName,
		Email:              form.Email,
		Passwd:             form.Password,
		MustChangePassword: true,
		LoginType:          auth.Plain,
	}
	if form.MustChangePassword != nil {
		u.MustChangePassword = *form.MustChangePassword
	}

	parseAuthSource(ctx, u, form.SourceID, form.LoginName)
	if ctx.Written() {
		return
	}
	if !password.IsComplexEnough(form.Password) {
		err := errors.New("PasswordComplexity")
		ctx.Error(http.StatusBadRequest, "PasswordComplexity", err)
		return
	}
	pwned, err := password.IsPwned(ctx, form.Password)
	if pwned {
		if err != nil {
			log.Error(err.Error())
		}
		ctx.Error(http.StatusBadRequest, "PasswordPwned", errors.New("PasswordPwned"))
		return
	}

	overwriteDefault := &user_model.CreateUserOverwriteOptions{
		IsActive: util.OptionalBoolTrue,
	}

	if form.Restricted != nil {
		overwriteDefault.IsRestricted = util.OptionalBoolOf(*form.Restricted)
	}

	if form.Visibility != "" {
		visibility := api.VisibilityModes[form.Visibility]
		overwriteDefault.Visibility = &visibility
	}

	// Update the user creation timestamp. This can only be done after the user
	// record has been inserted into the database; the insert intself will always
	// set the creation timestamp to "now".
	if form.Created != nil {
		u.CreatedUnix = timeutil.TimeStamp(form.Created.Unix())
		u.UpdatedUnix = u.CreatedUnix
	}

	if err := user_model.CreateUser(ctx, u, overwriteDefault); err != nil {
		if user_model.IsErrUserAlreadyExist(err) ||
			user_model.IsErrEmailAlreadyUsed(err) ||
			db.IsErrNameReserved(err) ||
			db.IsErrNameCharsNotAllowed(err) ||
			user_model.IsErrEmailCharIsNotSupported(err) ||
			user_model.IsErrEmailInvalid(err) ||
			db.IsErrNamePatternNotAllowed(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateUser", err)
		}
		return
	}
	log.Trace("Account created by admin (%s): %s", ctx.Doer.Name, u.Name)

	// Send email notification.
	if form.SendNotify {
		mailer.SendRegisterNotifyMail(u)
	}
	ctx.JSON(http.StatusCreated, convert.ToUser(ctx, u, ctx.Doer))
}

// EditUser api for modifying a user's information
func EditUser(ctx *context.APIContext) {
	// swagger:operation PATCH /admin/users/{username} admin adminEditUser
	// ---
	// summary: Edit an existing user
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user to edit
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditUserOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/User"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.EditUserOption)

	parseAuthSource(ctx, ctx.ContextUser, form.SourceID, form.LoginName)
	if ctx.Written() {
		return
	}

	if len(form.Password) != 0 {
		if len(form.Password) < setting.MinPasswordLength {
			ctx.Error(http.StatusBadRequest, "PasswordTooShort", fmt.Errorf("password must be at least %d characters", setting.MinPasswordLength))
			return
		}
		if !password.IsComplexEnough(form.Password) {
			err := errors.New("PasswordComplexity")
			ctx.Error(http.StatusBadRequest, "PasswordComplexity", err)
			return
		}
		pwned, err := password.IsPwned(ctx, form.Password)
		if pwned {
			if err != nil {
				log.Error(err.Error())
			}
			ctx.Error(http.StatusBadRequest, "PasswordPwned", errors.New("PasswordPwned"))
			return
		}
		if ctx.ContextUser.Salt, err = user_model.GetUserSalt(); err != nil {
			ctx.Error(http.StatusInternalServerError, "UpdateUser", err)
			return
		}
		if err = ctx.ContextUser.SetPassword(form.Password); err != nil {
			ctx.InternalServerError(err)
			return
		}
	}

	if form.MustChangePassword != nil {
		ctx.ContextUser.MustChangePassword = *form.MustChangePassword
	}

	ctx.ContextUser.LoginName = form.LoginName

	if form.FullName != nil {
		ctx.ContextUser.FullName = *form.FullName
	}
	var emailChanged bool
	if form.Email != nil {
		email := strings.TrimSpace(*form.Email)
		if len(email) == 0 {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("email is not allowed to be empty string"))
			return
		}

		if err := user_model.ValidateEmail(email); err != nil {
			ctx.InternalServerError(err)
			return
		}

		emailChanged = !strings.EqualFold(ctx.ContextUser.Email, email)
		ctx.ContextUser.Email = email
	}
	if form.Website != nil {
		ctx.ContextUser.Website = *form.Website
	}
	if form.Location != nil {
		ctx.ContextUser.Location = *form.Location
	}
	if form.Description != nil {
		ctx.ContextUser.Description = *form.Description
	}
	if form.Active != nil {
		ctx.ContextUser.IsActive = *form.Active
	}
	if len(form.Visibility) != 0 {
		ctx.ContextUser.Visibility = api.VisibilityModes[form.Visibility]
	}
	if form.Admin != nil {
		ctx.ContextUser.IsAdmin = *form.Admin
	}
	if form.AllowGitHook != nil {
		ctx.ContextUser.AllowGitHook = *form.AllowGitHook
	}
	if form.AllowImportLocal != nil {
		ctx.ContextUser.AllowImportLocal = *form.AllowImportLocal
	}
	if form.MaxRepoCreation != nil {
		ctx.ContextUser.MaxRepoCreation = *form.MaxRepoCreation
	}
	if form.AllowCreateOrganization != nil {
		ctx.ContextUser.AllowCreateOrganization = *form.AllowCreateOrganization
	}
	if form.ProhibitLogin != nil {
		ctx.ContextUser.ProhibitLogin = *form.ProhibitLogin
	}
	if form.Restricted != nil {
		ctx.ContextUser.IsRestricted = *form.Restricted
	}

	if err := user_model.UpdateUser(ctx, ctx.ContextUser, emailChanged); err != nil {
		if user_model.IsErrEmailAlreadyUsed(err) ||
			user_model.IsErrEmailCharIsNotSupported(err) ||
			user_model.IsErrEmailInvalid(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "UpdateUser", err)
		}
		return
	}
	log.Trace("Account profile updated by admin (%s): %s", ctx.Doer.Name, ctx.ContextUser.Name)

	ctx.JSON(http.StatusOK, convert.ToUser(ctx, ctx.ContextUser, ctx.Doer))
}

// DeleteUser api for deleting a user
func DeleteUser(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/users/{username} admin adminDeleteUser
	// ---
	// summary: Delete a user
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user to delete
	//   type: string
	//   required: true
	// - name: purge
	//   in: query
	//   description: purge the user from the system completely
	//   type: boolean
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	if ctx.ContextUser.IsOrganization() {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("%s is an organization not a user", ctx.ContextUser.Name))
		return
	}

	// admin should not delete themself
	if ctx.ContextUser.ID == ctx.Doer.ID {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("you cannot delete yourself"))
		return
	}

	if err := user_service.DeleteUser(ctx, ctx.ContextUser, ctx.FormBool("purge")); err != nil {
		if models.IsErrUserOwnRepos(err) ||
			models.IsErrUserHasOrgs(err) ||
			models.IsErrUserOwnPackages(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteUser", err)
		}
		return
	}
	log.Trace("Account deleted by admin(%s): %s", ctx.Doer.Name, ctx.ContextUser.Name)

	ctx.Status(http.StatusNoContent)
}

// CreatePublicKey api for creating a public key to a user
func CreatePublicKey(ctx *context.APIContext) {
	// swagger:operation POST /admin/users/{username}/keys admin adminCreatePublicKey
	// ---
	// summary: Add a public key on behalf of a user
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of the user
	//   type: string
	//   required: true
	// - name: key
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateKeyOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/PublicKey"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateKeyOption)

	user.CreateUserPublicKey(ctx, *form, ctx.ContextUser.ID)
}

// DeleteUserPublicKey api for deleting a user's public key
func DeleteUserPublicKey(ctx *context.APIContext) {
	// swagger:operation DELETE /admin/users/{username}/keys/{id} admin adminDeleteUserPublicKey
	// ---
	// summary: Delete a user's public key
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the key to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if err := asymkey_service.DeletePublicKey(ctx, ctx.ContextUser, ctx.ParamsInt64(":id")); err != nil {
		if asymkey_model.IsErrKeyNotExist(err) {
			ctx.NotFound()
		} else if asymkey_model.IsErrKeyAccessDenied(err) {
			ctx.Error(http.StatusForbidden, "", "You do not have access to this key")
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteUserPublicKey", err)
		}
		return
	}
	log.Trace("Key deleted by admin(%s): %s", ctx.Doer.Name, ctx.ContextUser.Name)

	ctx.Status(http.StatusNoContent)
}

// SearchUsers API for getting information of the users according the filter conditions
func SearchUsers(ctx *context.APIContext) {
	// swagger:operation GET /admin/users admin adminSearchUsers
	// ---
	// summary: Search users according filter conditions
	// produces:
	// - application/json
	// parameters:
	// - name: source_id
	//   in: query
	//   description: ID of the user's login source to search for
	//   type: integer
	//   format: int64
	// - name: login_name
	//   in: query
	//   description: user's login name to search for
	//   type: string
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
	//     "$ref": "#/responses/UserList"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	listOptions := utils.GetListOptions(ctx)

	users, maxResults, err := user_model.SearchUsers(ctx, &user_model.SearchUserOptions{
		Actor:       ctx.Doer,
		Type:        user_model.UserTypeIndividual,
		LoginName:   ctx.FormTrim("login_name"),
		SourceID:    ctx.FormInt64("source_id"),
		OrderBy:     db.SearchOrderByAlphabetically,
		ListOptions: listOptions,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchUsers", err)
		return
	}

	results := make([]*api.User, len(users))
	for i := range users {
		results[i] = convert.ToUser(ctx, users[i], ctx.Doer)
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, &results)
}

// RenameUser api for renaming a user
func RenameUser(ctx *context.APIContext) {
	// swagger:operation POST /admin/users/{username}/rename admin adminRenameUser
	// ---
	// summary: Rename a user
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: existing username of user
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/RenameUserOption"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	if ctx.ContextUser.IsOrganization() {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("%s is an organization not a user", ctx.ContextUser.Name))
		return
	}

	oldName := ctx.ContextUser.Name
	newName := web.GetForm(ctx).(*api.RenameUserOption).NewName

	// Check if user name has been changed
	if err := user_service.RenameUser(ctx, ctx.ContextUser, newName); err != nil {
		switch {
		case user_model.IsErrUsernameNotChanged(err):
			// Noop as username is not changed
			ctx.Status(http.StatusNoContent)
		case user_model.IsErrUserAlreadyExist(err):
			ctx.Error(http.StatusUnprocessableEntity, "", ctx.Tr("form.username_been_taken"))
		case db.IsErrNameReserved(err):
			ctx.Error(http.StatusUnprocessableEntity, "", ctx.Tr("user.form.name_reserved", newName))
		case db.IsErrNamePatternNotAllowed(err):
			ctx.Error(http.StatusUnprocessableEntity, "", ctx.Tr("user.form.name_pattern_not_allowed", newName))
		case db.IsErrNameCharsNotAllowed(err):
			ctx.Error(http.StatusUnprocessableEntity, "", ctx.Tr("user.form.name_chars_not_allowed", newName))
		default:
			ctx.ServerError("ChangeUserName", err)
		}
		return
	}

	log.Trace("User name changed: %s -> %s", oldName, newName)
	ctx.Status(http.StatusOK)
}
