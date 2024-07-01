// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"errors"
	"strings"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/audit"
)

// AdminAddOrSetPrimaryEmailAddress is used by admins to add or set a user's primary email address
func AdminAddOrSetPrimaryEmailAddress(ctx context.Context, doer, u *user_model.User, emailStr string) error {
	if strings.EqualFold(u.Email, emailStr) {
		return nil
	}

	if err := user_model.ValidateEmailForAdmin(emailStr); err != nil {
		return err
	}

	// Check if address exists already
	email, err := user_model.GetEmailAddressByEmail(ctx, emailStr)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return err
	}
	if email != nil && email.UID != u.ID {
		return user_model.ErrEmailAlreadyUsed{Email: emailStr}
	}

	// Update old primary address
	primary, err := user_model.GetPrimaryEmailAddressOfUser(ctx, u.ID)
	if err != nil {
		return err
	}

	primary.IsPrimary = false
	if err := user_model.UpdateEmailAddress(ctx, primary); err != nil {
		return err
	}

	// Insert new or update existing address
	if email != nil {
		email.IsPrimary = true
		email.IsActivated = true
		if err := user_model.UpdateEmailAddress(ctx, email); err != nil {
			return err
		}
	} else {
		email = &user_model.EmailAddress{
			UID:         u.ID,
			Email:       emailStr,
			IsActivated: true,
			IsPrimary:   true,
		}
		if _, err := user_model.InsertEmailAddress(ctx, email); err != nil {
			return err
		}
	}

	u.Email = emailStr

	if err := user_model.UpdateUserCols(ctx, u, "email"); err != nil {
		return err
	}

	audit.RecordUserEmailPrimaryChange(ctx, doer, u, email)

	return nil
}

func ReplacePrimaryEmailAddress(ctx context.Context, doer, u *user_model.User, emailStr string) error {
	if strings.EqualFold(u.Email, emailStr) {
		return nil
	}

	if err := user_model.ValidateEmail(emailStr); err != nil {
		return err
	}

	if !u.IsOrganization() {
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
		email = &user_model.EmailAddress{
			UID:         u.ID,
			Email:       emailStr,
			IsActivated: true,
			IsPrimary:   true,
		}
		if _, err := user_model.InsertEmailAddress(ctx, email); err != nil {
			return err
		}

		u.Email = emailStr

		if err := user_model.UpdateUserCols(ctx, u, "email"); err != nil {
			return err
		}

		audit.RecordUserEmailPrimaryChange(ctx, doer, u, email)
	} else {
		u.Email = emailStr

		if err := user_model.UpdateUserCols(ctx, u, "email"); err != nil {
			return err
		}
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
		audit.RecordUserEmailAdd(ctx, doer, u, email)
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
		audit.RecordUserEmailRemove(ctx, doer, u, email)
	}

	return nil
}
