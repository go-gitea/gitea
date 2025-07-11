// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

// Actions email preference
const (
	NotificationGiteaActionsAll         = "all"
	NotificationGiteaActionsFailureOnly = "failureonly"
	NotificationGiteaActionsDisabled    = "disabled"
)

type NotificationSettings struct {
	UserID  int64  `xorm:"pk"`
	User    *User  `xorm:"-"`
	Actions string `xorm:"NOT NULL DEFAULT 'failureonly'"`
}

func (NotificationSettings) TableName() string {
	return "user_notification_settings"
}

func init() {
	db.RegisterModel(new(NotificationSettings))
}

// GetUserNotificationSettings returns a user's fine-grained notification preference
func GetUserNotificationSettings(ctx context.Context, userID int64) (*NotificationSettings, error) {
	settings := &NotificationSettings{}
	if has, err := db.GetEngine(ctx).Where("user_id=?", userID).Get(settings); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	user, err := GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	settings.User = user
	return settings, nil
}

func UpdateUserNotificationSettings(ctx context.Context, settings *NotificationSettings) error {
	_, err := db.GetEngine(ctx).Where("user_id = ?", settings.UserID).
		Update(&NotificationSettings{
			Actions: settings.Actions,
		})
	return err
}
