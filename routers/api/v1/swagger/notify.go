// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// NotificationThread
// swagger:response NotificationThread
type swaggerNotificationThread struct {
	// in:body
	Body api.NotificationThread `json:"body"`
}

// NotificationThreadList
// swagger:response NotificationThreadList
type swaggerNotificationThreadList struct {
	// in:body
	Body []api.NotificationThread `json:"body"`
}

// Number of unread notifications
// swagger:response NotificationCount
type swaggerNotificationCount struct {
	// in:body
	Body api.NotificationCount `json:"body"`
}
