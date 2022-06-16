// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	goctx "context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	issue_service "code.gitea.io/gitea/services/issue"
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
		count, err := models.GetNotificationCount(c, c.Doer, models.NotificationStatusUnread)
		if err != nil {
			if err != goctx.Canceled {
				log.Error("Unable to GetNotificationCount for user:%-v: %v", c.Doer, err)
			}
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

	total, err := models.GetNotificationCount(c, c.Doer, status)
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
	notifications, err := models.NotificationsForUser(c, c.Doer, statuses, page, perPage)
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

	if _, err := models.SetNotificationStatus(notificationID, c.Doer, status); err != nil {
		c.ServerError("SetNotificationStatus", err)
		return
	}

	if !c.FormBool("noredirect") {
		url := fmt.Sprintf("%s/notifications?page=%s", setting.AppSubURL, url.QueryEscape(c.FormString("page")))
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
	err := models.UpdateNotificationStatuses(c.Doer, models.NotificationStatusUnread, models.NotificationStatusRead)
	if err != nil {
		c.ServerError("ErrUpdateNotificationStatuses", err)
		return
	}

	c.Redirect(setting.AppSubURL+"/notifications", http.StatusSeeOther)
}

// NotificationSubscriptions returns the list of subscribed issues
func NotificationSubscriptions(c *context.Context) {
	page := c.FormInt("page")
	if page < 1 {
		page = 1
	}

	sortType := c.FormString("sort")
	c.Data["SortType"] = sortType

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

	var labelIDs []int64
	selectedLabels := c.FormString("labels")
	c.Data["Labels"] = selectedLabels
	if len(selectedLabels) > 0 && selectedLabels != "0" {
		var err error
		labelIDs, err = base.StringsToInt64s(strings.Split(selectedLabels, ","))
		if err != nil {
			c.ServerError("StringsToInt64s", err)
			return
		}
	}

	count, err := models.CountIssues(&models.IssuesOptions{
		SubscriberID: c.Doer.ID,
		IsClosed:     showClosed,
		IsPull:       issueTypeBool,
		LabelIDs:     labelIDs,
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
		SubscriberID: c.Doer.ID,
		SortType:     sortType,
		IsClosed:     showClosed,
		IsPull:       issueTypeBool,
		LabelIDs:     labelIDs,
	})
	if err != nil {
		c.ServerError("Issues", err)
		return
	}
	c.Data["Issues"] = issues

	c.Data["IssueRefEndNames"], c.Data["IssueRefURLs"] = issue_service.GetRefEndNamesAndURLs(issues, "")

	commitStatus, err := pull_service.GetIssuesLastCommitStatus(c, issues)
	if err != nil {
		c.ServerError("GetIssuesLastCommitStatus", err)
		return
	}
	c.Data["CommitStatus"] = commitStatus

	issueList := models.IssueList(issues)
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

	var orderBy db.SearchOrderBy
	c.Data["SortType"] = c.FormString("sort")
	switch c.FormString("sort") {
	case "newest":
		orderBy = db.SearchOrderByNewest
	case "oldest":
		orderBy = db.SearchOrderByOldest
	case "recentupdate":
		orderBy = db.SearchOrderByRecentUpdated
	case "leastupdate":
		orderBy = db.SearchOrderByLeastUpdated
	case "reversealphabetically":
		orderBy = db.SearchOrderByAlphabeticallyReverse
	case "alphabetically":
		orderBy = db.SearchOrderByAlphabetically
	case "moststars":
		orderBy = db.SearchOrderByStarsReverse
	case "feweststars":
		orderBy = db.SearchOrderByStars
	case "mostforks":
		orderBy = db.SearchOrderByForksReverse
	case "fewestforks":
		orderBy = db.SearchOrderByForks
	default:
		c.Data["SortType"] = "recentupdate"
		orderBy = db.SearchOrderByRecentUpdated
	}

	repos, count, err := models.SearchRepository(&models.SearchRepoOptions{
		ListOptions: db.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		},
		Actor:              c.Doer,
		Keyword:            c.FormTrim("q"),
		OrderBy:            orderBy,
		Private:            c.IsSigned,
		WatchedByID:        c.Doer.ID,
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

// NewAvailable returns the notification counts
func NewAvailable(ctx *context.Context) {
	ctx.JSON(http.StatusOK, structs.NotificationCount{New: models.CountUnread(ctx, ctx.Doer.ID)})
}
