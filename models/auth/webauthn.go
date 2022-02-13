// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"context"
	"encoding/base32"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/duo-labs/webauthn/webauthn"
	"xorm.io/xorm"
)

// ErrWebAuthnCredentialNotExist represents a "ErrWebAuthnCRedentialNotExist" kind of error.
type ErrWebAuthnCredentialNotExist struct {
	ID           int64
	CredentialID string
}

func (err ErrWebAuthnCredentialNotExist) Error() string {
	if err.CredentialID == "" {
		return fmt.Sprintf("WebAuthn credential does not exist [id: %d]", err.ID)
	}
	return fmt.Sprintf("WebAuthn credential does not exist [credential_id: %s]", err.CredentialID)
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
	CredentialID    string `xorm:"INDEX"`
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
func (cred *WebAuthnCredential) UpdateSignCount() error {
	return cred.updateSignCount(db.DefaultContext)
}

func (cred *WebAuthnCredential) updateSignCount(ctx context.Context) error {
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
		credID, _ := base32.HexEncoding.DecodeString(cred.CredentialID)
		creds = append(creds, webauthn.Credential{
			ID:              credID,
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
func GetWebAuthnCredentialsByUID(uid int64) (WebAuthnCredentialList, error) {
	return getWebAuthnCredentialsByUID(db.DefaultContext, uid)
}

func getWebAuthnCredentialsByUID(ctx context.Context, uid int64) (WebAuthnCredentialList, error) {
	creds := make(WebAuthnCredentialList, 0)
	return creds, db.GetEngine(ctx).Where("user_id = ?", uid).Find(&creds)
}

// ExistsWebAuthnCredentialsForUID returns if the given user has credentials
func ExistsWebAuthnCredentialsForUID(uid int64) (bool, error) {
	return existsWebAuthnCredentialsByUID(db.DefaultContext, uid)
}

func existsWebAuthnCredentialsByUID(ctx context.Context, uid int64) (bool, error) {
	return db.GetEngine(ctx).Where("user_id = ?", uid).Exist(&WebAuthnCredential{})
}

// GetWebAuthnCredentialByName returns WebAuthn credential by id
func GetWebAuthnCredentialByName(uid int64, name string) (*WebAuthnCredential, error) {
	return getWebAuthnCredentialByName(db.DefaultContext, uid, name)
}

func getWebAuthnCredentialByName(ctx context.Context, uid int64, name string) (*WebAuthnCredential, error) {
	cred := new(WebAuthnCredential)
	if found, err := db.GetEngine(ctx).Where("user_id = ? AND lower_name = ?", uid, strings.ToLower(name)).Get(cred); err != nil {
		return nil, err
	} else if !found {
		return nil, ErrWebAuthnCredentialNotExist{}
	}
	return cred, nil
}

// GetWebAuthnCredentialByID returns WebAuthn credential by id
func GetWebAuthnCredentialByID(id int64) (*WebAuthnCredential, error) {
	return getWebAuthnCredentialByID(db.DefaultContext, id)
}

func getWebAuthnCredentialByID(ctx context.Context, id int64) (*WebAuthnCredential, error) {
	cred := new(WebAuthnCredential)
	if found, err := db.GetEngine(ctx).ID(id).Get(cred); err != nil {
		return nil, err
	} else if !found {
		return nil, ErrWebAuthnCredentialNotExist{ID: id}
	}
	return cred, nil
}

// HasWebAuthnRegistrationsByUID returns whether a given user has WebAuthn registrations
func HasWebAuthnRegistrationsByUID(uid int64) (bool, error) {
	return db.GetEngine(db.DefaultContext).Where("user_id = ?", uid).Exist(&WebAuthnCredential{})
}

// GetWebAuthnCredentialByCredID returns WebAuthn credential by credential ID
func GetWebAuthnCredentialByCredID(userID int64, credID string) (*WebAuthnCredential, error) {
	return getWebAuthnCredentialByCredID(db.DefaultContext, userID, credID)
}

func getWebAuthnCredentialByCredID(ctx context.Context, userID int64, credID string) (*WebAuthnCredential, error) {
	cred := new(WebAuthnCredential)
	if found, err := db.GetEngine(ctx).Where("user_id = ? AND credential_id = ?", userID, credID).Get(cred); err != nil {
		return nil, err
	} else if !found {
		return nil, ErrWebAuthnCredentialNotExist{CredentialID: credID}
	}
	return cred, nil
}

// CreateCredential will create a new WebAuthnCredential from the given Credential
func CreateCredential(userID int64, name string, cred *webauthn.Credential) (*WebAuthnCredential, error) {
	return createCredential(db.DefaultContext, userID, name, cred)
}

func createCredential(ctx context.Context, userID int64, name string, cred *webauthn.Credential) (*WebAuthnCredential, error) {
	c := &WebAuthnCredential{
		UserID:          userID,
		Name:            name,
		CredentialID:    base32.HexEncoding.EncodeToString(cred.ID),
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
func DeleteCredential(id, userID int64) (bool, error) {
	return deleteCredential(db.DefaultContext, id, userID)
}

func deleteCredential(ctx context.Context, id, userID int64) (bool, error) {
	had, err := db.GetEngine(ctx).ID(id).Where("user_id = ?", userID).Delete(&WebAuthnCredential{})
	return had > 0, err
}

// WebAuthnCredentials implementns the webauthn.User interface
func WebAuthnCredentials(userID int64) ([]webauthn.Credential, error) {
	dbCreds, err := GetWebAuthnCredentialsByUID(userID)
	if err != nil {
		return nil, err
	}

	return dbCreds.ToCredentials(), nil
}
