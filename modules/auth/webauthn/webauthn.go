// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webauthn

import (
	"encoding/binary"
	"encoding/gob"
	"net/url"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/duo-labs/webauthn/protocol"
	"github.com/duo-labs/webauthn/webauthn"
)

//WebAuthn represents the global WebAuthn instance
var WebAuthn *webauthn.WebAuthn

//Init initializes the WebAuthn instance from the config.
func Init() {
	gob.Register(&webauthn.SessionData{})

	appURL, _ := url.Parse(setting.AppURL)

	WebAuthn = &webauthn.WebAuthn{
		Config: &webauthn.Config{
			RPDisplayName: setting.AppName,
			RPID:          setting.Domain,
			RPOrigin:      protocol.FullyQualifiedOrigin(appURL),
			AuthenticatorSelection: protocol.AuthenticatorSelection{
				UserVerification: "discouraged",
			},
			AttestationPreference: protocol.PreferDirectAttestation,
		},
	}
}

// User represents an implementation of webauthn.User based on User model
type User user_model.User

//WebAuthnID implements the webauthn.User interface
func (u *User) WebAuthnID() []byte {
	id := make([]byte, 8)
	binary.PutVarint(id, u.ID)
	return id
}

//WebAuthnName implements the webauthn.User interface
func (u *User) WebAuthnName() string {
	if u.LoginName == "" {
		return u.Name
	}
	return u.LoginName
}

//WebAuthnDisplayName implements the webauthn.User interface
func (u *User) WebAuthnDisplayName() string {
	return (*user_model.User)(u).DisplayName()
}

//WebAuthnIcon implements the webauthn.User interface
func (u *User) WebAuthnIcon() string {
	return (*user_model.User)(u).AvatarLink()
}

//WebAuthnCredentials implementns the webauthn.User interface
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	dbCreds, err := auth.GetWebAuthnCredentialsByUID(u.ID)
	if err != nil {
		return nil
	}

	return dbCreds.ToCredentials()
}
