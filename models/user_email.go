// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
	"xorm.io/builder"
)

// ActivateEmail activates the email address to given user.
func ActivateEmail(email *user_model.EmailAddress) error {
	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := updateActivation(sess, email, true); err != nil {
		return err
	}
	return sess.Commit()
}

func updateActivation(e db.Engine, email *user_model.EmailAddress, activate bool) error {
	user, err := getUserByID(e, email.UID)
	if err != nil {
		return err
	}
	if user.Rands, err = GetUserSalt(); err != nil {
		return err
	}
	email.IsActivated = activate
	if _, err := e.ID(email.ID).Cols("is_activated").Update(email); err != nil {
		return err
	}
	return updateUserCols(e, user, "rands")
}

// MakeEmailPrimary sets primary email address of given user.
func MakeEmailPrimary(email *user_model.EmailAddress) error {
	has, err := db.GetEngine(db.DefaultContext).Get(email)
	if err != nil {
		return err
	} else if !has {
		return user_model.ErrEmailAddressNotExist{Email: email.Email}
	}

	if !email.IsActivated {
		return user_model.ErrEmailNotActivated
	}

	user := &User{}
	has, err = db.GetEngine(db.DefaultContext).ID(email.UID).Get(user)
	if err != nil {
		return err
	} else if !has {
		return ErrUserNotExist{email.UID, "", 0}
	}

	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	// 1. Update user table
	user.Email = email.Email
	if _, err = sess.ID(user.ID).Cols("email").Update(user); err != nil {
		return err
	}

	// 2. Update old primary email
	if _, err = sess.Where("uid=? AND is_primary=?", email.UID, true).Cols("is_primary").Update(&user_model.EmailAddress{
		IsPrimary: false,
	}); err != nil {
		return err
	}

	// 3. update new primary email
	email.IsPrimary = true
	if _, err = sess.ID(email.ID).Cols("is_primary").Update(email); err != nil {
		return err
	}

	return sess.Commit()
}

// SearchEmailOrderBy is used to sort the results from SearchEmails()
type SearchEmailOrderBy string

func (s SearchEmailOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	SearchEmailOrderByEmail        SearchEmailOrderBy = "email_address.lower_email ASC, email_address.is_primary DESC, email_address.id ASC"
	SearchEmailOrderByEmailReverse SearchEmailOrderBy = "email_address.lower_email DESC, email_address.is_primary ASC, email_address.id DESC"
	SearchEmailOrderByName         SearchEmailOrderBy = "`user`.lower_name ASC, email_address.is_primary DESC, email_address.id ASC"
	SearchEmailOrderByNameReverse  SearchEmailOrderBy = "`user`.lower_name DESC, email_address.is_primary ASC, email_address.id DESC"
)

// SearchEmailOptions are options to search e-mail addresses for the admin panel
type SearchEmailOptions struct {
	db.ListOptions
	Keyword     string
	SortType    SearchEmailOrderBy
	IsPrimary   util.OptionalBool
	IsActivated util.OptionalBool
}

// SearchEmailResult is an e-mail address found in the user or email_address table
type SearchEmailResult struct {
	UID         int64
	Email       string
	IsActivated bool
	IsPrimary   bool
	// From User
	Name     string
	FullName string
}

// SearchEmails takes options i.e. keyword and part of email name to search,
// it returns results in given range and number of total results.
func SearchEmails(opts *SearchEmailOptions) ([]*SearchEmailResult, int64, error) {
	var cond builder.Cond = builder.Eq{"`user`.`type`": UserTypeIndividual}
	if len(opts.Keyword) > 0 {
		likeStr := "%" + strings.ToLower(opts.Keyword) + "%"
		cond = cond.And(builder.Or(
			builder.Like{"lower(`user`.full_name)", likeStr},
			builder.Like{"`user`.lower_name", likeStr},
			builder.Like{"email_address.lower_email", likeStr},
		))
	}

	switch {
	case opts.IsPrimary.IsTrue():
		cond = cond.And(builder.Eq{"email_address.is_primary": true})
	case opts.IsPrimary.IsFalse():
		cond = cond.And(builder.Eq{"email_address.is_primary": false})
	}

	switch {
	case opts.IsActivated.IsTrue():
		cond = cond.And(builder.Eq{"email_address.is_activated": true})
	case opts.IsActivated.IsFalse():
		cond = cond.And(builder.Eq{"email_address.is_activated": false})
	}

	count, err := db.GetEngine(db.DefaultContext).Join("INNER", "`user`", "`user`.ID = email_address.uid").
		Where(cond).Count(new(user_model.EmailAddress))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	orderby := opts.SortType.String()
	if orderby == "" {
		orderby = SearchEmailOrderByEmail.String()
	}

	opts.SetDefaultValues()

	emails := make([]*SearchEmailResult, 0, opts.PageSize)
	err = db.GetEngine(db.DefaultContext).Table("email_address").
		Select("email_address.*, `user`.name, `user`.full_name").
		Join("INNER", "`user`", "`user`.ID = email_address.uid").
		Where(cond).
		OrderBy(orderby).
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		Find(&emails)

	return emails, count, err
}

// ActivateUserEmail will change the activated state of an email address,
// either primary or secondary (all in the email_address table)
func ActivateUserEmail(userID int64, email string, activate bool) (err error) {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	// Activate/deactivate a user's secondary email address
	// First check if there's another user active with the same address
	addr := user_model.EmailAddress{UID: userID, LowerEmail: strings.ToLower(email)}
	if has, err := sess.Get(&addr); err != nil {
		return err
	} else if !has {
		return fmt.Errorf("no such email: %d (%s)", userID, email)
	}
	if addr.IsActivated == activate {
		// Already in the desired state; no action
		return nil
	}
	if activate {
		if used, err := user_model.IsEmailActive(ctx, email, addr.ID); err != nil {
			return fmt.Errorf("unable to check isEmailActive() for %s: %v", email, err)
		} else if used {
			return user_model.ErrEmailAlreadyUsed{Email: email}
		}
	}
	if err = updateActivation(sess, &addr, activate); err != nil {
		return fmt.Errorf("unable to updateActivation() for %d:%s: %w", addr.ID, addr.Email, err)
	}

	// Activate/deactivate a user's primary email address and account
	if addr.IsPrimary {
		user := User{ID: userID, Email: email}
		if has, err := sess.Get(&user); err != nil {
			return err
		} else if !has {
			return fmt.Errorf("no user with ID: %d and Email: %s", userID, email)
		}
		// The user's activation state should be synchronized with the primary email
		if user.IsActive != activate {
			user.IsActive = activate
			if user.Rands, err = GetUserSalt(); err != nil {
				return fmt.Errorf("unable to generate salt: %v", err)
			}
			if err = updateUserCols(sess, &user, "is_active", "rands"); err != nil {
				return fmt.Errorf("unable to updateUserCols() for user ID: %d: %v", userID, err)
			}
		}
	}

	return committer.Commit()
}
