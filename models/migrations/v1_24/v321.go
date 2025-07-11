// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24

import (
	"context"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/xorm"
)

type NotificationSettings struct {
	UserID  int64  `xorm:"pk"`
	Actions string `xorm:"NOT NULL DEFAULT 'failureonly'"`
}

func (*NotificationSettings) TableName() string {
	return "user_notification_settings"
}

func AddFineGrainedActionsNotificationSettings(x *xorm.Engine) error {
	if err := x.Sync(&NotificationSettings{}); err != nil {
		return err
	}

	settings := make([]NotificationSettings, 0, 100)

	type User struct {
		ID int64 `xorm:"pk autoincr"`
	}

	if err := db.Iterate(context.Background(), nil, func(ctx context.Context, user *User) error {
		settings = append(settings, NotificationSettings{
			UserID:  user.ID,
			Actions: user_model.NotificationActionsFailureOnly,
		})
		if len(settings) >= 100 {
			_, err := x.Insert(&settings)
			if err != nil {
				return err
			}
			settings = settings[:0]
		}
		return nil
	}); err != nil {
		return err
	}

	if len(settings) > 0 {
		if _, err := x.Insert(&settings); err != nil {
			return err
		}
	}

	return nil
}
