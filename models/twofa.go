// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/base64"
	"time"

	"github.com/Unknwon/com"
	"github.com/go-xorm/xorm"
	"github.com/pquerna/otp/totp"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"
)

// Twofa represents a two-factor authentication token.
type Twofa struct {
	ID           int64 `xorm:"pk autoincr"`
	UID          int64 `xorm:"UNIQUE INDEX"`
	Secret       string
	ScratchToken string

	Created     time.Time `xorm:"-"`
	CreatedUnix int64     `xorm:"INDEX"`
	Updated     time.Time `xorm:"-"` // Note: Updated must below Created for AfterSet.
	UpdatedUnix int64     `xorm:"INDEX"`
}

// BeforeInsert will be invoked by XORM before inserting a record representing this object.
func (t *Twofa) BeforeInsert() {
	t.CreatedUnix = time.Now().Unix()
}

// BeforeUpdate is invoked from XORM before updating this object.
func (t *Twofa) BeforeUpdate() {
	t.UpdatedUnix = time.Now().Unix()
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (t *Twofa) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "created_unix":
		t.Created = time.Unix(t.CreatedUnix, 0).Local()
	case "updated_unix":
		t.Updated = time.Unix(t.UpdatedUnix, 0).Local()
	}
}

// GenerateScratchToken recreates the scratch token the user is using.
func (t *Twofa) GenerateScratchToken() error {
	token, err := base.GetRandomString(8)
	if err != nil {
		return err
	}
	t.ScratchToken = token
	return nil
}

// VerifyScratchToken verifies if the specified scratch token is valid.
func (t *Twofa) VerifyScratchToken(token string) bool {
	if len(token) == 0 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(t.ScratchToken)) == 1
}

func (t *Twofa) getEncryptionKey() []byte {
	k := md5.Sum([]byte(setting.SecretKey))
	return k[:]
}

// SetSecret sets the 2FA secret.
func (t *Twofa) SetSecret(secret string) error {
	secretBytes, err := com.AESEncrypt(t.getEncryptionKey(), []byte(secret))
	if err != nil {
		return err
	}
	t.Secret = base64.StdEncoding.EncodeToString(secretBytes)
	return nil
}

// ValidateTOTP validates the provided passcode.
func (t *Twofa) ValidateTOTP(passcode string) (bool, error) {
	decodedStoredSecret, err := base64.StdEncoding.DecodeString(t.Secret)
	if err != nil {
		return false, err
	}
	secret, err := com.AESDecrypt(t.getEncryptionKey(), decodedStoredSecret)
	if err != nil {
		return false, err
	}
	secretStr := string(secret)
	return totp.Validate(passcode, secretStr), nil
}

// NewTwofa creates a new two-factor authentication token.
func NewTwofa(t *Twofa) error {
	err := t.GenerateScratchToken()
	if err != nil {
		return err
	}
	_, err = x.Insert(t)
	return err
}

// UpdateTwofa updates a two-factor authentication token.
func UpdateTwofa(t *Twofa) error {
	_, err := x.Id(t.ID).AllCols().Update(t)
	return err
}

// GetTwofaByUID returns the two-factor authentication token associated with
// the user, if any.
func GetTwofaByUID(uid int64) (*Twofa, error) {
	twofa := &Twofa{UID: uid}
	has, err := x.Get(twofa)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTwofaNotEnrolled{uid}
	}
	return twofa, nil
}

// DeleteTwofaByID deletes two-factor authentication token by given ID.
func DeleteTwofaByID(id, userID int64) error {
	cnt, err := x.Id(id).Delete(&Twofa{
		UID: userID,
	})
	if err != nil {
		return err
	} else if cnt != 1 {
		return ErrTwofaNotEnrolled{userID}
	}
	return nil
}
