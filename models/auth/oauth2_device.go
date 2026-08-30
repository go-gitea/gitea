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

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
)

const (
	oauth2DeviceAuthorizationValidity           = 10 * time.Minute
	oauth2DeviceAuthorizationIntervalSeconds    = 5
	oauth2DeviceAuthorizationSlowDownSeconds    = 5
	oauth2DeviceAuthorizationMaxIntervalSeconds = 300
	oauth2DeviceAuthorizationUserCodeLength     = 8
)

// OAuth2DeviceAuthorizationMaxPendingPerRequester bounds how many rows a single caller can
// hold open for one client. The device authorization endpoint is unauthenticated and client
// IDs are public, so a per-app-only cap would let anyone lock out a client's real users.
var OAuth2DeviceAuthorizationMaxPendingPerRequester int64 = 100

type OAuth2DeviceAuthorizationStatus string

const (
	OAuth2DeviceAuthorizationPending  OAuth2DeviceAuthorizationStatus = "pending"
	OAuth2DeviceAuthorizationApproved OAuth2DeviceAuthorizationStatus = "approved"
	OAuth2DeviceAuthorizationDenied   OAuth2DeviceAuthorizationStatus = "denied"
	OAuth2DeviceAuthorizationConsumed OAuth2DeviceAuthorizationStatus = "consumed"
)

var (
	ErrOAuth2DeviceAuthorizationInvalidated  = errors.New("oauth2 device authorization changed state")
	ErrOAuth2DeviceAuthorizationLimitReached = errors.New("too many pending oauth2 device authorizations")
)

// deleteExpiredDeviceAuthorizations removes device authorizations past their validity.
// Denied and consumed rows are kept until they expire so a polling client still learns
// why it was rejected instead of seeing the code vanish.
func deleteExpiredDeviceAuthorizations(ctx context.Context) error {
	_, err := db.GetEngine(ctx).Where("expires_at_unix < ?", timeutil.TimeStampNow()).
		Delete(new(OAuth2DeviceAuthorization))
	return err
}

// OAuth2DeviceAuthorization stores state for the OAuth device authorization flow.
type OAuth2DeviceAuthorization struct {
	ID                  int64 `xorm:"pk autoincr"`
	ApplicationID       int64 `xorm:"INDEX"`
	UserID              int64 `xorm:"INDEX"`
	GrantID             int64
	RequesterIP         string
	DeviceCodeHash      string                          `xorm:"unique"`
	UserCode            string                          `xorm:"unique"`
	Scope               string                          `xorm:"TEXT"`
	Status              OAuth2DeviceAuthorizationStatus `xorm:"NOT NULL"`
	PollIntervalSeconds int64                           `xorm:"NOT NULL DEFAULT 5"`
	LastPolledUnix      timeutil.TimeStamp
	ExpiresAtUnix       timeutil.TimeStamp `xorm:"INDEX"`
	CreatedUnix         timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix         timeutil.TimeStamp `xorm:"updated"`
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
	if len(normalized) != oauth2DeviceAuthorizationUserCodeLength {
		return normalized
	}
	half := oauth2DeviceAuthorizationUserCodeLength / 2
	return normalized[:half] + "-" + normalized[half:]
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
func CreateOAuth2DeviceAuthorization(ctx context.Context, app *OAuth2Application, scope, requesterIP string) (*OAuth2DeviceAuthorization, string, error) {
	if err := deleteExpiredDeviceAuthorizations(ctx); err != nil {
		return nil, "", err
	}

	pending, err := db.GetEngine(ctx).Where(
		"application_id = ? AND requester_ip = ? AND status = ? AND expires_at_unix > ?",
		app.ID, requesterIP, OAuth2DeviceAuthorizationPending, timeutil.TimeStampNow(),
	).Count(new(OAuth2DeviceAuthorization))
	if err != nil {
		return nil, "", err
	}
	if pending >= OAuth2DeviceAuthorizationMaxPendingPerRequester {
		return nil, "", ErrOAuth2DeviceAuthorizationLimitReached
	}

	deviceCode := generateOAuth2DeviceCode()

	// Retry user code generation a few times in case of unique constraint collision.
	const maxRetries = 5
	for range maxRetries {
		userCode := generateOAuth2UserCode()

		// Check for collision before inserting.
		switch _, err := GetOAuth2DeviceAuthorizationByUserCode(ctx, userCode); {
		case err == nil:
			continue
		case !errors.Is(err, util.ErrNotExist):
			return nil, "", err
		}

		deviceAuthorization := &OAuth2DeviceAuthorization{
			ApplicationID:       app.ID,
			RequesterIP:         requesterIP,
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
		return nil, util.NewNotExistErrorf("device authorization by ID not found")
	}
	return deviceAuthorization, nil
}

// GetOAuth2DeviceAuthorizationByDeviceCode returns the device authorization matching the device code.
func GetOAuth2DeviceAuthorizationByDeviceCode(ctx context.Context, deviceCode string) (*OAuth2DeviceAuthorization, error) {
	deviceAuthorization := new(OAuth2DeviceAuthorization)
	if has, err := db.GetEngine(ctx).Where("device_code_hash = ?", hashOAuth2DeviceCode(deviceCode)).Get(deviceAuthorization); err != nil {
		return nil, err
	} else if !has {
		return nil, util.NewNotExistErrorf("device authorization by device code not found")
	}
	return deviceAuthorization, nil
}

// GetOAuth2DeviceAuthorizationByUserCode returns the device authorization matching the user code.
func GetOAuth2DeviceAuthorizationByUserCode(ctx context.Context, userCode string) (*OAuth2DeviceAuthorization, error) {
	deviceAuthorization := new(OAuth2DeviceAuthorization)
	normalized := NormalizeOAuth2DeviceUserCode(userCode)
	if has, err := db.GetEngine(ctx).Where("user_code = ?", normalized).Get(deviceAuthorization); err != nil {
		return nil, err
	} else if !has {
		return nil, util.NewNotExistErrorf("device authorization by user code not found")
	}
	return deviceAuthorization, nil
}

// NormalizeOAuth2DeviceUserCode normalizes the user-visible device code for storage and lookup.
func NormalizeOAuth2DeviceUserCode(userCode string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(userCode)))
}

func generateOAuth2DeviceCode() string {
	return "gtd_" + base32Lower.EncodeToString(util.FastCryptoRandomBytes(32))
}

func generateOAuth2UserCode() string {
	const chars = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	buf := make([]byte, oauth2DeviceAuthorizationUserCodeLength)
	for i := range buf {
		buf[i] = chars[util.FastCryptoRandomInt(len(chars))]
	}
	return string(buf)
}

func hashOAuth2DeviceCode(deviceCode string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(deviceCode)))
	return hex.EncodeToString(hash[:])
}
