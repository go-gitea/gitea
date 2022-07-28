// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func increaseCredentialIDTo1640(x *xorm.Engine) error {
	// Create webauthnCredential table
	type webauthnCredential struct {
		ID              int64 `xorm:"pk autoincr"`
		Name            string
		LowerName       string `xorm:"unique(s)"`
		UserID          int64  `xorm:"INDEX unique(s)"`
		CredentialID    string `xorm:"INDEX VARCHAR(1640)"` // CredentialID is at most 1023 bytes as per spec released 20 July 2022 -> 1640 base32 encoding
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
		_, err := x.Exec("ALTER TABLE webauthn_credential MODIFY COLUMN credential_id VARCHAR(1640)")
		if err != nil {
			return err
		}
	case schemas.ORACLE:
		_, err := x.Exec("ALTER TABLE webauthn_credential MODIFY credential_id VARCHAR(1640)")
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
		_, err := x.Exec("ALTER TABLE webauthn_credential ALTER COLUMN credential_id TYPE VARCHAR(1640)")
		if err != nil {
			return err
		}
	default:
		// SQLite doesn't support ALTER COLUMN, and it already makes String _TEXT_ by default so no migration needed
		// nor is there any need to re-migrate
	}
	return nil
}
