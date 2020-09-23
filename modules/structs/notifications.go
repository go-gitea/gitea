// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// NotificationThread expose Notification on API
type NotificationThread struct {
	ID         int64                `json:"id"`
	Repository *Repository          `json:"repository"`
	Subject    *NotificationSubject `json:"subject"`
	Unread     bool                 `json:"unread"`
	Pinned     bool                 `json:"pinned"`
	UpdatedAt  time.Time            `json:"updated_at"`
	URL        string               `json:"url"`
}

// NotificationSubject contains the notification subject (Issue/Pull/Commit)
type NotificationSubject struct {
	Title            string    `json:"title"`
	URL              string    `json:"url"`
	LatestCommentURL string    `json:"latest_comment_url"`
	Type             string    `json:"type" binding:"In(Issue,Pull,Commit)"`
	State            StateType `json:"state"`
}

// NotificationCount number of unread notifications
type NotificationCount struct {
	New int64 `json:"new"`
}
