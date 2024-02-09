// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/minio/sha256-simd"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/pbkdf2"
)

//
// Two-factor authentication
//

// ErrTwoFactorNotEnrolled indicates that a user is not enrolled in two-factor authentication.
type ErrTwoFactorNotEnrolled struct {
	UID int64
}

// IsErrTwoFactorNotEnrolled checks if an error is a ErrTwoFactorNotEnrolled.
func IsErrTwoFactorNotEnrolled(err error) bool {
	_, ok := err.(ErrTwoFactorNotEnrolled)
	return ok
}

func (err ErrTwoFactorNotEnrolled) Error() string {
	return fmt.Sprintf("user not enrolled in 2FA [uid: %d]", err.UID)
}

// Unwrap unwraps this as a ErrNotExist err
func (err ErrTwoFactorNotEnrolled) Unwrap() error {
	return util.ErrNotExist
}

// TwoFactor represents a two-factor authentication token.
type TwoFactor struct {
	ID               int64 `xorm:"pk autoincr"`
	UID              int64 `xorm:"UNIQUE"`
	Secret           string
	ScratchSalt      string
	ScratchHash      string
	LastUsedPasscode string             `xorm:"VARCHAR(10)"`
	CreatedUnix      timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix      timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(TwoFactor))
}

// GenerateScratchToken recreates the scratch token the user is using.
func (t *TwoFactor) GenerateScratchToken() (string, error) {
	tokenBytes, err := util.CryptoRandomBytes(6)
	if err != nil {
		return "", err
	}
	// these chars are specially chosen, avoid ambiguous chars like `0`, `O`, `1`, `I`.
	const base32Chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	token := base32.NewEncoding(base32Chars).WithPadding(base32.NoPadding).EncodeToString(tokenBytes)
	t.ScratchSalt, _ = util.CryptoRandomString(10)
	t.ScratchHash = HashToken(token, t.ScratchSalt)
	return token, nil
}

// HashToken return the hashable salt
func HashToken(token, salt string) string {
	tempHash := pbkdf2.Key([]byte(token), []byte(salt), 10000, 50, sha256.New)
	return hex.EncodeToString(tempHash)
}

// VerifyScratchToken verifies if the specified scratch token is valid.
func (t *TwoFactor) VerifyScratchToken(token string) bool {
	if len(token) == 0 {
		return false
	}
	tempHash := HashToken(token, t.ScratchSalt)
	return subtle.ConstantTimeCompare([]byte(t.ScratchHash), []byte(tempHash)) == 1
}

func (t *TwoFactor) getEncryptionKey() []byte {
	k := md5.Sum([]byte(setting.SecretKey))
	return k[:]
}

// SetSecret sets the 2FA secret.
func (t *TwoFactor) SetSecret(secretString string) error {
	secretBytes, err := secret.AesEncrypt(t.getEncryptionKey(), []byte(secretString))
	if err != nil {
		return err
	}
	t.Secret = base64.StdEncoding.EncodeToString(secretBytes)
	return nil
}

// ValidateTOTP validates the provided passcode.
func (t *TwoFactor) ValidateTOTP(passcode string) (bool, error) {
	decodedStoredSecret, err := base64.StdEncoding.DecodeString(t.Secret)
	if err != nil {
		return false, err
	}
	secretBytes, err := secret.AesDecrypt(t.getEncryptionKey(), decodedStoredSecret)
	if err != nil {
		return false, err
	}
	secretStr := string(secretBytes)
	return totp.Validate(passcode, secretStr), nil
}

// NewTwoFactor creates a new two-factor authentication token.
func NewTwoFactor(ctx context.Context, t *TwoFactor) error {
	_, err := db.GetEngine(ctx).Insert(t)
	return err
}

// UpdateTwoFactor updates a two-factor authentication token.
func UpdateTwoFactor(ctx context.Context, t *TwoFactor) error {
	_, err := db.GetEngine(ctx).ID(t.ID).AllCols().Update(t)
	return err
}

// GetTwoFactorByUID returns the two-factor authentication token associated with
// the user, if any.
func GetTwoFactorByUID(ctx context.Context, uid int64) (*TwoFactor, error) {
	twofa := &TwoFactor{}
	has, err := db.GetEngine(ctx).Where("uid=?", uid).Get(twofa)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTwoFactorNotEnrolled{uid}
	}
	return twofa, nil
}

// HasTwoFactorByUID returns the two-factor authentication token associated with
// the user, if any.
func HasTwoFactorByUID(ctx context.Context, uid int64) (bool, error) {
	return db.GetEngine(ctx).Where("uid=?", uid).Exist(&TwoFactor{})
}

// DeleteTwoFactorByID deletes two-factor authentication token by given ID.
func DeleteTwoFactorByID(ctx context.Context, id, userID int64) error {
	cnt, err := db.GetEngine(ctx).ID(id).Delete(&TwoFactor{
		UID: userID,
	})
	if err != nil {
		return err
	} else if cnt != 1 {
		return ErrTwoFactorNotEnrolled{userID}
	}
	return nil
}
