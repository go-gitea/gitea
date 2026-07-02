// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/services/audit"
)

// ReplacePrimaryEmailAddress replaces the user's primary email address with the given email address.
// It also updates the user's email field to match the new primary email address.
func ReplacePrimaryEmailAddress(ctx context.Context, doer, u *user_model.User, emailStr string) error {
	// FIXME: this check is from old logic, but it is not right, there are far more user types, not only "organization"
	if u.IsOrganization() {
		return util.NewInvalidArgumentErrorf("user %s is an organization", u.Name)
	}

	if strings.EqualFold(u.Email, emailStr) {
		return nil
	}

	if err := user_model.ValidateEmail(emailStr); err != nil {
		return err
	}

	var newEmail *user_model.EmailAddress
	if err := db.WithTx(ctx, func(ctx context.Context) error {
		// Check if address exists already
		email, err := user_model.GetEmailAddressByEmail(ctx, emailStr)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			return err
		}
		if email != nil {
			if email.IsPrimary && email.UID == u.ID {
				return nil
			}
			return user_model.ErrEmailAlreadyUsed{Email: emailStr}
		}

		// Remove old primary address
		primary, err := user_model.GetPrimaryEmailAddressOfUser(ctx, u.ID)
		if err != nil {
			return err
		}
		if _, err := db.DeleteByID[user_model.EmailAddress](ctx, primary.ID); err != nil {
			return err
		}

		// Insert new primary address
		newEmail = &user_model.EmailAddress{
			UID:         u.ID,
			Email:       emailStr,
			IsActivated: true,
			IsPrimary:   true,
		}
		if _, err := user_model.InsertEmailAddress(ctx, newEmail); err != nil {
			return err
		}

		u.Email = emailStr
		return user_model.UpdateUserCols(ctx, u, "email")
	}); err != nil {
		return err
	}

	if newEmail != nil {
		audit.Record(ctx, audit_model.UserEmailPrimaryChange, doer, u,
			fmt.Sprintf("Changed primary email of user %s to %s.", u.Name, newEmail.Email), "email", newEmail.Email)
	}

	return nil
}

func AddEmailAddresses(ctx context.Context, doer, u *user_model.User, emailsToAdd []string) error {
	emails := make([]*user_model.EmailAddress, 0, len(emailsToAdd))

	for _, emailStr := range emailsToAdd {
		if err := user_model.ValidateEmail(emailStr); err != nil {
			return err
		}

		// Check if address exists already
		email, err := user_model.GetEmailAddressByEmail(ctx, emailStr)
		if err != nil && !errors.Is(err, util.ErrNotExist) {
			return err
		}
		if email != nil {
			return user_model.ErrEmailAlreadyUsed{Email: emailStr}
		}

		// Insert new address
		email = &user_model.EmailAddress{
			UID:         u.ID,
			Email:       emailStr,
			IsActivated: !setting.Service.RegisterEmailConfirm,
			IsPrimary:   false,
		}
		if _, err := user_model.InsertEmailAddress(ctx, email); err != nil {
			return err
		}

		emails = append(emails, email)
	}

	for _, email := range emails {
		audit.Record(ctx, audit_model.UserEmailAdd, doer, u,
			fmt.Sprintf("Added email %s to user %s.", email.Email, u.Name), "email", email.Email)
	}

	return nil
}

func DeleteEmailAddresses(ctx context.Context, doer, u *user_model.User, emailsToRemove []string) error {
	emails := make([]*user_model.EmailAddress, 0, len(emailsToRemove))

	for _, emailStr := range emailsToRemove {
		// Check if address exists
		email, err := user_model.GetEmailAddressOfUser(ctx, emailStr, u.ID)
		if err != nil {
			return err
		}
		if email.IsPrimary {
			return user_model.ErrPrimaryEmailCannotDelete{Email: emailStr}
		}

		// Remove address
		if _, err := db.DeleteByID[user_model.EmailAddress](ctx, email.ID); err != nil {
			return err
		}

		emails = append(emails, email)
	}

	for _, email := range emails {
		audit.Record(ctx, audit_model.UserEmailRemove, doer, u,
			fmt.Sprintf("Removed email %s from user %s.", email.Email, u.Name), "email", email.Email)
	}

	return nil
}
