// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"fmt"

	auth_model "code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	password_module "code.gitea.io/gitea/modules/auth/password"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
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

	if opts.Visibility.Has() {
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
