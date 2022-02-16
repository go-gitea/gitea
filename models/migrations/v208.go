// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"encoding/base32"
	"encoding/base64"

	"xorm.io/xorm"
)

func useBase32HexForCredIDInWebAuthnCredential(x *xorm.Engine) error {
	// Create webauthnCredential table
	type webauthnCredential struct {
		ID           int64  `xorm:"pk autoincr"`
		CredentialID string `xorm:"INDEX VARCHAR(410)"`
	}
	if err := x.Sync2(&webauthnCredential{}); err != nil {
		return err
	}

	var start int
	regs := make([]*webauthnCredential, 0, 50)
	for {
		err := x.OrderBy("id").Limit(50, start).Find(&regs)
		if err != nil {
			return err
		}

		for _, reg := range regs {
			credID, _ := base64.RawStdEncoding.DecodeString(reg.CredentialID)
			reg.CredentialID = base32.HexEncoding.EncodeToString(credID)

			_, err := x.Update(reg)
			if err != nil {
				return err
			}
		}

		if len(regs) < 50 {
			break
		}
		start += 50
		regs = regs[:0]
	}

	return nil
}
