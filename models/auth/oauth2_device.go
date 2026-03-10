// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm"
)

type OAuth2Device struct {
	ID             int64  `xorm:"pk autoincr"`
	DeviceCode     string `xorm:"-"`
	DeviceCodeHash string `xorm:"UNIQUE"` // sha256 of device code
	DeviceCodeSalt string
	DeviceCodeID   string             `xorm:"INDEX"`
	UserCode       string             `xorm:"INDEX VARCHAR(9)"`
	Application    *OAuth2Application `xorm:"-"`
	ApplicationID  int64              `xorm:"INDEX"`
	Scope          string             `xorm:"TEXT"`
	GrantID        int64
	CreatedUnix    timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix    timeutil.TimeStamp `xorm:"updated"`
	ExpiredUnix    timeutil.TimeStamp
}

// TableName sets the table name to `oauth2_device`
func (device *OAuth2Device) TableName() string {
	return "oauth2_device"
}

func (device *OAuth2Device) LoadApplication(ctx context.Context) error {
	if device.Application != nil {
		return nil
	}

	application := new(OAuth2Application)
	has, err := db.GetEngine(ctx).ID(device.ApplicationID).Get(application)
	if err != nil {
		return err
	}
	if !has {
		return &ErrOAuthApplicationNotFound{ID: device.ApplicationID}
	}

	device.Application = application

	return nil
}

func generateUserCode() (string, error) {
	rBytes, err := util.CryptoRandomBytes(8)
	if err != nil {
		return "", err
	}
	letters := "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, 8)
	for i := range code {
		code[i] = letters[int(rBytes[i])%len(letters)]
	}
	return fmt.Sprintf("%s-%s", string(code[:4]), string(code[4:])), nil
}

// CreateDevice generates a device for an user
func (app *OAuth2Application) CreateDevice(ctx context.Context, scope string) (*OAuth2Device, error) {
	userCode, err := generateUserCode()
	if err != nil {
		return nil, err
	}

	device := &OAuth2Device{
		ApplicationID: app.ID,
		UserCode:      userCode,
		Scope:         scope,
		ExpiredUnix:   timeutil.TimeStampNow().Add(setting.OAuth2.DeviceFlowExpirationTime),
	}

	salt, err := util.CryptoRandomString(10)
	if err != nil {
		return nil, err
	}
	code, err := util.CryptoRandomBytes(20)
	if err != nil {
		return nil, err
	}

	device.DeviceCode = hex.EncodeToString(code)
	device.DeviceCodeSalt = salt
	device.DeviceCodeID = device.DeviceCode[len(device.DeviceCode)-8:]
	device.DeviceCodeHash = HashToken(device.DeviceCode, device.DeviceCodeSalt)

	err = db.Insert(ctx, device)
	if err != nil {
		return nil, err
	}

	return device, nil
}

func (app *OAuth2Application) GetDeviceByDeviceCode(ctx context.Context, deviceCode string) (*OAuth2Device, error) {
	if len(deviceCode) != 40 {
		return nil, &ErrOAuth2DeviceNotFound{UserCode: deviceCode}
	}
	for _, x := range []byte(deviceCode) {
		if x < '0' || (x > '9' && x < 'a') || x > 'f' {
			return nil, &ErrOAuth2DeviceNotFound{UserCode: deviceCode}
		}
	}

	deviceCodeID := deviceCode[len(deviceCode)-8:]
	var deviceList []OAuth2Device
	err := db.GetEngine(ctx).Table(&OAuth2Device{}).Where("device_code_id = ? AND application_id = ?", deviceCodeID, app.ID).Find(&deviceList)
	if err != nil {
		return nil, err
	} else if len(deviceList) == 0 {
		return nil, &ErrOAuth2DeviceNotFound{UserCode: deviceCode}
	}

	for _, t := range deviceList {
		tempHash := HashToken(deviceCode, t.DeviceCodeSalt)
		if subtle.ConstantTimeCompare([]byte(t.DeviceCodeHash), []byte(tempHash)) == 1 {
			return &t, nil
		}
	}

	return nil, &ErrOAuth2DeviceNotFound{UserCode: deviceCode}
}

func (device *OAuth2Device) GetGrant(ctx context.Context) (*OAuth2DeviceGrant, error) {
	if device.GrantID <= 0 {
		return nil, fmt.Errorf("no grant found for device: %d", device.ID)
	}

	grant := new(OAuth2DeviceGrant)
	_, err := db.GetEngine(ctx).ID(device.GrantID).Get(grant)

	return grant, err
}

type ErrOAuth2DeviceNotFound struct {
	UserCode string
	ID       int64
}

func (err *ErrOAuth2DeviceNotFound) Error() string {
	return fmt.Sprintf("oauth2 device not found: [user_code: %s. id: %d]", err.UserCode, err.ID)
}

func IsErrOAuth2DeviceNotFound(err error) bool {
	_, ok := err.(*ErrOAuth2DeviceNotFound)
	return ok
}

func GetDeviceByUserCode(ctx context.Context, userCode string) (*OAuth2Device, error) {
	device := new(OAuth2Device)

	ok, err := db.GetEngine(ctx).Where("user_code = ? AND grant_id = ? AND expired_unix > ?",
		userCode, 0, timeutil.TimeStampNow()).Get(device)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &ErrOAuth2DeviceNotFound{UserCode: userCode}
	}

	return device, nil
}

func GetDeviceByID(ctx context.Context, id int64) (*OAuth2Device, error) {
	device := new(OAuth2Device)
	ok, err := db.GetEngine(ctx).ID(id).Get(device)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &ErrOAuth2DeviceNotFound{ID: id}
	}

	return device, nil
}

type OAuth2DeviceGrant struct {
	ID            int64              `xorm:"pk autoincr"`
	UserID        int64              `xorm:"INDEX"`
	Application   *OAuth2Application `xorm:"-"`
	ApplicationID int64              `xorm:"INDEX"`
	DeviceID      int64              `xorm:"INDEX"`
	Counter       int64              `xorm:"NOT NULL DEFAULT 1"`
	UserCode      string             `xorm:"VARCHAR(9)"`
	Scope         string             `xorm:"TEXT"`
	Nonce         string             `xorm:"TEXT"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
}

// TableName sets theOAuth2DeviceGrant table name to `oauth2_device_grant`
func (grant *OAuth2DeviceGrant) TableName() string {
	return "oauth2_device_grant"
}

func (device *OAuth2Device) CreateGrant(ctx context.Context, userID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		grant := &OAuth2DeviceGrant{
			UserID:        userID,
			DeviceID:      device.ID,
			ApplicationID: device.ApplicationID,
			Scope:         device.Scope,
			UserCode:      device.UserCode,
		}
		err := db.Insert(ctx, grant)
		if err != nil {
			return err
		}

		device.GrantID = grant.ID
		_, err = db.GetEngine(ctx).ID(device.ID).Cols("grant_id").Update(device)
		return err
	})
}

// GetOAuth2GrantByID returns the grant with the given ID
func GetOAuth2DeviceGrantByID(ctx context.Context, id int64) (grant *OAuth2DeviceGrant, err error) {
	grant = new(OAuth2DeviceGrant)
	if has, err := db.GetEngine(ctx).ID(id).Get(grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return grant, err
}

func (grant *OAuth2DeviceGrant) IncreaseCounter(ctx context.Context) error {
	_, err := db.GetEngine(ctx).ID(grant.ID).Incr("counter").Update(new(OAuth2Grant))
	if err != nil {
		return err
	}
	updatedGrant, err := GetOAuth2DeviceGrantByID(ctx, grant.ID)
	if err != nil {
		return err
	}
	grant.Counter = updatedGrant.Counter
	return nil
}

func (grant *OAuth2DeviceGrant) GetID() int64 {
	return -grant.ID
}

func (grant *OAuth2DeviceGrant) GetCounter() int64 {
	return grant.Counter
}

func (grant *OAuth2DeviceGrant) ScopeContains(scope string) bool {
	return slices.Contains(strings.Split(grant.Scope, " "), scope)
}

func (grant *OAuth2DeviceGrant) GetApplicationID() int64 {
	return grant.ApplicationID
}

func (grant *OAuth2DeviceGrant) GetUserID() int64 {
	return grant.UserID
}

func (grant *OAuth2DeviceGrant) GetNonce() string {
	return grant.Nonce
}

func (grant *OAuth2DeviceGrant) GetScope() string {
	return grant.Scope
}

// GetOAuth2GrantsByUserID lists all grants of a certain user
func GetOAuth2DeviceGrantsByUserID(ctx context.Context, uid int64) ([]*OAuth2DeviceGrant, error) {
	type joinedOAuth2DeviceGrant struct {
		Grant       *OAuth2DeviceGrant `xorm:"extends"`
		Application *OAuth2Application `xorm:"extends"`
	}
	var results *xorm.Rows
	var err error
	if results, err = db.GetEngine(ctx).
		Table("oauth2_device_grant").
		Where("user_id = ?", uid).
		Join("INNER", "oauth2_application", "application_id = oauth2_application.id").
		Rows(new(joinedOAuth2DeviceGrant)); err != nil {
		return nil, err
	}
	defer results.Close()
	grants := make([]*OAuth2DeviceGrant, 0)
	for results.Next() {
		joinedGrant := new(joinedOAuth2DeviceGrant)
		if err := results.Scan(joinedGrant); err != nil {
			return nil, err
		}
		joinedGrant.Grant.Application = joinedGrant.Application
		grants = append(grants, joinedGrant.Grant)
	}
	return grants, nil
}

// RevokeOAuth2Grant deletes the device grant with grantID and userID
func RevokeOAuth2DeviceGrant(ctx context.Context, grantID, userID int64) error {
	if grantID <= 0 {
		return errors.New("invalid grant ID")
	}

	return db.WithTx(ctx, func(ctx context.Context) error {
		_, err := db.GetEngine(ctx).Where(builder.Eq{"grant_id": grantID}).
			Delete(&OAuth2Device{})
		if err != nil {
			return err
		}

		_, err = db.GetEngine(ctx).Where(builder.Eq{"id": grantID, "user_id": userID}).
			Delete(&OAuth2DeviceGrant{})
		return err
	})
}
