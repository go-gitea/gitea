// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package notifications wraps activities_model notification-status mutations
// with the matching real-time push, so route handlers cannot forget either side.
package notifications

import (
	"context"

	activities_model "gitea.dev/models/activities"
	user_model "gitea.dev/models/user"
	notify_service "gitea.dev/services/notify"
)

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

// SetRepoReadBy marks a repository notification read when the user visits the repository.
func SetRepoReadBy(ctx context.Context, userID, repoID int64) error {
	changed, err := activities_model.SetRepoReadBy(ctx, userID, repoID)
	if err != nil {
		return err
	}
	if changed {
		notify_service.NotificationCountChange(ctx, userID)
	}
	return nil
}

// SetCommitReadBy marks a commit notification read when the user visits the commit.
func SetCommitReadBy(ctx context.Context, repoID, userID int64, commitID string) error {
	changed, err := activities_model.SetCommitReadBy(ctx, repoID, userID, commitID)
	if err != nil {
		return err
	}
	if changed {
		notify_service.NotificationCountChange(ctx, userID)
	}
	return nil
}

// SetReleaseReadBy marks a release notification read when the user visits the release.
func SetReleaseReadBy(ctx context.Context, releaseID, userID int64) error {
	changed, err := activities_model.SetReleaseReadBy(ctx, releaseID, userID)
	if err != nil {
		return err
	}
	if changed {
		notify_service.NotificationCountChange(ctx, userID)
	}
	return nil
}

func SetNotificationStatus(ctx context.Context, notificationID int64, user *user_model.User, status activities_model.NotificationStatus) (*activities_model.Notification, error) {
	notif, err := activities_model.SetNotificationStatus(ctx, notificationID, user, status)
	if err != nil {
		return notif, err
	}
	notify_service.NotificationCountChange(ctx, user.ID)
	return notif, nil
}

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

func MarkAllRead(ctx context.Context, user *user_model.User) error {
	changed, err := activities_model.UpdateNotificationStatuses(ctx, user, activities_model.NotificationStatusUnread, activities_model.NotificationStatusRead)
	if err != nil {
		return err
	}
	if changed > 0 {
		notify_service.NotificationCountChange(ctx, user.ID)
	}
	return nil
}
