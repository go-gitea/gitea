// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/base64"
	"encoding/binary"

	"code.gitea.io/gitea/modules/timeutil"

	"github.com/duo-labs/webauthn/webauthn"
)

//WebAuthnCredential represents the WebAuthn credential data for a public-key
//credential conformant to WebAuthn Level 1
type WebAuthnCredential struct {
	ID              int64 `xorm:"pk autoincr"`
	Name            string
	UserID          int64  `xorm:"INDEX"`
	CredentialID    string `xorm:"INDEX"`
	PublicKey       []byte
	AttestationType string
	AAGUID          []byte
	SignCount       uint32 `xorm:"BIGINT"`
	CloneWarning    bool
	CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix     timeutil.TimeStamp `xorm:"INDEX updated"`
}

// TableName returns a better table name for WebAuthnCredential
func (cred WebAuthnCredential) TableName() string {
	return "webauthn_credential"
}

// UpdateSignCount will update the database value of SignCount
func (cred *WebAuthnCredential) UpdateSignCount() error {
	return cred.updateSignCount(x)
}

func (cred *WebAuthnCredential) updateSignCount(e Engine) error {
	_, err := e.ID(cred.ID).Cols("sign_count").Update(cred)
	return err
}

// WebAuthnCredentialList is a list of *WebAuthnCredential
type WebAuthnCredentialList []*WebAuthnCredential

// ToCredentials will convert all WebAuthnCredentials to webauthn.Credentials
func (list WebAuthnCredentialList) ToCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, 0, len(list))
	for _, cred := range list {
		credID, _ := base64.RawStdEncoding.DecodeString(cred.CredentialID)
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

//GetWebAuthnCredentialsByUID returns all WebAuthn credentials of the given user
func GetWebAuthnCredentialsByUID(uid int64) (WebAuthnCredentialList, error) {
	return getWebAuthnCredentialsByUID(x, uid)
}

func getWebAuthnCredentialsByUID(e Engine, uid int64) (WebAuthnCredentialList, error) {
	creds := make(WebAuthnCredentialList, 0)
	return creds, e.Where("user_id = ?", uid).Find(&creds)
}

// GetWebAuthnCredentialByID returns WebAuthn credential by id
func GetWebAuthnCredentialByID(id int64) (*WebAuthnCredential, error) {
	return getWebAuthnCredentialByID(x, id)
}

func getWebAuthnCredentialByID(e Engine, id int64) (*WebAuthnCredential, error) {
	cred := new(WebAuthnCredential)
	if found, err := e.ID(id).Get(cred); err != nil {
		return nil, err
	} else if !found {
		return nil, ErrWebAuthnCredentialNotExist{ID: id}
	}
	return cred, nil
}

// GetWebAuthnCredentialByCredID returns WebAuthn credential by credential ID
func GetWebAuthnCredentialByCredID(credID string) (*WebAuthnCredential, error) {
	return getWebAuthnCredentialByCredID(x, credID)
}

func getWebAuthnCredentialByCredID(e Engine, credID string) (*WebAuthnCredential, error) {
	cred := new(WebAuthnCredential)
	if found, err := e.Where("credential_id = ?", credID).Get(cred); err != nil {
		return nil, err
	} else if !found {
		return nil, ErrWebAuthnCredentialNotExist{CredentialID: credID}
	}
	return cred, nil
}

// CreateCredential will create a new WebAuthnCredential from the given Credential
func CreateCredential(user *User, name string, cred *webauthn.Credential) (*WebAuthnCredential, error) {
	return createCredential(x, user, name, cred)
}

func createCredential(e Engine, user *User, name string, cred *webauthn.Credential) (*WebAuthnCredential, error) {
	c := &WebAuthnCredential{
		UserID:          user.ID,
		Name:            name,
		CredentialID:    base64.RawStdEncoding.EncodeToString(cred.ID),
		PublicKey:       cred.PublicKey,
		AttestationType: cred.AttestationType,
		AAGUID:          cred.Authenticator.AAGUID,
		SignCount:       0,
		CloneWarning:    false,
	}

	_, err := e.InsertOne(c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// DeleteCredential will delete WebAuthnCredential
func DeleteCredential(cred *WebAuthnCredential) error {
	return deleteCredential(x, cred)
}

func deleteCredential(e Engine, cred *WebAuthnCredential) error {
	_, err := e.Delete(cred)
	return err
}

//WebAuthnID implements the webauthn.User interface
func (u *User) WebAuthnID() []byte {
	id := make([]byte, 8)
	binary.PutVarint(id, u.ID)
	return id
}

//WebAuthnName implements the webauthn.User interface
func (u *User) WebAuthnName() string {
	return u.LoginName
}

//WebAuthnDisplayName implements the webauthn.User interface
func (u *User) WebAuthnDisplayName() string {
	return u.Name
}

//WebAuthnIcon implements the webauthn.User interface
func (u *User) WebAuthnIcon() string {
	return ""
}

//WebAuthnCredentials implementns the webauthn.User interface
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	dbCreds, err := GetWebAuthnCredentialsByUID(u.ID)
	if err != nil {
		return nil
	}

	return dbCreds.ToCredentials()
}
