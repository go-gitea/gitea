// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-webauthn/webauthn/webauthn"
	"xorm.io/xorm"
)

// ErrWebAuthnCredentialNotExist represents a "ErrWebAuthnCRedentialNotExist" kind of error.
type ErrWebAuthnCredentialNotExist struct {
	ID           int64
	CredentialID []byte
}

func (err ErrWebAuthnCredentialNotExist) Error() string {
	if len(err.CredentialID) == 0 {
		return fmt.Sprintf("WebAuthn credential does not exist [id: %d]", err.ID)
	}
	return fmt.Sprintf("WebAuthn credential does not exist [credential_id: %x]", err.CredentialID)
}

// Unwrap unwraps this as a ErrNotExist err
func (err ErrWebAuthnCredentialNotExist) Unwrap() error {
	return util.ErrNotExist
}

// IsErrWebAuthnCredentialNotExist checks if an error is a ErrWebAuthnCredentialNotExist.
func IsErrWebAuthnCredentialNotExist(err error) bool {
	_, ok := err.(ErrWebAuthnCredentialNotExist)
	return ok
}

// WebAuthnCredential represents the WebAuthn credential data for a public-key
// credential conformant to WebAuthn Level 1
type WebAuthnCredential struct {
	ID              int64 `xorm:"pk autoincr"`
	Name            string
	LowerName       string `xorm:"unique(s)"`
	UserID          int64  `xorm:"INDEX unique(s)"`
	CredentialID    []byte `xorm:"INDEX VARBINARY(1024)"`
	PublicKey       []byte
	AttestationType string
	AAGUID          []byte
	SignCount       uint32 `xorm:"BIGINT"`
	CloneWarning    bool
	CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix     timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(WebAuthnCredential))
}

// TableName returns a better table name for WebAuthnCredential
func (cred WebAuthnCredential) TableName() string {
	return "webauthn_credential"
}

// UpdateSignCount will update the database value of SignCount
func (cred *WebAuthnCredential) UpdateSignCount(ctx context.Context) error {
	_, err := db.GetEngine(ctx).ID(cred.ID).Cols("sign_count").Update(cred)
	return err
}

// BeforeInsert will be invoked by XORM before updating a record
func (cred *WebAuthnCredential) BeforeInsert() {
	cred.LowerName = strings.ToLower(cred.Name)
}

// BeforeUpdate will be invoked by XORM before updating a record
func (cred *WebAuthnCredential) BeforeUpdate() {
	cred.LowerName = strings.ToLower(cred.Name)
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (cred *WebAuthnCredential) AfterLoad(session *xorm.Session) {
	cred.LowerName = strings.ToLower(cred.Name)
}

// WebAuthnCredentialList is a list of *WebAuthnCredential
type WebAuthnCredentialList []*WebAuthnCredential

// ToCredentials will convert all WebAuthnCredentials to webauthn.Credentials
func (list WebAuthnCredentialList) ToCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, 0, len(list))
	for _, cred := range list {
		creds = append(creds, webauthn.Credential{
			ID:              cred.CredentialID,
			PublicKey:       cred.PublicKey,
			AttestationType: cred.AttestationType,
			Authenticator: webauthn.Authenticator{
				AAGUID:       cred.AAGUID,
				SignCount:    cred.SignCount,
				CloneWarning: cred.CloneWarning,
			},
		})
	}
	return creds
}

// GetWebAuthnCredentialsByUID returns all WebAuthn credentials of the given user
func GetWebAuthnCredentialsByUID(ctx context.Context, uid int64) (WebAuthnCredentialList, error) {
	creds := make(WebAuthnCredentialList, 0)
	return creds, db.GetEngine(ctx).Where("user_id = ?", uid).Find(&creds)
}

// ExistsWebAuthnCredentialsForUID returns if the given user has credentials
func ExistsWebAuthnCredentialsForUID(ctx context.Context, uid int64) (bool, error) {
	return db.GetEngine(ctx).Where("user_id = ?", uid).Exist(&WebAuthnCredential{})
}

// GetWebAuthnCredentialByName returns WebAuthn credential by id
func GetWebAuthnCredentialByName(ctx context.Context, uid int64, name string) (*WebAuthnCredential, error) {
	cred := new(WebAuthnCredential)
	if found, err := db.GetEngine(ctx).Where("user_id = ? AND lower_name = ?", uid, strings.ToLower(name)).Get(cred); err != nil {
		return nil, err
	} else if !found {
		return nil, ErrWebAuthnCredentialNotExist{}
	}
	return cred, nil
}

// GetWebAuthnCredentialByID returns WebAuthn credential by id
func GetWebAuthnCredentialByID(ctx context.Context, id int64) (*WebAuthnCredential, error) {
	cred := new(WebAuthnCredential)
	if found, err := db.GetEngine(ctx).ID(id).Get(cred); err != nil {
		return nil, err
	} else if !found {
		return nil, ErrWebAuthnCredentialNotExist{ID: id}
	}
	return cred, nil
}

// HasWebAuthnRegistrationsByUID returns whether a given user has WebAuthn registrations
func HasWebAuthnRegistrationsByUID(ctx context.Context, uid int64) (bool, error) {
	return db.GetEngine(ctx).Where("user_id = ?", uid).Exist(&WebAuthnCredential{})
}

// GetWebAuthnCredentialByCredID returns WebAuthn credential by credential ID
func GetWebAuthnCredentialByCredID(ctx context.Context, userID int64, credID []byte) (*WebAuthnCredential, error) {
	cred := new(WebAuthnCredential)
	if found, err := db.GetEngine(ctx).Where("user_id = ? AND credential_id = ?", userID, credID).Get(cred); err != nil {
		return nil, err
	} else if !found {
		return nil, ErrWebAuthnCredentialNotExist{CredentialID: credID}
	}
	return cred, nil
}

// CreateCredential will create a new WebAuthnCredential from the given Credential
func CreateCredential(ctx context.Context, userID int64, name string, cred *webauthn.Credential) (*WebAuthnCredential, error) {
	c := &WebAuthnCredential{
		UserID:          userID,
		Name:            name,
		CredentialID:    cred.ID,
		PublicKey:       cred.PublicKey,
		AttestationType: cred.AttestationType,
		AAGUID:          cred.Authenticator.AAGUID,
		SignCount:       cred.Authenticator.SignCount,
		CloneWarning:    false,
	}

	if err := db.Insert(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// DeleteCredential will delete WebAuthnCredential
func DeleteCredential(ctx context.Context, id, userID int64) (bool, error) {
	had, err := db.GetEngine(ctx).ID(id).Where("user_id = ?", userID).Delete(&WebAuthnCredential{})
	return had > 0, err
}

// WebAuthnCredentials implementns the webauthn.User interface
func WebAuthnCredentials(ctx context.Context, userID int64) ([]webauthn.Credential, error) {
	dbCreds, err := GetWebAuthnCredentialsByUID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return dbCreds.ToCredentials(), nil
}
