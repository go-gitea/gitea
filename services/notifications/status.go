// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package notifications wraps activities_model notification-status mutations
// with the matching real-time push, so route handlers cannot forget either side.
package notifications

import (
	"context"

	activities_model "code.gitea.io/gitea/models/activities"
	user_model "code.gitea.io/gitea/models/user"
	notify_service "code.gitea.io/gitea/services/notify"
)

// SetIssueReadBy marks an issue as read by the user. A no-op (already read) is
// detected via the bool from the model and skips the push.
func SetIssueReadBy(ctx context.Context, issueID, userID int64) error {
	changed, err := activities_model.SetIssueReadBy(ctx, issueID, userID)
	if err != nil {
		return err
	}
	if changed {
		notify_service.NotificationCountChange(ctx, userID)
	}
	return nil
}

// SetNotificationStatus transitions one notification and pushes the new count.
func SetNotificationStatus(ctx context.Context, notificationID int64, user *user_model.User, status activities_model.NotificationStatus) (*activities_model.Notification, error) {
	notif, err := activities_model.SetNotificationStatus(ctx, notificationID, user, status)
	if err != nil {
		return notif, err
	}
	notify_service.NotificationCountChange(ctx, user.ID)
	return notif, nil
}

// SetManyNotificationStatuses transitions a batch and fires one push at the end.
// Returns the successfully-updated notifications so callers can build a response.
func SetManyNotificationStatuses(ctx context.Context, ns []*activities_model.Notification, user *user_model.User, status activities_model.NotificationStatus) ([]*activities_model.Notification, error) {
	out := make([]*activities_model.Notification, 0, len(ns))
	for _, n := range ns {
		notif, err := activities_model.SetNotificationStatus(ctx, n.ID, user, status)
		if err != nil {
			return out, err
		}
		out = append(out, notif)
	}
	if len(out) > 0 {
		notify_service.NotificationCountChange(ctx, user.ID)
	}
	return out, nil
}

// MarkAllRead transitions every unread notification for the user to read.
func MarkAllRead(ctx context.Context, user *user_model.User) error {
	if err := activities_model.UpdateNotificationStatuses(ctx, user, activities_model.NotificationStatusUnread, activities_model.NotificationStatusRead); err != nil {
		return err
	}
	notify_service.NotificationCountChange(ctx, user.ID)
	return nil
}
