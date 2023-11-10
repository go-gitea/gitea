// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webauthn

import (
	"encoding/binary"
	"encoding/gob"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthn represents the global WebAuthn instance
var WebAuthn *webauthn.WebAuthn

// Init initializes the WebAuthn instance from the config.
func Init() {
	gob.Register(&webauthn.SessionData{})

	appURL, _ := protocol.FullyQualifiedOrigin(setting.AppURL)

	WebAuthn = &webauthn.WebAuthn{
		Config: &webauthn.Config{
			RPDisplayName: setting.AppName,
			RPID:          setting.Domain,
			RPOrigins:     []string{appURL},
			AuthenticatorSelection: protocol.AuthenticatorSelection{
				UserVerification: "discouraged",
			},
			AttestationPreference: protocol.PreferDirectAttestation,
		},
	}
}

// User represents an implementation of webauthn.User based on User model
type User user_model.User

// WebAuthnID implements the webauthn.User interface
func (u *User) WebAuthnID() []byte {
	id := make([]byte, 8)
	binary.PutVarint(id, u.ID)
	return id
}

// WebAuthnName implements the webauthn.User interface
func (u *User) WebAuthnName() string {
	if u.LoginName == "" {
		return u.Name
	}
	return u.LoginName
}

// WebAuthnDisplayName implements the webauthn.User interface
func (u *User) WebAuthnDisplayName() string {
	return (*user_model.User)(u).DisplayName()
}

// WebAuthnIcon implements the webauthn.User interface
func (u *User) WebAuthnIcon() string {
	return (*user_model.User)(u).AvatarLink(db.DefaultContext)
}

// WebAuthnCredentials implementns the webauthn.User interface
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	dbCreds, err := auth.GetWebAuthnCredentialsByUID(db.DefaultContext, u.ID)
	if err != nil {
		return nil
	}

	return dbCreds.ToCredentials()
}
