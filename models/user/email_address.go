// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

var (
	// ErrEmailNotActivated e-mail address has not been activated error
	ErrEmailNotActivated = errors.New("E-mail address has not been activated")
)

// ErrEmailInvalid represents an error where the email address does not comply with RFC 5322
type ErrEmailInvalid struct {
	Email string
}

// IsErrEmailInvalid checks if an error is an ErrEmailInvalid
func IsErrEmailInvalid(err error) bool {
	_, ok := err.(ErrEmailInvalid)
	return ok
}

func (err ErrEmailInvalid) Error() string {
	return fmt.Sprintf("e-mail invalid [email: %s]", err.Email)
}

// ErrEmailAlreadyUsed represents a "EmailAlreadyUsed" kind of error.
type ErrEmailAlreadyUsed struct {
	Email string
}

// IsErrEmailAlreadyUsed checks if an error is a ErrEmailAlreadyUsed.
func IsErrEmailAlreadyUsed(err error) bool {
	_, ok := err.(ErrEmailAlreadyUsed)
	return ok
}

func (err ErrEmailAlreadyUsed) Error() string {
	return fmt.Sprintf("e-mail already in use [email: %s]", err.Email)
}

// ErrEmailAddressNotExist email address not exist
type ErrEmailAddressNotExist struct {
	Email string
}

// IsErrEmailAddressNotExist checks if an error is an ErrEmailAddressNotExist
func IsErrEmailAddressNotExist(err error) bool {
	_, ok := err.(ErrEmailAddressNotExist)
	return ok
}

func (err ErrEmailAddressNotExist) Error() string {
	return fmt.Sprintf("Email address does not exist [email: %s]", err.Email)
}

// ErrPrimaryEmailCannotDelete primary email address cannot be deleted
type ErrPrimaryEmailCannotDelete struct {
	Email string
}

// IsErrPrimaryEmailCannotDelete checks if an error is an ErrPrimaryEmailCannotDelete
func IsErrPrimaryEmailCannotDelete(err error) bool {
	_, ok := err.(ErrPrimaryEmailCannotDelete)
	return ok
}

func (err ErrPrimaryEmailCannotDelete) Error() string {
	return fmt.Sprintf("Primary email address cannot be deleted [email: %s]", err.Email)
}

// EmailAddress is the list of all email addresses of a user. It also contains the
// primary email address which is saved in user table.
type EmailAddress struct {
	ID          int64  `xorm:"pk autoincr"`
	UID         int64  `xorm:"INDEX NOT NULL"`
	Email       string `xorm:"UNIQUE NOT NULL"`
	LowerEmail  string `xorm:"UNIQUE NOT NULL"`
	IsActivated bool
	IsPrimary   bool `xorm:"DEFAULT(false) NOT NULL"`
}

func init() {
	db.RegisterModel(new(EmailAddress))
}

// BeforeInsert will be invoked by XORM before inserting a record
func (email *EmailAddress) BeforeInsert() {
	if email.LowerEmail == "" {
		email.LowerEmail = strings.ToLower(email.Email)
	}
}

// ValidateEmail check if email is a allowed address
func ValidateEmail(email string) error {
	if len(email) == 0 {
		return nil
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return ErrEmailInvalid{email}
	}

	// TODO: add an email allow/block list

	return nil
}

// GetEmailAddresses returns all email addresses belongs to given user.
func GetEmailAddresses(uid int64) ([]*EmailAddress, error) {
	emails := make([]*EmailAddress, 0, 5)
	if err := db.GetEngine(db.DefaultContext).
		Where("uid=?", uid).
		Asc("id").
		Find(&emails); err != nil {
		return nil, err
	}
	return emails, nil
}

// GetEmailAddressByID gets a user's email address by ID
func GetEmailAddressByID(uid, id int64) (*EmailAddress, error) {
	// User ID is required for security reasons
	email := &EmailAddress{UID: uid}
	if has, err := db.GetEngine(db.DefaultContext).ID(id).Get(email); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return email, nil
}

// IsEmailActive check if email is activated with a different emailID
func IsEmailActive(ctx context.Context, email string, excludeEmailID int64) (bool, error) {
	if len(email) == 0 {
		return true, nil
	}

	// Can't filter by boolean field unless it's explicit
	cond := builder.NewCond()
	cond = cond.And(builder.Eq{"lower_email": strings.ToLower(email)}, builder.Neq{"id": excludeEmailID})
	if setting.Service.RegisterEmailConfirm {
		// Inactive (unvalidated) addresses don't count as active if email validation is required
		cond = cond.And(builder.Eq{"is_activated": true})
	}

	var em EmailAddress
	if has, err := db.GetEngine(ctx).Where(cond).Get(&em); has || err != nil {
		if has {
			log.Info("isEmailActive(%q, %d) found duplicate in email ID %d", email, excludeEmailID, em.ID)
		}
		return has, err
	}

	return false, nil
}

// IsEmailUsed returns true if the email has been used.
func IsEmailUsed(ctx context.Context, email string) (bool, error) {
	if len(email) == 0 {
		return true, nil
	}

	return db.GetEngine(ctx).Where("lower_email=?", strings.ToLower(email)).Get(&EmailAddress{})
}

func addEmailAddress(ctx context.Context, email *EmailAddress) error {
	email.Email = strings.TrimSpace(email.Email)
	used, err := IsEmailUsed(ctx, email.Email)
	if err != nil {
		return err
	} else if used {
		return ErrEmailAlreadyUsed{email.Email}
	}

	if err = ValidateEmail(email.Email); err != nil {
		return err
	}

	return db.Insert(ctx, email)
}

// AddEmailAddress adds an email address to given user.
func AddEmailAddress(email *EmailAddress) error {
	return addEmailAddress(db.DefaultContext, email)
}

// AddEmailAddresses adds an email address to given user.
func AddEmailAddresses(emails []*EmailAddress) error {
	if len(emails) == 0 {
		return nil
	}

	// Check if any of them has been used
	for i := range emails {
		emails[i].Email = strings.TrimSpace(emails[i].Email)
		used, err := IsEmailUsed(db.DefaultContext, emails[i].Email)
		if err != nil {
			return err
		} else if used {
			return ErrEmailAlreadyUsed{emails[i].Email}
		}
		if err = ValidateEmail(emails[i].Email); err != nil {
			return err
		}
	}

	if err := db.Insert(db.DefaultContext, emails); err != nil {
		return fmt.Errorf("Insert: %v", err)
	}

	return nil
}

// DeleteEmailAddress deletes an email address of given user.
func DeleteEmailAddress(email *EmailAddress) (err error) {
	if email.IsPrimary {
		return ErrPrimaryEmailCannotDelete{Email: email.Email}
	}

	var deleted int64
	// ask to check UID
	address := EmailAddress{
		UID: email.UID,
	}
	if email.ID > 0 {
		deleted, err = db.GetEngine(db.DefaultContext).ID(email.ID).Delete(&address)
	} else {
		if email.Email != "" && email.LowerEmail == "" {
			email.LowerEmail = strings.ToLower(email.Email)
		}
		deleted, err = db.GetEngine(db.DefaultContext).
			Where("lower_email=?", email.LowerEmail).
			Delete(&address)
	}

	if err != nil {
		return err
	} else if deleted != 1 {
		return ErrEmailAddressNotExist{Email: email.Email}
	}
	return nil
}

// DeleteEmailAddresses deletes multiple email addresses
func DeleteEmailAddresses(emails []*EmailAddress) (err error) {
	for i := range emails {
		if err = DeleteEmailAddress(emails[i]); err != nil {
			return err
		}
	}

	return nil
}
