// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	pull_service "code.gitea.io/gitea/services/pull"
)

const (
	tplNotification              base.TplName = "user/notification/notification"
	tplNotificationDiv           base.TplName = "user/notification/notification_div"
	tplNotificationSubscriptions base.TplName = "user/notification/notification_subscriptions"
)

// GetNotificationCount is the middleware that sets the notification count in the context
func GetNotificationCount(c *context.Context) {
	if strings.HasPrefix(c.Req.URL.Path, "/api") {
		return
	}

	if !c.IsSigned {
		return
	}

	c.Data["NotificationUnreadCount"] = func() int64 {
		count, err := models.GetNotificationCount(c.User, models.NotificationStatusUnread)
		if err != nil {
			c.ServerError("GetNotificationCount", err)
			return -1
		}

		return count
	}
}

// Notifications is the notifications page
func Notifications(c *context.Context) {
	getNotifications(c)
	if c.Written() {
		return
	}
	if c.FormBool("div-only") {
		c.Data["SequenceNumber"] = c.FormString("sequence-number")
		c.HTML(http.StatusOK, tplNotificationDiv)
		return
	}
	c.HTML(http.StatusOK, tplNotification)
}

func getNotifications(c *context.Context) {
	var (
		keyword = c.FormTrim("q")
		status  models.NotificationStatus
		page    = c.FormInt("page")
		perPage = c.FormInt("perPage")
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

	total, err := models.GetNotificationCount(c.User, status)
	if err != nil {
		c.ServerError("ErrGetNotificationCount", err)
		return
	}

	// redirect to last page if request page is more than total pages
	pager := context.NewPagination(int(total), perPage, page, 5)
	if pager.Paginater.Current() < page {
		c.Redirect(fmt.Sprintf("%s/notifications?q=%s&page=%d", setting.AppSubURL, url.QueryEscape(c.FormString("q")), pager.Paginater.Current()))
		return
	}

	statuses := []models.NotificationStatus{status, models.NotificationStatusPinned}
	notifications, err := models.NotificationsForUser(c.User, statuses, page, perPage)
	if err != nil {
		c.ServerError("ErrNotificationsForUser", err)
		return
	}

	failCount := 0

	repos, failures, err := notifications.LoadRepos()
	if err != nil {
		c.ServerError("LoadRepos", err)
		return
	}
	notifications = notifications.Without(failures)
	if err := repos.LoadAttributes(); err != nil {
		c.ServerError("LoadAttributes", err)
		return
	}
	failCount += len(failures)

	failures, err = notifications.LoadIssues()
	if err != nil {
		c.ServerError("LoadIssues", err)
		return
	}
	notifications = notifications.Without(failures)
	failCount += len(failures)

	failures, err = notifications.LoadComments()
	if err != nil {
		c.ServerError("LoadComments", err)
		return
	}
	notifications = notifications.Without(failures)
	failCount += len(failures)

	if failCount > 0 {
		c.Flash.Error(fmt.Sprintf("ERROR: %d notifications were removed due to missing parts - check the logs", failCount))
	}

	c.Data["Title"] = c.Tr("notifications")
	c.Data["Keyword"] = keyword
	c.Data["Status"] = status
	c.Data["Notifications"] = notifications

	pager.SetDefaultParams(c)
	c.Data["Page"] = pager
}

// NotificationStatusPost is a route for changing the status of a notification
func NotificationStatusPost(c *context.Context) {
	var (
		notificationID = c.FormInt64("notification_id")
		statusStr      = c.FormString("status")
		status         models.NotificationStatus
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

	if _, err := models.SetNotificationStatus(notificationID, c.User, status); err != nil {
		c.ServerError("SetNotificationStatus", err)
		return
	}

	if !c.FormBool("noredirect") {
		url := fmt.Sprintf("%s/notifications?page=%s", setting.AppSubURL, c.FormString("page"))
		c.Redirect(url, http.StatusSeeOther)
	}

	getNotifications(c)
	if c.Written() {
		return
	}
	c.Data["Link"] = setting.AppURL + "notifications"
	c.Data["SequenceNumber"] = c.Req.PostFormValue("sequence-number")

	c.HTML(http.StatusOK, tplNotificationDiv)
}

// NotificationPurgePost is a route for 'purging' the list of notifications - marking all unread as read
func NotificationPurgePost(c *context.Context) {
	err := models.UpdateNotificationStatuses(c.User, models.NotificationStatusUnread, models.NotificationStatusRead)
	if err != nil {
		c.ServerError("ErrUpdateNotificationStatuses", err)
		return
	}

	url := fmt.Sprintf("%s/notifications", setting.AppSubURL)
	c.Redirect(url, http.StatusSeeOther)
}

// NotificationSubscriptions returns the list of subscribed issues
func NotificationSubscriptions(c *context.Context) {
	var page = c.FormInt("page")
	if page < 1 {
		page = 1
	}

	viewType := c.FormString("type")
	sortType := c.FormString("sort")
	types := []string{"all", "assigned", "created_by", "mentioned"}
	if !util.IsStringInSlice(viewType, types, true) {
		viewType = "all"
	}
	c.Data["SortType"] = sortType
	c.Data["ViewType"] = viewType

	state := c.FormString("state")
	states := []string{"all", "open", "closed"}
	if !util.IsStringInSlice(state, states, true) {
		state = "all"
	}
	c.Data["State"] = state
	var showClosed util.OptionalBool
	switch state {
	case "all":
		showClosed = util.OptionalBoolNone
	case "closed":
		showClosed = util.OptionalBoolTrue
	case "open":
		showClosed = util.OptionalBoolFalse
	}

	var (
		assigneeID  int64
		posterID    int64
		mentionedID int64
	)

	switch viewType {
	case "created_by":
		posterID = c.User.ID
	case "mentioned":
		mentionedID = c.User.ID
	case "assigned":
		assigneeID = c.User.ID
	}

	var issueTypeBool util.OptionalBool
	issueType := c.FormString("issueType")
	switch issueType {
	case "issues":
		issueTypeBool = util.OptionalBoolFalse
	case "pulls":
		issueTypeBool = util.OptionalBoolTrue
	default:
		issueTypeBool = util.OptionalBoolNone
	}
	c.Data["IssueType"] = issueType

	count, err := models.CountIssues(&models.IssuesOptions{
		SubscriberID: c.User.ID,
		AssigneeID:   assigneeID,
		MentionedID:  mentionedID,
		PosterID:     posterID,
		IsClosed:     showClosed,
		IsPull:       issueTypeBool,
	})
	if err != nil {
		c.ServerError("CountIssues", err)
		return
	}
	issues, err := models.Issues(&models.IssuesOptions{
		ListOptions: db.ListOptions{
			PageSize: setting.UI.IssuePagingNum,
			Page:     page,
		},
		SubscriberID: c.User.ID,
		AssigneeID:   assigneeID,
		MentionedID:  mentionedID,
		PosterID:     posterID,
		SortType:     sortType,
		IsClosed:     showClosed,
		IsPull:       issueTypeBool,
	})
	if err != nil {
		c.ServerError("Issues", err)
		return
	}
	c.Data["Issues"] = issues

	commitStatus, err := pull_service.GetIssuesLastCommitStatus(issues)
	if err != nil {
		c.ServerError("GetIssuesLastCommitStatus", err)
		return
	}
	c.Data["CommitStatus"] = commitStatus

	var issueList = models.IssueList(issues)
	approvalCounts, err := issueList.GetApprovalCounts()
	if err != nil {
		c.ServerError("ApprovalCounts", err)
		return
	}
	c.Data["ApprovalCounts"] = func(issueID int64, typ string) int64 {
		counts, ok := approvalCounts[issueID]
		if !ok || len(counts) == 0 {
			return 0
		}
		reviewTyp := models.ReviewTypeApprove
		if typ == "reject" {
			reviewTyp = models.ReviewTypeReject
		} else if typ == "waiting" {
			reviewTyp = models.ReviewTypeRequest
		}
		for _, count := range counts {
			if count.Type == reviewTyp {
				return count.Count
			}
		}
		return 0
	}

	c.Data["Status"] = 1
	c.Data["Title"] = c.Tr("notification.subscriptions")

	// redirect to last page if request page is more than total pages
	pager := context.NewPagination(int(count), setting.UI.IssuePagingNum, page, 5)
	if pager.Paginater.Current() < page {
		c.Redirect(fmt.Sprintf("/notifications/subscriptions?page=%d", pager.Paginater.Current()))
		return
	}
	//	pager.SetDefaultParams(c)
	pager.AddParam(c, "type", "ViewType")
	pager.AddParam(c, "sort", "SortType")
	pager.AddParam(c, "state", "State")
	c.Data["Page"] = pager

	c.HTML(http.StatusOK, tplNotificationSubscriptions)
}

// NotificationWatching returns the list of watching repos
func NotificationWatching(c *context.Context) {
	page := c.FormInt("page")
	if page < 1 {
		page = 1
	}

	var orderBy models.SearchOrderBy
	c.Data["SortType"] = c.FormString("sort")
	switch c.FormString("sort") {
	case "newest":
		orderBy = models.SearchOrderByNewest
	case "oldest":
		orderBy = models.SearchOrderByOldest
	case "recentupdate":
		orderBy = models.SearchOrderByRecentUpdated
	case "leastupdate":
		orderBy = models.SearchOrderByLeastUpdated
	case "reversealphabetically":
		orderBy = models.SearchOrderByAlphabeticallyReverse
	case "alphabetically":
		orderBy = models.SearchOrderByAlphabetically
	case "moststars":
		orderBy = models.SearchOrderByStarsReverse
	case "feweststars":
		orderBy = models.SearchOrderByStars
	case "mostforks":
		orderBy = models.SearchOrderByForksReverse
	case "fewestforks":
		orderBy = models.SearchOrderByForks
	default:
		c.Data["SortType"] = "recentupdate"
		orderBy = models.SearchOrderByRecentUpdated
	}

	repos, count, err := models.SearchRepository(&models.SearchRepoOptions{
		ListOptions: db.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		},
		Actor:              c.User,
		Keyword:            c.FormTrim("q"),
		OrderBy:            orderBy,
		Private:            c.IsSigned,
		WatchedByID:        c.User.ID,
		Collaborate:        util.OptionalBoolFalse,
		TopicOnly:          c.FormBool("topic"),
		IncludeDescription: setting.UI.SearchRepoDescription,
	})
	if err != nil {
		c.ServerError("ErrSearchRepository", err)
		return
	}
	total := int(count)
	c.Data["Total"] = total
	c.Data["Repos"] = repos

	// redirect to last page if request page is more than total pages
	pager := context.NewPagination(total, setting.UI.User.RepoPagingNum, page, 5)
	pager.SetDefaultParams(c)
	c.Data["Page"] = pager

	c.Data["Status"] = 2
	c.Data["Title"] = c.Tr("notification.watching")

	c.HTML(http.StatusOK, tplNotificationSubscriptions)
}
