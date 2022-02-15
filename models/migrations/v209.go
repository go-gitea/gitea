// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"encoding/base32"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/timeutil"

	"github.com/tstranex/u2f"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func increaseCredentialIDTo410(x *xorm.Engine) error {
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

	switch x.Dialect().URI().DBType {
	case schemas.MYSQL:
		_, err := x.Exec("ALTER TABLE webauthn_credential MODIFY COLUMN credential_id VARCHAR(410)")
		if err != nil {
			return err
		}
	case schemas.ORACLE:
		_, err := x.Exec("ALTER TABLE webauthn_credential MODIFY credential_id VARCHAR(410)")
		if err != nil {
			return err
		}
	case schemas.MSSQL:
		// This column has an index on it. I could write all of the code to attempt to change the index OR
		// I could just use recreate table.
		sess := x.NewSession()
		if err := sess.Begin(); err != nil {
			_ = sess.Close()
			return err
		}

		if err := recreateTable(sess, new(webauthnCredential)); err != nil {
			_ = sess.Close()
			return err
		}
		if err := sess.Commit(); err != nil {
			_ = sess.Close()
			return err
		}
		if err := sess.Close(); err != nil {
			return err
		}
	case schemas.POSTGRES:
		_, err := x.Exec("ALTER TABLE webauthn_credential ALTER COLUMN credential_id TYPE VARCHAR(410)")
		if err != nil {
			return err
		}
	default:
		// SQLite doesn't support ALTER COLUMN, and it already makes String _TEXT_ by default so no migration needed
		// nor is there any need to re-migrate
		return nil
	}

	exist, err := x.IsTableExist("u2f_registration")
	if err != nil {
		return err
	}
	if !exist {
		return nil
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

			cred := &webauthnCredential{}
			has, err := x.ID(reg.ID).Where("id = ? AND user_id = ?", reg.ID, reg.UserID).Get(cred)
			if err != nil {
				return fmt.Errorf("unable to get webauthn_credential[%d]. Error: %v", reg.ID, err)
			}
			if !has {
				continue
			}
			remigratedCredID := base32.HexEncoding.EncodeToString(parsed.KeyHandle)
			if cred.CredentialID == remigratedCredID || (!strings.HasPrefix(remigratedCredID, cred.CredentialID) && cred.CredentialID != "") {
				continue
			}

			cred.CredentialID = remigratedCredID

			_, err = x.ID(cred.ID).Update(cred)
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
