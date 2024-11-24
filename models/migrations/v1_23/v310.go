// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"github.com/go-webauthn/webauthn/protocol"
	"xorm.io/xorm"
)

type WebAuthnCredential struct {
	Flags protocol.AuthenticatorFlags
}

func (cred WebAuthnCredential) TableName() string {
	return "webauthn_credential"
}

func AddFlagsOnWebAuthnCredential(x *xorm.Engine) error {
	if err := x.Sync(new(WebAuthnCredential)); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE webauthn_credential SET flags = 29 WHERE id > 0")
	return err
}
