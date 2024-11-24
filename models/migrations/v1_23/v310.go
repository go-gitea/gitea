// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"github.com/go-webauthn/webauthn/protocol"
	"xorm.io/xorm"
)

func AddFlagsOnWebAuthnCredential(x *xorm.Engine) error {
	type WebAuthnCredential struct {
		Flags protocol.AuthenticatorFlags
	}
	if err := x.Sync(new(WebAuthnCredential)); err != nil {
		return err
	}
	_, err := x.Exec("UPDATE webauthn_credential SET flags = 29")
	return err
}
