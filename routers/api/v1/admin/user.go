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
	//   description: username of the user whose data is to be edited
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
		if err := user_service.ReplacePrimaryEmailAddress(ctx, ctx.ContextUser, *form.Email); err != nil {
			switch {
			case user_model.IsErrEmailCharIsNotSupported(err), user_model.IsErrEmailInvalid(err):
				if !user_model.IsEmailDomainAllowed(*form.Email) {
					err = fmt.Errorf("the domain of user email %s conflicts with EMAIL_DOMAIN_ALLOWLIST or EMAIL_DOMAIN_BLOCKLIST", *form.Email)
				}
				ctx.APIError(http.StatusBadRequest, err)
			case user_model.IsErrEmailAlreadyUsed(err):
				ctx.APIError(http.StatusBadRequest, err)
			default:
				ctx.APIErrorInternal(err)
			}
			return
		}
	}

	opts := &user_service.UpdateOptions{
		FullName:                optional.FromPtr(form.FullName),
		Website:                 optional.FromPtr(form.Website),
		Location:                optional.FromPtr(form.Location),
		Description:             optional.FromPtr(form.Description),
		IsActive:                optional.FromPtr(form.Active),
		IsAdmin:                 user_service.UpdateOptionFieldFromPtr(form.Admin),
		Visibility:              optional.FromMapLookup(api.VisibilityModes, form.Visibility),
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
	//   description: username of the user to delete
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
		ctx.APIError(http.StatusUnprocessableEntity, errors.New("you cannot delete yourself"))
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
	//   description: username of the user who is to receive a public key
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
	//   description: username of the user whose public key is to be deleted
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
	//   description: identifier of the user, provided by the external authenticator
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// - name: sort
	//   in: query
	//   description: sort users by attribute. Supported values are
	//                "name", "created", "updated" and "id".
	//                Default is "name"
	//   type: string
	// - name: order
	//   in: query
	//   description: sort order, either "asc" (ascending) or "desc" (descending).
	//                Default is "asc", ignored if "sort" is not specified.
	//   type: string
	// - name: q
	//   in: query
	//   description: search term (username, full name, email)
	//   type: string
	// - name: visibility
	//   in: query
	//   description: visibility filter. Supported values are
	//                "public", "limited" and "private".
	//   type: string
	// - name: is_active
	//   in: query
	//   description: filter active users
	//   type: boolean
	// - name: is_admin
	//   in: query
	//   description: filter admin users
	//   type: boolean
	// - name: is_restricted
	//   in: query
	//   description: filter restricted users
	//   type: boolean
	// - name: is_2fa_enabled
	//   in: query
	//   description: filter 2FA enabled users
	//   type: boolean
	// - name: is_prohibit_login
	//   in: query
	//   description: filter login prohibited users
	//   type: boolean
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	listOptions := utils.GetListOptions(ctx)

	orderBy := db.SearchOrderByAlphabetically
	sortMode := ctx.FormString("sort")
	if len(sortMode) > 0 {
		sortOrder := ctx.FormString("order")
		if len(sortOrder) == 0 {
			sortOrder = "asc"
		}
		if searchModeMap, ok := user_model.AdminUserOrderByMap[sortOrder]; ok {
			if order, ok := searchModeMap[sortMode]; ok {
				orderBy = order
			} else {
				ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("Invalid sort mode: \"%s\"", sortMode))
				return
			}
		} else {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("Invalid sort order: \"%s\"", sortOrder))
			return
		}
	}

	var visible []api.VisibleType
	visibilityParam := ctx.FormString("visibility")
	if len(visibilityParam) > 0 {
		if visibility, ok := api.VisibilityModes[visibilityParam]; ok {
			visible = []api.VisibleType{visibility}
		} else {
			ctx.APIError(http.StatusUnprocessableEntity, fmt.Errorf("Invalid visibility: \"%s\"", visibilityParam))
			return
		}
	}

	searchOpts := user_model.SearchUserOptions{
		Actor:         ctx.Doer,
		Types:         []user_model.UserType{user_model.UserTypeIndividual},
		LoginName:     ctx.FormTrim("login_name"),
		SourceID:      ctx.FormInt64("source_id"),
		Keyword:       ctx.FormTrim("q"),
		Visible:       visible,
		OrderBy:       orderBy,
		ListOptions:   listOptions,
		SearchByEmail: true,
	}

	if ctx.FormString("is_active") != "" {
		searchOpts.IsActive = optional.Some(ctx.FormBool("is_active"))
	}
	if ctx.FormString("is_admin") != "" {
		searchOpts.IsAdmin = optional.Some(ctx.FormBool("is_admin"))
	}
	if ctx.FormString("is_restricted") != "" {
		searchOpts.IsRestricted = optional.Some(ctx.FormBool("is_restricted"))
	}
	if ctx.FormString("is_2fa_enabled") != "" {
		searchOpts.IsTwoFactorEnabled = optional.Some(ctx.FormBool("is_2fa_enabled"))
	}
	if ctx.FormString("is_prohibit_login") != "" {
		searchOpts.IsProhibitLogin = optional.Some(ctx.FormBool("is_prohibit_login"))
	}

	users, maxResults, err := user_model.SearchUsers(ctx, searchOpts)
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
	//   description: current username of the user
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
	if err := user_service.RenameUser(ctx, ctx.ContextUser, newName, ctx.Doer); err != nil {
		if user_model.IsErrUserAlreadyExist(err) || db.IsErrNameReserved(err) || db.IsErrNamePatternNotAllowed(err) || db.IsErrNameCharsNotAllowed(err) {
			ctx.APIError(http.StatusUnprocessableEntity, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
