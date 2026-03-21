// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"net/url"

	activities_model "code.gitea.io/gitea/models/activities"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToNotificationThread convert a Notification to api.NotificationThread
func ToNotificationThread(ctx context.Context, n *activities_model.Notification) *api.NotificationThread {
	result := &api.NotificationThread{
		ID:        n.ID,
		Unread:    !(n.Status == activities_model.NotificationStatusRead || n.Status == activities_model.NotificationStatusPinned),
		Pinned:    n.Status == activities_model.NotificationStatusPinned,
		UpdatedAt: n.UpdatedUnix.AsTime(),
		URL:       n.APIURL(),
	}

	// since user only get notifications when he has access to use minimal access mode
	if n.Repository != nil {
		perm, err := access_model.GetUserRepoPermission(ctx, n.Repository, n.User)
		if err != nil {
			log.Error("GetUserRepoPermission failed: %v", err)
			return result
		}
		if perm.HasAnyUnitAccessOrPublicAccess() { // if user has been revoked access to repo, do not show repo info
			result.Repository = ToRepo(ctx, n.Repository, perm)
			// This permission is not correct and we should not be reporting it
			for repository := result.Repository; repository != nil; repository = repository.Parent {
				repository.Permissions = nil
			}
		}
	}

	// handle Subject
	switch n.Source {
	case activities_model.NotificationSourceIssue:
		result.Subject = &api.NotificationSubject{Type: api.NotifySubjectIssue}
		if n.Issue != nil {
			result.Subject.Title = n.Issue.Title
			result.Subject.URL = n.Issue.APIURL(ctx)
			result.Subject.HTMLURL = n.Issue.HTMLURL(ctx)
			result.Subject.State = n.Issue.State()
			comment, err := n.Issue.GetLastComment(ctx)
			if err == nil && comment != nil {
				result.Subject.LatestCommentURL = comment.APIURL(ctx)
				result.Subject.LatestCommentHTMLURL = comment.HTMLURL(ctx)
			}
		}
	case activities_model.NotificationSourcePullRequest:
		result.Subject = &api.NotificationSubject{Type: api.NotifySubjectPull}
		if n.Issue != nil {
			result.Subject.Title = n.Issue.Title
			result.Subject.URL = n.Issue.APIURL(ctx)
			result.Subject.HTMLURL = n.Issue.HTMLURL(ctx)
			result.Subject.State = n.Issue.State()
			comment, err := n.Issue.GetLastComment(ctx)
			if err == nil && comment != nil {
				result.Subject.LatestCommentURL = comment.APIURL(ctx)
				result.Subject.LatestCommentHTMLURL = comment.HTMLURL(ctx)
			}

			if err := n.Issue.LoadPullRequest(ctx); err == nil &&
				n.Issue.PullRequest != nil &&
				n.Issue.PullRequest.HasMerged {
				result.Subject.State = "merged"
			}
		}
	case activities_model.NotificationSourceCommit:
		url := n.Repository.HTMLURL() + "/commit/" + url.PathEscape(n.CommitID)
		result.Subject = &api.NotificationSubject{
			Type:    api.NotifySubjectCommit,
			Title:   n.CommitID,
			URL:     url,
			HTMLURL: url,
		}
	case activities_model.NotificationSourceRepository:
		result.Subject = &api.NotificationSubject{
			Type:  api.NotifySubjectRepository,
			Title: n.Repository.FullName(),
			// FIXME: this is a relative URL, rather useless and inconsistent, but keeping for backwards compat
			URL:     n.Repository.Link(),
			HTMLURL: n.Repository.HTMLURL(),
		}
	}

	return result
}

// ToNotifications convert list of Notification to api.NotificationThread list
func ToNotifications(ctx context.Context, nl activities_model.NotificationList) []*api.NotificationThread {
	result := make([]*api.NotificationThread, 0, len(nl))
	for _, n := range nl {
		result = append(result, ToNotificationThread(ctx, n))
	}
	return result
}
