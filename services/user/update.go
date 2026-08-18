// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"

	activities_model "gitea.dev/models/activities"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	password_module "gitea.dev/modules/auth/password"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

type UpdateOptionField[T any] struct {
	FieldValue T
	FromSync   bool
}

func UpdateOptionFieldFromValue[T any](value T) optional.Option[UpdateOptionField[T]] {
	return optional.Some(UpdateOptionField[T]{FieldValue: value})
}

func UpdateOptionFieldFromSync[T any](value T) optional.Option[UpdateOptionField[T]] {
	return optional.Some(UpdateOptionField[T]{FieldValue: value, FromSync: true})
}

func UpdateOptionFieldFromPtr[T any](value *T) optional.Option[UpdateOptionField[T]] {
	if value == nil {
		return optional.None[UpdateOptionField[T]]()
	}
	return UpdateOptionFieldFromValue(*value)
}

type UpdateOptions struct {
	KeepEmailPrivate             optional.Option[bool]
	FullName                     optional.Option[string]
	Website                      optional.Option[string]
	Location                     optional.Option[string]
	Description                  optional.Option[string]
	AllowGitHook                 optional.Option[bool]
	AllowImportLocal             optional.Option[bool]
	MaxRepoCreation              optional.Option[int]
	IsRestricted                 optional.Option[bool]
	Visibility                   optional.Option[structs.VisibleType]
	KeepActivityPrivate          optional.Option[bool]
	Language                     optional.Option[string]
	Theme                        optional.Option[string]
	DiffViewStyle                optional.Option[string]
	AllowCreateOrganization      optional.Option[bool]
	IsActive                     optional.Option[bool]
	IsAdmin                      optional.Option[UpdateOptionField[bool]]
	EmailNotificationsPreference optional.Option[string]
	SetLastLogin                 bool
	RepoAdminChangeTeamAccess    optional.Option[bool]
}

func UpdateUser(ctx context.Context, u *user_model.User, opts *UpdateOptions) error {
	cols := make([]string, 0, 20)

	if opts.KeepEmailPrivate.Has() {
		u.KeepEmailPrivate = opts.KeepEmailPrivate.Value()

		cols = append(cols, "keep_email_private")
	}

	if opts.FullName.Has() {
		u.FullName = opts.FullName.Value()

		cols = append(cols, "full_name")
	}
	if opts.Website.Has() {
		u.Website = opts.Website.Value()

		cols = append(cols, "website")
	}
	if opts.Location.Has() {
		u.Location = opts.Location.Value()

		cols = append(cols, "location")
	}
	if opts.Description.Has() {
		u.Description = opts.Description.Value()

		cols = append(cols, "description")
	}
	if opts.Language.Has() {
		u.Language = opts.Language.Value()

		cols = append(cols, "language")
	}
	if opts.Theme.Has() {
		u.Theme = opts.Theme.Value()

		cols = append(cols, "theme")
	}
	if opts.DiffViewStyle.Has() {
		u.DiffViewStyle = opts.DiffViewStyle.Value()

		cols = append(cols, "diff_view_style")
	}

	if opts.AllowGitHook.Has() {
		u.AllowGitHook = opts.AllowGitHook.Value()

		cols = append(cols, "allow_git_hook")
	}
	if opts.AllowImportLocal.Has() {
		u.AllowImportLocal = opts.AllowImportLocal.Value()

		cols = append(cols, "allow_import_local")
	}

	if opts.MaxRepoCreation.Has() {
		u.MaxRepoCreation = opts.MaxRepoCreation.Value()

		cols = append(cols, "max_repo_creation")
	}

	if opts.IsActive.Has() {
		u.IsActive = opts.IsActive.Value()

		cols = append(cols, "is_active")
	}
	if opts.IsRestricted.Has() {
		u.IsRestricted = opts.IsRestricted.Value()

		cols = append(cols, "is_restricted")
	}
	if opts.IsAdmin.Has() {
		if opts.IsAdmin.Value().FieldValue /* true */ {
			if u.IsTypeBot() {
				return user_model.ErrBotCanNotBeAdmin
			}
			u.IsAdmin = opts.IsAdmin.Value().FieldValue // set IsAdmin=true
			cols = append(cols, "is_admin")
		} else if !user_model.IsLastAdminUser(ctx, u) /* not the last admin */ {
			u.IsAdmin = opts.IsAdmin.Value().FieldValue // it's safe to change it from false to true (not the last admin)
			cols = append(cols, "is_admin")
		} else /* IsAdmin=false but this is the last admin user */ { //nolint:gocritic // make it easier to read
			if !opts.IsAdmin.Value().FromSync {
				return user_model.ErrDeleteLastAdminUser{UID: u.ID}
			}
			// else: syncing from external-source, this user is the last admin, so skip the "IsAdmin=false" change
		}
	}

	// only validate and persist the visibility when it actually changes
	if opts.Visibility.Has() && opts.Visibility.Value() != u.Visibility {
		if !u.IsOrganization() && !setting.Service.AllowedUserVisibilityModesSlice.IsAllowedVisibility(opts.Visibility.Value()) {
			return fmt.Errorf("visibility mode not allowed: %s", opts.Visibility.Value().String())
		}
		u.Visibility = opts.Visibility.Value()

		cols = append(cols, "visibility")
	}
	if opts.KeepActivityPrivate.Has() {
		u.KeepActivityPrivate = opts.KeepActivityPrivate.Value()

		cols = append(cols, "keep_activity_private")
	}

	if opts.AllowCreateOrganization.Has() {
		u.AllowCreateOrganization = opts.AllowCreateOrganization.Value()

		cols = append(cols, "allow_create_organization")
	}
	if opts.RepoAdminChangeTeamAccess.Has() {
		u.RepoAdminChangeTeamAccess = opts.RepoAdminChangeTeamAccess.Value()

		cols = append(cols, "repo_admin_change_team_access")
	}

	if opts.EmailNotificationsPreference.Has() {
		u.EmailNotificationsPreference = opts.EmailNotificationsPreference.Value()

		cols = append(cols, "email_notifications_preference")
	}

	if opts.SetLastLogin {
		u.SetLastLogin()

		cols = append(cols, "last_login_unix")
	}

	return user_model.UpdateUserCols(ctx, u, cols...)
}

type UpdateAuthOptions struct {
	LoginSource        optional.Option[int64]
	LoginName          optional.Option[string]
	Password           optional.Option[string]
	MustChangePassword optional.Option[bool]
	ProhibitLogin      optional.Option[bool]
}

func UpdateAuth(ctx context.Context, u *user_model.User, opts *UpdateAuthOptions) error {
	if u.IsTypeBot() {
		// a bot only ever authenticates with an access token, so it has neither a password nor an auth source
		if opts.Password.Has() {
			return fmt.Errorf("%w: a bot account cannot have a password", util.ErrInvalidArgument)
		}
		if opts.LoginSource.ValueOrDefault(0) != 0 || opts.LoginName.ValueOrDefault("") != "" {
			return fmt.Errorf("%w: a bot account cannot be linked to an authentication source", util.ErrInvalidArgument)
		}
		u.LoginType, u.LoginSource, u.LoginName = auth_model.Plain, 0, ""
	} else {
		if opts.LoginSource.Has() {
			source, err := auth_model.GetSourceByID(ctx, opts.LoginSource.Value())
			if err != nil {
				return err
			}

			u.LoginType = source.Type
			u.LoginSource = source.ID
		}
		if opts.LoginName.Has() {
			u.LoginName = opts.LoginName.Value()
		}
	}

	deleteAuthTokens := false
	if opts.Password.Has() && (u.IsLocal() || u.IsOAuth2()) {
		password := opts.Password.Value()

		if len(password) < setting.MinPasswordLength {
			return password_module.ErrMinLength
		}
		if !password_module.IsComplexEnough(password) {
			return password_module.ErrComplexity
		}
		if err := password_module.IsPwned(ctx, password); err != nil {
			return err
		}

		if err := u.SetPassword(password); err != nil {
			return err
		}

		deleteAuthTokens = true
	}

	if opts.MustChangePassword.Has() {
		u.MustChangePassword = opts.MustChangePassword.Value()
	}
	if opts.ProhibitLogin.Has() {
		u.ProhibitLogin = opts.ProhibitLogin.Value()
	}

	if err := user_model.UpdateUserCols(ctx, u, "login_type", "login_source", "login_name", "passwd", "passwd_hash_algo", "salt", "must_change_password", "prohibit_login"); err != nil {
		return err
	}

	if deleteAuthTokens {
		return auth_model.DeleteAuthTokensByUserID(ctx, u.ID)
	}
	return nil
}

// CheckConvertUserType checks whether the account type of the given user can be converted
func CheckConvertUserType(u *user_model.User) error {
	switch {
	case u.IsAdmin:
		// automation does not need site-wide root access, so the admin permission has to be
		// dropped deliberately before the account can become a bot
		return user_model.ErrBotCanNotBeAdmin
	case !u.IsIndividual() && !u.IsTypeBot():
		return user_model.ErrUserTypeCanNotConvert
	}
	return nil
}

// ConvertUserType converts a user between the individual and bot types.
// When converting to a bot the user becomes a local, non-interactive account:
// its password and auth source are cleared so it can only be used with access tokens.
func ConvertUserType(ctx context.Context, u *user_model.User, targetType user_model.UserType) error {
	if u.Type == targetType {
		return nil
	}
	if targetType != user_model.UserTypeIndividual && targetType != user_model.UserTypeBot {
		return user_model.ErrUserTypeCanNotConvert
	}
	if err := CheckConvertUserType(u); err != nil {
		return err
	}

	updatedUser := *u
	updatedUser.Type = targetType
	cols := []string{"type"}

	if targetType == user_model.UserTypeBot {
		// see models/user/bot_user_design.md for the fate of every credential and auth artifact
		updatedUser.Passwd = ""
		updatedUser.PasswdHashAlgo = ""
		updatedUser.Salt = ""
		updatedUser.MustChangePassword = false
		updatedUser.LoginType = auth_model.Plain
		updatedUser.LoginSource = 0
		updatedUser.LoginName = ""
		cols = append(cols, "passwd", "passwd_hash_algo", "salt", "must_change_password", "login_type", "login_source", "login_name")

		// atomic, so a mid-sequence failure cannot leave a half-converted account
		if err := db.WithTx(ctx, func(ctx context.Context) error {
			if err := user_model.UpdateUserCols(ctx, &updatedUser, cols...); err != nil {
				return err
			}
			// revoke persisted sign-in sessions so the former individual cannot stay logged in
			if err := auth_model.DeleteAuthTokensByUserID(ctx, updatedUser.ID); err != nil {
				return err
			}
			// remove OAuth2 applications and grants owned/authorized by the account
			if err := auth_model.DeleteOAuth2RelictsByUserID(ctx, updatedUser.ID); err != nil {
				return err
			}
			// TOTP and WebAuthn only guard an interactive sign-in, which a bot no longer has
			if _, _, err := auth_model.DisableTwoFactor(ctx, updatedUser.ID); err != nil {
				return err
			}
			// a bot has no inbox, so drop the notifications it accumulated as an individual
			if err := db.DeleteBeans(ctx, &activities_model.Notification{UserID: updatedUser.ID}); err != nil {
				return err
			}
			// an OpenID URI is an external identity that can sign the account in, so it goes too
			if err := db.DeleteBeans(ctx, &user_model.UserOpenID{UID: updatedUser.ID}); err != nil {
				return err
			}
			// the account is now local, so drop any external (OAuth2/LDAP/...) login links
			return user_model.RemoveAllAccountLinks(ctx, &updatedUser)
		}); err != nil {
			return err
		}
	} else if err := user_model.UpdateUserCols(ctx, &updatedUser, cols...); err != nil {
		return err
	}

	*u = updatedUser
	return nil
}
