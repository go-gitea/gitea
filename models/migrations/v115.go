package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addWebAuthnCred(x *xorm.Engine) error {
	type webauthnCredential struct {
		ID              int64 `xorm:"pk autoincr"`
		Name            string
		UserID          int64  `xorm:"INDEX"`
		CredentialID    string `xorm:"INDEX"`
		PublicKey       []byte
		AttestationType string
		AAGUID          []byte
		SignCount       uint32 `xorm:"BIGINT"`
		CloneWarning    bool
		CreatedUnix     timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix     timeutil.TimeStamp `xorm:"INDEX updated"`
	}
	return x.Sync2(&webauthnCredential{})
}
