// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplNotification base.TplName = "user/notification/notification"
)

// GetNotificationCount is the middleware that sets the notification count in the context
func GetNotificationCount(c *context.Context) {
	if strings.HasPrefix(c.Req.URL.Path, "/api") {
		return
	}

	if !c.IsSigned {
		return
	}

	count, err := models.GetNotificationCount(c.User, models.NotificationStatusUnread)
	if err != nil {
		c.ServerError("GetNotificationCount", err)
		return
	}

	c.Data["NotificationUnreadCount"] = count
}

// Notifications is the notifications page
func Notifications(c *context.Context) {
	var (
		keyword = strings.Trim(c.Query("q"), " ")
		status  models.NotificationStatus
		page    = c.QueryInt("page")
		perPage = c.QueryInt("perPage")
	)
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	switch keyword {
	case "read":
		status = models.NotificationStatusRead
	default:
		status = models.NotificationStatusUnread
	}

	statuses := []models.NotificationStatus{status, models.NotificationStatusPinned}
	notifications, err := models.NotificationsForUser(c.User, statuses, page, perPage)
	if err != nil {
		c.ServerError("ErrNotificationsForUser", err)
		return
	}

	repos, err := notifications.LoadRepos()
	if err != nil {
		c.ServerError("LoadRepos", err)
		return
	}
	if err := repos.LoadAttributes(); err != nil {
		c.ServerError("LoadAttributes", err)
		return
	}

	if err := notifications.LoadIssues(); err != nil {
		c.ServerError("LoadIssues", err)
		return
	}
	if err := notifications.LoadComments(); err != nil {
		c.ServerError("LoadComments", err)
		return
	}

	total, err := models.GetNotificationCount(c.User, status)
	if err != nil {
		c.ServerError("ErrGetNotificationCount", err)
		return
	}

	title := c.Tr("notifications")
	if status == models.NotificationStatusUnread && total > 0 {
		title = fmt.Sprintf("(%d) %s", total, title)
	}
	c.Data["Title"] = title
	c.Data["Keyword"] = keyword
	c.Data["Status"] = status
	c.Data["Notifications"] = notifications

	pager := context.NewPagination(int(total), perPage, page, 5)
	pager.SetDefaultParams(c)
	c.Data["Page"] = pager

	c.HTML(200, tplNotification)
}

// NotificationStatusPost is a route for changing the status of a notification
func NotificationStatusPost(c *context.Context) {
	var (
		notificationID, _ = strconv.ParseInt(c.Req.PostFormValue("notification_id"), 10, 64)
		statusStr         = c.Req.PostFormValue("status")
		status            models.NotificationStatus
	)

	switch statusStr {
	case "read":
		status = models.NotificationStatusRead
	case "unread":
		status = models.NotificationStatusUnread
	case "pinned":
		status = models.NotificationStatusPinned
	default:
		c.ServerError("InvalidNotificationStatus", errors.New("Invalid notification status"))
		return
	}

	if err := models.SetNotificationStatus(notificationID, c.User, status); err != nil {
		c.ServerError("SetNotificationStatus", err)
		return
	}

	url := fmt.Sprintf("%s/notifications", setting.AppSubURL)
	c.Redirect(url, 303)
}

// NotificationPurgePost is a route for 'purging' the list of notifications - marking all unread as read
func NotificationPurgePost(c *context.Context) {
	err := models.UpdateNotificationStatuses(c.User, models.NotificationStatusUnread, models.NotificationStatusRead)
	if err != nil {
		c.ServerError("ErrUpdateNotificationStatuses", err)
		return
	}

	url := fmt.Sprintf("%s/notifications", setting.AppSubURL)
	c.Redirect(url, 303)
}
