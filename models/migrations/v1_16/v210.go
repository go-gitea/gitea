// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16 //nolint

import (
	"encoding/base32"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/tstranex/u2f"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

// v208 migration was completely broken
func RemigrateU2FCredentials(x *xorm.Engine) error {
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
	if err := x.Sync(&webauthnCredential{}); err != nil {
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

		if err := base.RecreateTable(sess, new(webauthnCredential)); err != nil {
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

		err = func() error {
			sess := x.NewSession()
			defer sess.Close()
			if err := sess.Begin(); err != nil {
				return fmt.Errorf("unable to allow start session. Error: %w", err)
			}
			if x.Dialect().URI().DBType == schemas.MSSQL {
				if _, err := sess.Exec("SET IDENTITY_INSERT `webauthn_credential` ON"); err != nil {
					return fmt.Errorf("unable to allow identity insert on webauthn_credential. Error: %w", err)
				}
			}
			for _, reg := range regs {
				parsed := new(u2f.Registration)
				err = parsed.UnmarshalBinary(reg.Raw)
				if err != nil {
					continue
				}
				pubKey, err := parsed.PubKey.ECDH()
				if err != nil {
					continue
				}
				remigrated := &webauthnCredential{
					ID:              reg.ID,
					Name:            reg.Name,
					LowerName:       strings.ToLower(reg.Name),
					UserID:          reg.UserID,
					CredentialID:    base32.HexEncoding.EncodeToString(parsed.KeyHandle),
					PublicKey:       pubKey.Bytes(),
					AttestationType: "fido-u2f",
					AAGUID:          []byte{},
					SignCount:       reg.Counter,
					UpdatedUnix:     reg.UpdatedUnix,
					CreatedUnix:     reg.CreatedUnix,
				}

				has, err := sess.ID(reg.ID).Get(new(webauthnCredential))
				if err != nil {
					return fmt.Errorf("unable to get webauthn_credential[%d]. Error: %w", reg.ID, err)
				}
				if !has {
					has, err := sess.Where("`lower_name`=?", remigrated.LowerName).And("`user_id`=?", remigrated.UserID).Exist(new(webauthnCredential))
					if err != nil {
						return fmt.Errorf("unable to check webauthn_credential[lower_name: %s, user_id: %d]. Error: %w", remigrated.LowerName, remigrated.UserID, err)
					}
					if !has {
						_, err = sess.Insert(remigrated)
						if err != nil {
							return fmt.Errorf("unable to (re)insert webauthn_credential[%d]. Error: %w", reg.ID, err)
						}

						continue
					}
				}

				_, err = sess.ID(remigrated.ID).AllCols().Update(remigrated)
				if err != nil {
					return fmt.Errorf("unable to update webauthn_credential[%d]. Error: %w", reg.ID, err)
				}
			}
			return sess.Commit()
		}()
		if err != nil {
			return err
		}

		if len(regs) < 50 {
			break
		}
		start += 50
		regs = regs[:0]
	}

	if x.Dialect().URI().DBType == schemas.POSTGRES {
		if _, err := x.Exec("SELECT setval('webauthn_credential_id_seq', COALESCE((SELECT MAX(id)+1 FROM `webauthn_credential`), 1), false)"); err != nil {
			return err
		}
	}

	return nil
}
