// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"

	"gitea.dev/models/db"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
)

// ManagerRegistrationToken authorizes Manager registration in a site or user scope.
type ManagerRegistrationToken struct {
	ID       int64
	Token    string `xorm:"UNIQUE"`
	UserID   int64  `xorm:"index"`
	IsActive bool

	Created timeutil.TimeStamp `xorm:"created"`
	Updated timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(ManagerRegistrationToken))
}

func (*ManagerRegistrationToken) TableName() string {
	return "codespace_manager_registration_token"
}

// GetManagerRegistrationToken returns a registration token by its exact value.
func GetManagerRegistrationToken(ctx context.Context, token string) (*ManagerRegistrationToken, error) {
	var registrationToken ManagerRegistrationToken
	has, err := db.GetEngine(ctx).Where("token = ?", token).Get(&registrationToken)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, fmt.Errorf("manager registration token: %w", util.ErrNotExist)
	}
	return &registrationToken, nil
}

// GetLatestManagerRegistrationToken returns the newest token for one scope.
func GetLatestManagerRegistrationToken(ctx context.Context, userID int64) (*ManagerRegistrationToken, error) {
	var registrationToken ManagerRegistrationToken
	has, err := db.GetEngine(ctx).Where("user_id = ?", userID).Desc("id").Get(&registrationToken)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, fmt.Errorf("manager registration token: %w", util.ErrNotExist)
	}
	return &registrationToken, nil
}

// NewManagerRegistrationToken creates an active token and invalidates the old token for the scope.
func NewManagerRegistrationToken(ctx context.Context, userID int64) (*ManagerRegistrationToken, error) {
	registrationToken := &ManagerRegistrationToken{Token: util.CryptoRandomString(40), UserID: userID, IsActive: true}
	err := db.WithTx(ctx, func(ctx context.Context) error {
		if _, err := db.GetEngine(ctx).Where("user_id = ?", userID).Cols("is_active").Update(&ManagerRegistrationToken{IsActive: false}); err != nil {
			return err
		}
		_, err := db.GetEngine(ctx).Insert(registrationToken)
		return err
	})
	return registrationToken, err
}

// GetOrCreateActiveManagerRegistrationToken returns the stable active token for one scope.
func GetOrCreateActiveManagerRegistrationToken(ctx context.Context, userID int64) (*ManagerRegistrationToken, error) {
	registrationToken, err := GetLatestManagerRegistrationToken(ctx, userID)
	if errors.Is(err, util.ErrNotExist) || (registrationToken != nil && !registrationToken.IsActive) {
		return NewManagerRegistrationToken(ctx, userID)
	}
	return registrationToken, err
}
