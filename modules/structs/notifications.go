// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// NotificationThread expose Notification on API
type NotificationThread struct {
	// ID is the unique identifier for the notification thread
	ID int64 `json:"id"`
	// Repository is the repository associated with the notification
	Repository *Repository `json:"repository"`
	// Subject contains details about the notification subject
	Subject *NotificationSubject `json:"subject"`
	// Unread indicates if the notification has been read
	Unread bool `json:"unread"`
	// Pinned indicates if the notification is pinned
	Pinned bool `json:"pinned"`
	// UpdatedAt is the time when the notification was last updated
	UpdatedAt time.Time `json:"updated_at"`
	// URL is the API URL for this notification thread
	URL string `json:"url"`
}

// NotificationSubject contains the notification subject (Issue/Pull/Commit)
type NotificationSubject struct {
	// Title is the title of the notification subject
	Title string `json:"title"`
	// URL is the API URL for the notification subject
	URL string `json:"url"`
	// LatestCommentURL is the API URL for the latest comment
	LatestCommentURL string `json:"latest_comment_url"`
	// HTMLURL is the web URL for the notification subject
	HTMLURL string `json:"html_url"`
	// LatestCommentHTMLURL is the web URL for the latest comment
	LatestCommentHTMLURL string `json:"latest_comment_html_url"`
	// Type indicates the type of the notification subject
	Type NotifySubjectType `json:"type" binding:"In(Issue,Pull,Commit,Repository)"`
	// State indicates the current state of the notification subject
	State NotifySubjectStateType `json:"state"`
}

// NotificationCount number of unread notifications
type NotificationCount struct {
	// New is the number of unread notifications
	New int64 `json:"new"`
}

// NotifySubjectStateType represents the state of a notification subject
// swagger:enum NotifySubjectStateType
type NotifySubjectStateType string

const (
	// NotifySubjectStateOpen is an open subject
	NotifySubjectStateOpen NotifySubjectStateType = "open"
	// NotifySubjectStateClosed is a closed subject
	NotifySubjectStateClosed NotifySubjectStateType = "closed"
	// NotifySubjectStateMerged is a merged pull request
	NotifySubjectStateMerged NotifySubjectStateType = "merged"
)

// NotifySubjectType represent type of notification subject
//
// swagger:enum NotifySubjectType
type NotifySubjectType string

const (
	// NotifySubjectIssue an issue is subject of an notification
	NotifySubjectIssue NotifySubjectType = "Issue"
	// NotifySubjectPull an pull is subject of an notification
	NotifySubjectPull NotifySubjectType = "Pull"
	// NotifySubjectCommit an commit is subject of an notification
	NotifySubjectCommit NotifySubjectType = "Commit"
	// NotifySubjectRepository an repository is subject of an notification
	NotifySubjectRepository NotifySubjectType = "Repository"
)
