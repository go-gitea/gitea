// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_23 //nolint

import (
	"code.gitea.io/gitea/modules/json"

	"github.com/go-webauthn/webauthn/webauthn"
	"xorm.io/xorm"
)

type WebAuthnCredential struct {
	CredentialFlags string `xorm:"TEXT DEFAULT ''"`
}

func (cred WebAuthnCredential) TableName() string {
	return "webauthn_credential"
}

func AddFlagsOnWebAuthnCredential(x *xorm.Engine) error {
	if err := x.Sync(new(WebAuthnCredential)); err != nil {
		return err
	}

	defaultCredentialFlags := webauthn.CredentialFlags{
		BackupEligible: true,
	}
	defaultCredentialFlagsJSON, err := json.Marshal(defaultCredentialFlags)
	if err != nil {
		return err
	}
	_, err = x.Exec("UPDATE webauthn_credential SET credential_flags = ? WHERE credential_flags = '' OR credential_flags IS NULL", string(defaultCredentialFlagsJSON))
	return err
}
