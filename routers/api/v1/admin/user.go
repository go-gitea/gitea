// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"errors"
	"fmt"
	"net/http"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	org_model "code.gitea.io/gitea/models/organization"
	packages_model "code.gitea.io/gitea/models/packages"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/user"
	"code.gitea.io/gitea/routers/api/v1/utils"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/mailer"
	user_service "code.gitea.io/gitea/services/user"
)

func parseAuthSource(ctx *context.APIContext, u *user_model.User, sourceID int64) {
	if sourceID == 0 {
		return
	}

	source, err := auth.GetSourceByID(ctx, sourceID)
	if err != nil {
		if auth.IsErrSourceNotExist(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	u.LoginType = source.Type
	u.LoginSource = source.ID
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
		LoginName:          form.LoginName,
	}
	if form.MustChangePassword != nil {
		u.MustChangePassword = *form.MustChangePassword
	}

	parseAuthSource(ctx, u, form.SourceID)
	if ctx.Written() {
		return
	}

	if u.LoginType == auth.Plain {
		if len(form.Password) < setting.MinPasswordLength {
			err := errors.New("PasswordIsRequired")
			ctx.APIError(http.StatusBadRequest, err)
			return
		}

		if !password.IsComplexEnough(form.Password) {
			err := errors.New("PasswordComplexity")
			ctx.APIError(http.StatusBadRequest, err)
			return
		}

		if err := password.IsPwned(ctx, form.Password); err != nil {
			if password.IsErrIsPwnedRequest(err) {
				log.Error(err.Error())
			}
			ctx.APIError(http.StatusBadRequest, errors.New("PasswordPwned"))
			return
		}
	}

	overwriteDefault := &user_model.CreateUserOverwriteOptions{
		IsActive:     optional.Some(true),
		IsRestricted: optional.FromPtr(form.Restricted),
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

	if err := user_model.AdminCreateUser(ctx, u, &user_model.Meta{}, overwriteDefault); err != nil {
		if user_model.IsErrUserAlreadyExist(err) ||
			user_model.IsErrEmailAlreadyUsed(err) ||
			db.IsErrNameReserved(err) ||
			db.IsErrNameCharsNotAllowed(err) ||
			user_model.IsErrEmailCharIsNotSupported(err) ||
			user_model.IsErrEmailInvalid(err) ||
			db.IsErrNamePatternNotAllowed(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	if !user_model.IsEmailDomainAllowed(u.Email) {
		ctx.Resp.Header().Add("X-Gitea-Warning", fmt.Sprintf("the domain of user email %s conflicts with EMAIL_DOMAIN_ALLOWLIST or EMAIL_DOMAIN_BLOCKLIST", u.Email))
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
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.EditUserOption)

	authOpts := &user_service.UpdateAuthOptions{
		LoginSource:        optional.FromNonDefault(form.SourceID),
		LoginName:          optional.Some(form.LoginName),
		Password:           optional.FromNonDefault(form.Password),
		MustChangePassword: optional.FromPtr(form.MustChangePassword),
		ProhibitLogin:      optional.FromPtr(form.ProhibitLogin),
	}
	if err := user_service.UpdateAuth(ctx, ctx.ContextUser, authOpts); err != nil {
		switch {
		case errors.Is(err, password.ErrMinLength):
			ctx.APIError(http.StatusBadRequest, fmt.Errorf("password must be at least %d characters", setting.MinPasswordLength))
		case errors.Is(err, password.ErrComplexity):
			ctx.APIError(http.StatusBadRequest, err)
		case errors.Is(err, password.ErrIsPwned), password.IsErrIsPwnedRequest(err):
			ctx.APIError(http.StatusBadRequest, err)
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}

	if form.Email != nil {
		if err := user_service.AdminAddOrSetPrimaryEmailAddress(ctx, ctx.ContextUser, *form.Email); err != nil {
			switch {
			case user_model.IsErrEmailCharIsNotSupported(err), user_model.IsErrEmailInvalid(err):
				ctx.APIError(http.StatusBadRequest, err)
			case user_model.IsErrEmailAlreadyUsed(err):
				ctx.APIError(http.StatusBadRequest, err)
			default:
				ctx.APIErrorInternal(err)
			}
			return
		}

		if !user_model.IsEmailDomainAllowed(*form.Email) {
			ctx.Resp.Header().Add("X-Gitea-Warning", fmt.Sprintf("the domain of user email %s conflicts with EMAIL_DOMAIN_ALLOWLIST or EMAIL_DOMAIN_BLOCKLIST", *form.Email))
		}
	}

	opts := &user_service.UpdateOptions{
		FullName:                optional.FromPtr(form.FullName),
		Website:                 optional.FromPtr(form.Website),
		Location:                optional.FromPtr(form.Location),
		Description:             optional.FromPtr(form.Description),
		IsActive:                optional.FromPtr(form.Active),
		IsAdmin:                 optional.FromPtr(form.Admin),
		Visibility:              optional.FromNonDefault(api.VisibilityModes[form.Visibility]),
		AllowGitHook:            optional.FromPtr(form.AllowGitHook),
		AllowImportLocal:        optional.FromPtr(form.AllowImportLocal),
		MaxRepoCreation:         optional.FromPtr(form.MaxRepoCreation),
		AllowCreateOrganization: optional.FromPtr(form.AllowCreateOrganization),
		IsRestricted:            optional.FromPtr(form.Restricted),
	}

	if err := user_service.UpdateUser(ctx, ctx.ContextUser, opts); err != nil {
		if user_model.IsErrDeleteLastAdminUser(err) {
			ctx.APIError(http.StatusBadRequest, err)
		} else {
			ctx.APIErrorInternal(err)
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
		ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("%s is an organization not a user", ctx.ContextUser.Name))
		return
	}

	// admin should not delete themself
	if ctx.ContextUser.ID == ctx.Doer.ID {
		ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("you cannot delete yourself"))
		return
	}

	if err := user_service.DeleteUser(ctx, ctx.ContextUser, ctx.FormBool("purge")); err != nil {
		if repo_model.IsErrUserOwnRepos(err) ||
			org_model.IsErrUserHasOrgs(err) ||
			packages_model.IsErrUserOwnPackages(err) ||
			user_model.IsErrDeleteLastAdminUser(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else {
			ctx.APIErrorInternal(err)
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

	if err := asymkey_service.DeletePublicKey(ctx, ctx.ContextUser, ctx.PathParamInt64("id")); err != nil {
		if asymkey_model.IsErrKeyNotExist(err) {
			ctx.APIErrorNotFound()
		} else if asymkey_model.IsErrKeyAccessDenied(err) {
			ctx.APIError(http.StatusForbidden, "You do not have access to this key")
		} else {
			ctx.APIErrorInternal(err)
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
		ctx.APIErrorInternal(err)
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
		ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("%s is an organization not a user", ctx.ContextUser.Name))
		return
	}

	newName := web.GetForm(ctx).(*api.RenameUserOption).NewName

	// Check if username has been changed
	if err := user_service.RenameUser(ctx, ctx.ContextUser, newName); err != nil {
		if user_model.IsErrUserAlreadyExist(err) || db.IsErrNameReserved(err) || db.IsErrNamePatternNotAllowed(err) || db.IsErrNameCharsNotAllowed(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
