// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17 //nolint

import (
	"encoding/base32"
	"fmt"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func StoreWebauthnCredentialIDAsBytes(x *xorm.Engine) error {
	// Create webauthnCredential table
	type webauthnCredential struct {
		ID           int64 `xorm:"pk autoincr"`
		Name         string
		LowerName    string `xorm:"unique(s)"`
		UserID       int64  `xorm:"INDEX unique(s)"`
		CredentialID string `xorm:"INDEX VARCHAR(410)"`
		// Note the lack of INDEX here - these will be created once the column is renamed in v223.go
		CredentialIDBytes []byte `xorm:"VARBINARY(1024)"` // CredentialID is at most 1023 bytes as per spec released 20 July 2022
		PublicKey         []byte
		AttestationType   string
		AAGUID            []byte
		SignCount         uint32 `xorm:"BIGINT"`
		CloneWarning      bool
		CreatedUnix       timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix       timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	if err := x.Sync(&webauthnCredential{}); err != nil {
		return err
	}

	var start int
	creds := make([]*webauthnCredential, 0, 50)
	for {
		err := x.Select("id, credential_id").OrderBy("id").Limit(50, start).Find(&creds)
		if err != nil {
			return err
		}

		err = func() error {
			sess := x.NewSession()
			defer sess.Close()
			if err := sess.Begin(); err != nil {
				return fmt.Errorf("unable to allow start session. Error: %w", err)
			}
			for _, cred := range creds {
				cred.CredentialIDBytes, err = base32.HexEncoding.DecodeString(cred.CredentialID)
				if err != nil {
					return fmt.Errorf("unable to parse credential id %s for credential[%d]: %w", cred.CredentialID, cred.ID, err)
				}
				count, err := sess.ID(cred.ID).Cols("credential_id_bytes").Update(cred)
				if count != 1 || err != nil {
					return fmt.Errorf("unable to update credential id bytes for credential[%d]: %d,%w", cred.ID, count, err)
				}
			}
			return sess.Commit()
		}()
		if err != nil {
			return err
		}

		if len(creds) < 50 {
			break
		}
		start += 50
		creds = creds[:0]
	}
	return nil
}
