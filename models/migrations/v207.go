// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"crypto/elliptic"
	"encoding/base64"
	"strings"

	"code.gitea.io/gitea/modules/timeutil"

	"github.com/tstranex/u2f"
	"xorm.io/xorm"
)

func addWebAuthnCred(x *xorm.Engine) error {
	// Create webauthnCredential table
	type webauthnCredential struct {
		ID              int64 `xorm:"pk autoincr"`
		Name            string
		LowerName       string `xorm:"unique(s)"`
		UserID          int64  `xorm:"INDEX unique(s)"`
		CredentialID    string `xorm:"INDEX VARCHAR(410)"` // CredentalID in U2F is at most 255bytes / 5 * 8 = 408 - add a few extra characters for safety
		PublicKey       []byte
		AttestationType string
		AAGUID          []byte
		SignCount       uint32 `xorm:"BIGINT"`
		CloneWarning    bool
		CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix     timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	if err := x.Sync2(&webauthnCredential{}); err != nil {
		return err
	}

	// Now migrate the old u2f registrations to the new format
	type u2fRegistration struct {
		ID          int64 `xorm:"pk autoincr"`
		Name        string
		UserID      int64 `xorm:"INDEX"`
		Raw         []byte
		Counter     uint32             `xorm:"BIGINT"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	var start int
	regs := make([]*u2fRegistration, 0, 50)
	for {
		err := x.OrderBy("id").Limit(50, start).Find(&regs)
		if err != nil {
			return err
		}

		for _, reg := range regs {
			parsed := new(u2f.Registration)
			err = parsed.UnmarshalBinary(reg.Raw)
			if err != nil {
				continue
			}

			c := &webauthnCredential{
				ID:              reg.ID,
				Name:            reg.Name,
				LowerName:       strings.ToLower(reg.Name),
				UserID:          reg.UserID,
				CredentialID:    base64.RawStdEncoding.EncodeToString(parsed.KeyHandle),
				PublicKey:       elliptic.Marshal(elliptic.P256(), parsed.PubKey.X, parsed.PubKey.Y),
				AttestationType: "fido-u2f",
				AAGUID:          []byte{},
				SignCount:       reg.Counter,
			}

			_, err := x.Insert(c)
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
