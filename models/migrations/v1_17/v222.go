// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_17 //nolint

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func DropOldCredentialIDColumn(x *xorm.Engine) error {
	// This migration maybe rerun so that we should check if it has been run
	credentialIDExist, err := x.Dialect().IsColumnExist(x.DB(), context.Background(), "webauthn_credential", "credential_id")
	if err != nil {
		return err
	}
	if !credentialIDExist {
		// Column is already non-extant
		return nil
	}
	credentialIDBytesExists, err := x.Dialect().IsColumnExist(x.DB(), context.Background(), "webauthn_credential", "credential_id_bytes")
	if err != nil {
		return err
	}
	if !credentialIDBytesExists {
		// looks like 221 hasn't properly run
		return fmt.Errorf("webauthn_credential does not have a credential_id_bytes column... it is not safe to run this migration")
	}

	// Create webauthnCredential table
	type webauthnCredential struct {
		ID           int64 `xorm:"pk autoincr"`
		Name         string
		LowerName    string `xorm:"unique(s)"`
		UserID       int64  `xorm:"INDEX unique(s)"`
		CredentialID string `xorm:"INDEX VARCHAR(410)"`
		// Note the lack of the INDEX on CredentialIDBytes - we will add this in v223.go
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

	// Drop the old credential ID
	sess := x.NewSession()
	defer sess.Close()

	if err := base.DropTableColumns(sess, "webauthn_credential", "credential_id"); err != nil {
		return fmt.Errorf("unable to drop old credentialID column: %w", err)
	}
	return sess.Commit()
}
