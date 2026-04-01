// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

const (
	oauth2DeviceAuthorizationValidity           = 10 * time.Minute
	oauth2DeviceAuthorizationIntervalSeconds    = 5
	oauth2DeviceAuthorizationSlowDownSeconds    = 5
	oauth2DeviceAuthorizationMaxIntervalSeconds = 300
	oauth2DeviceAuthorizationFormattedCodeParts = 2
	oauth2DeviceAuthorizationCodePartLength     = 4
	oauth2DeviceAuthorizationUserCodeLength     = oauth2DeviceAuthorizationFormattedCodeParts * oauth2DeviceAuthorizationCodePartLength
)

type OAuth2DeviceAuthorizationStatus string

const (
	OAuth2DeviceAuthorizationPending  OAuth2DeviceAuthorizationStatus = "pending"
	OAuth2DeviceAuthorizationApproved OAuth2DeviceAuthorizationStatus = "approved"
	OAuth2DeviceAuthorizationDenied   OAuth2DeviceAuthorizationStatus = "denied"
	OAuth2DeviceAuthorizationConsumed OAuth2DeviceAuthorizationStatus = "consumed"
)

var ErrOAuth2DeviceAuthorizationInvalidated = errors.New("oauth2 device authorization changed state")

// DeleteExpiredDeviceAuthorizations removes device authorizations that are
// expired or in a terminal state (denied/consumed).
func DeleteExpiredDeviceAuthorizations(ctx context.Context) error {
	_, err := db.GetEngine(ctx).Where(
		"expires_at_unix < ? OR status IN (?, ?)",
		timeutil.TimeStampNow(),
		OAuth2DeviceAuthorizationDenied,
		OAuth2DeviceAuthorizationConsumed,
	).Delete(new(OAuth2DeviceAuthorization))
	return err
}

const oauth2DeviceAuthorizationUserCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// OAuth2DeviceAuthorization stores state for the OAuth device authorization flow.
type OAuth2DeviceAuthorization struct {
	ID                  int64                           `xorm:"pk autoincr"`
	ApplicationID       int64                           `xorm:"INDEX"`
	UserID              int64                           `xorm:"INDEX"`
	GrantID             int64                           `xorm:"INDEX"`
	DeviceCodeHash      string                          `xorm:"unique"`
	UserCode            string                          `xorm:"unique"`
	Scope               string                          `xorm:"TEXT"`
	Status              OAuth2DeviceAuthorizationStatus `xorm:"INDEX NOT NULL"`
	PollIntervalSeconds int64                           `xorm:"NOT NULL DEFAULT 5"`
	LastPolledUnix      timeutil.TimeStamp              `xorm:"INDEX"`
	ExpiresAtUnix       timeutil.TimeStamp              `xorm:"INDEX"`
	CreatedUnix         timeutil.TimeStamp              `xorm:"created"`
	UpdatedUnix         timeutil.TimeStamp              `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(OAuth2DeviceAuthorization))
}

// TableName sets the table name to `oauth2_device_authorization`.
func (d *OAuth2DeviceAuthorization) TableName() string {
	return "oauth2_device_authorization"
}

// IsExpired reports whether the device authorization is expired.
func (d *OAuth2DeviceAuthorization) IsExpired() bool {
	if d.ExpiresAtUnix.IsZero() {
		return true
	}
	return d.ExpiresAtUnix <= timeutil.TimeStampNow()
}

// FormattedUserCode returns the user-facing device code.
func (d *OAuth2DeviceAuthorization) FormattedUserCode() string {
	normalized := NormalizeOAuth2DeviceUserCode(d.UserCode)
	if len(normalized) != oauth2DeviceAuthorizationFormattedCodeParts*oauth2DeviceAuthorizationCodePartLength {
		return normalized
	}
	return normalized[:oauth2DeviceAuthorizationCodePartLength] + "-" + normalized[oauth2DeviceAuthorizationCodePartLength:]
}

// RegisterPoll updates the device authorization with the current poll time.
// It returns true if the client should be slowed down.
func (d *OAuth2DeviceAuthorization) RegisterPoll(ctx context.Context) (bool, error) {
	now := timeutil.TimeStampNow()
	if !d.LastPolledUnix.IsZero() && d.LastPolledUnix+timeutil.TimeStamp(d.PollIntervalSeconds) > now {
		d.PollIntervalSeconds = min(d.PollIntervalSeconds+oauth2DeviceAuthorizationSlowDownSeconds, oauth2DeviceAuthorizationMaxIntervalSeconds)
		d.LastPolledUnix = now
		_, err := db.GetEngine(ctx).ID(d.ID).Cols("poll_interval_seconds", "last_polled_unix").Update(d)
		return true, err
	}

	d.LastPolledUnix = now
	_, err := db.GetEngine(ctx).ID(d.ID).Cols("last_polled_unix").Update(d)
	return false, err
}

// MarkApproved persists the approved device authorization.
func (d *OAuth2DeviceAuthorization) MarkApproved(ctx context.Context, grantID, userID int64) error {
	d.GrantID = grantID
	d.UserID = userID
	d.Status = OAuth2DeviceAuthorizationApproved
	affected, err := db.GetEngine(ctx).Where("id = ? AND status = ?", d.ID, OAuth2DeviceAuthorizationPending).
		Cols("grant_id", "user_id", "status").Update(d)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrOAuth2DeviceAuthorizationInvalidated
	}
	return nil
}

// MarkDenied persists the denied device authorization.
func (d *OAuth2DeviceAuthorization) MarkDenied(ctx context.Context, userID int64) error {
	d.UserID = userID
	d.Status = OAuth2DeviceAuthorizationDenied
	affected, err := db.GetEngine(ctx).Where("id = ? AND status = ?", d.ID, OAuth2DeviceAuthorizationPending).
		Cols("user_id", "status").Update(d)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrOAuth2DeviceAuthorizationInvalidated
	}
	return nil
}

// MarkConsumed persists the consumed device authorization.
func (d *OAuth2DeviceAuthorization) MarkConsumed(ctx context.Context) error {
	d.Status = OAuth2DeviceAuthorizationConsumed
	affected, err := db.GetEngine(ctx).Where("id = ? AND status = ?", d.ID, OAuth2DeviceAuthorizationApproved).
		Cols("status").Update(d)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrOAuth2DeviceAuthorizationInvalidated
	}
	return nil
}

// CreateOAuth2DeviceAuthorization creates a new device authorization and returns the plaintext device code.
func CreateOAuth2DeviceAuthorization(ctx context.Context, app *OAuth2Application, scope string) (*OAuth2DeviceAuthorization, string, error) {
	deviceCode, err := generateOAuth2DeviceCode()
	if err != nil {
		return nil, "", err
	}

	// Retry user code generation a few times in case of unique constraint collision.
	const maxRetries = 5
	for range maxRetries {
		userCode, err := generateOAuth2UserCode()
		if err != nil {
			return nil, "", err
		}

		// Check for collision before inserting.
		existing, err := GetOAuth2DeviceAuthorizationByUserCode(ctx, userCode)
		if err != nil {
			return nil, "", err
		}
		if existing != nil {
			continue
		}

		deviceAuthorization := &OAuth2DeviceAuthorization{
			ApplicationID:       app.ID,
			DeviceCodeHash:      hashOAuth2DeviceCode(deviceCode),
			UserCode:            userCode,
			Scope:               strings.TrimSpace(scope),
			Status:              OAuth2DeviceAuthorizationPending,
			PollIntervalSeconds: oauth2DeviceAuthorizationIntervalSeconds,
			ExpiresAtUnix:       timeutil.TimeStamp(time.Now().Add(oauth2DeviceAuthorizationValidity).Unix()),
		}

		if err := db.Insert(ctx, deviceAuthorization); err != nil {
			return nil, "", err
		}

		return deviceAuthorization, deviceCode, nil
	}

	return nil, "", errors.New("failed to generate unique device user code after retries")
}

// GetOAuth2DeviceAuthorizationByID returns the device authorization with the given ID.
func GetOAuth2DeviceAuthorizationByID(ctx context.Context, id int64) (*OAuth2DeviceAuthorization, error) {
	deviceAuthorization := new(OAuth2DeviceAuthorization)
	if has, err := db.GetEngine(ctx).ID(id).Get(deviceAuthorization); err != nil {
		return nil, err
	} else if !has {
		return nil, nil //nolint:nilnil // return nil to indicate that the object does not exist
	}
	return deviceAuthorization, nil
}

// GetOAuth2DeviceAuthorizationByDeviceCode returns the device authorization matching the device code.
func GetOAuth2DeviceAuthorizationByDeviceCode(ctx context.Context, deviceCode string) (*OAuth2DeviceAuthorization, error) {
	deviceAuthorization := new(OAuth2DeviceAuthorization)
	if has, err := db.GetEngine(ctx).Where("device_code_hash = ?", hashOAuth2DeviceCode(deviceCode)).Get(deviceAuthorization); err != nil {
		return nil, err
	} else if !has {
		return nil, nil //nolint:nilnil // return nil to indicate that the object does not exist
	}
	return deviceAuthorization, nil
}

// GetOAuth2DeviceAuthorizationByUserCode returns the device authorization matching the user code.
func GetOAuth2DeviceAuthorizationByUserCode(ctx context.Context, userCode string) (*OAuth2DeviceAuthorization, error) {
	deviceAuthorization := new(OAuth2DeviceAuthorization)
	normalized := NormalizeOAuth2DeviceUserCode(userCode)
	if normalized == "" {
		return nil, nil //nolint:nilnil // return nil to indicate that the object does not exist
	}
	if has, err := db.GetEngine(ctx).Where("user_code = ?", normalized).Get(deviceAuthorization); err != nil {
		return nil, err
	} else if !has {
		return nil, nil //nolint:nilnil // return nil to indicate that the object does not exist
	}
	return deviceAuthorization, nil
}

// NormalizeOAuth2DeviceUserCode normalizes the user-visible device code for storage and lookup.
func NormalizeOAuth2DeviceUserCode(userCode string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(userCode)))
}

func generateOAuth2DeviceCode() (string, error) {
	rBytes, err := util.CryptoRandomBytes(32)
	if err != nil {
		return "", err
	}
	return "gtd_" + base32Lower.EncodeToString(rBytes), nil
}

func generateOAuth2UserCode() (string, error) {
	buf := make([]byte, oauth2DeviceAuthorizationUserCodeLength)
	limit := int64(len(oauth2DeviceAuthorizationUserCodeAlphabet))
	for i := range buf {
		num, err := util.CryptoRandomInt(limit)
		if err != nil {
			return "", err
		}
		buf[i] = oauth2DeviceAuthorizationUserCodeAlphabet[num]
	}
	return string(buf), nil
}

func hashOAuth2DeviceCode(deviceCode string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(deviceCode)))
	return hex.EncodeToString(hash[:])
}
