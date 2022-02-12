// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func increaseCredentialIDTo410(x *xorm.Engine) error {
	// Create webauthnCredential table
	type webauthnCredential struct {
		ID           int64  `xorm:"pk autoincr"`
		CredentialID string `xorm:"INDEX VARCHAR(410)"`
	}
	if err := x.Sync2(&webauthnCredential{}); err != nil {
		return err
	}

	switch x.Dialect().URI().DBType {
	case schemas.MYSQL:
		_, err := x.Exec("ALTER TABLE webauthn_credential MODIFY COLUMN content VARCHAR(410)")
		if err != nil {
			return err
		}
	case schemas.ORACLE:
		_, err := x.Exec("ALTER TABLE webauthn_credential MODIFY content VARCHAR(410)")
		if err != nil {
			return err
		}
	case schemas.MSSQL:
		_, err := x.Exec("ALTER TABLE webauthn_credential ALTER COLUMN content VARCHAR(410)")
		if err != nil {
			return err
		}
	case schemas.POSTGRES:
		_, err := x.Exec("ALTER TABLE webauthn_credential ALTER COLUMN content TYPE VARCHAR(410)")
		if err != nil {
			return err
		}
	default:
		// SQLite doesn't support ALTER COLUMN, and it seem to already makes String _TEXT_ by default so no migration needed
	}

	return nil
}
