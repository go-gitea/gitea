// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	goctx "context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
)

const (
	tplNotification              templates.TplName = "user/notification/notification"
	tplNotificationDiv           templates.TplName = "user/notification/notification_div"
	tplNotificationSubscriptions templates.TplName = "user/notification/notification_subscriptions"
)

// GetNotificationCount is the middleware that sets the notification count in the context
func GetNotificationCount(ctx *context.Context) {
	if strings.HasPrefix(ctx.Req.URL.Path, "/api") {
		return
	}

	if !ctx.IsSigned {
		return
	}

	ctx.Data["NotificationUnreadCount"] = func() int64 {
		count, err := db.Count[activities_model.Notification](ctx, activities_model.FindNotificationOptions{
			UserID: ctx.Doer.ID,
			Status: []activities_model.NotificationStatus{activities_model.NotificationStatusUnread},
		})
		if err != nil {
			if err != goctx.Canceled {
				log.Error("Unable to GetNotificationCount for user:%-v: %v", ctx.Doer, err)
			}
			return -1
		}

		return count
	}
}

// Notifications is the notifications page
func Notifications(ctx *context.Context) {
	getNotifications(ctx)
	if ctx.Written() {
		return
	}
	if ctx.FormBool("div-only") {
		ctx.Data["SequenceNumber"] = ctx.FormString("sequence-number")
		ctx.HTML(http.StatusOK, tplNotificationDiv)
		return
	}
	ctx.HTML(http.StatusOK, tplNotification)
}

func getNotifications(ctx *context.Context) {
	var (
		keyword = ctx.FormTrim("q")
		status  activities_model.NotificationStatus
		page    = ctx.FormInt("page")
		perPage = ctx.FormInt("perPage")
	)
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	switch keyword {
	case "read":
		status = activities_model.NotificationStatusRead
	default:
		status = activities_model.NotificationStatusUnread
	}

	total, err := db.Count[activities_model.Notification](ctx, activities_model.FindNotificationOptions{
		UserID: ctx.Doer.ID,
		Status: []activities_model.NotificationStatus{status},
	})
	if err != nil {
		ctx.ServerError("ErrGetNotificationCount", err)
		return
	}

	// redirect to last page if request page is more than total pages
	pager := context.NewPagination(int(total), perPage, page, 5)
	if pager.Paginater.Current() < page {
		ctx.Redirect(fmt.Sprintf("%s/notifications?q=%s&page=%d", setting.AppSubURL, url.QueryEscape(ctx.FormString("q")), pager.Paginater.Current()))
		return
	}

	statuses := []activities_model.NotificationStatus{status, activities_model.NotificationStatusPinned}
	nls, err := db.Find[activities_model.Notification](ctx, activities_model.FindNotificationOptions{
		ListOptions: db.ListOptions{
			PageSize: perPage,
			Page:     page,
		},
		UserID: ctx.Doer.ID,
		Status: statuses,
	})
	if err != nil {
		ctx.ServerError("db.Find[activities_model.Notification]", err)
		return
	}

	notifications := activities_model.NotificationList(nls)

	failCount := 0

	repos, failures, err := notifications.LoadRepos(ctx)
	if err != nil {
		ctx.ServerError("LoadRepos", err)
		return
	}
	notifications = notifications.Without(failures)
	if err := repos.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}
	failCount += len(failures)

	failures, err = notifications.LoadIssues(ctx)
	if err != nil {
		ctx.ServerError("LoadIssues", err)
		return
	}

	if err = notifications.LoadIssuePullRequests(ctx); err != nil {
		ctx.ServerError("LoadIssuePullRequests", err)
		return
	}

	notifications = notifications.Without(failures)
	failCount += len(failures)

	failures, err = notifications.LoadComments(ctx)
	if err != nil {
		ctx.ServerError("LoadComments", err)
		return
	}
	notifications = notifications.Without(failures)
	failCount += len(failures)

	if failCount > 0 {
		ctx.Flash.Error(fmt.Sprintf("ERROR: %d notifications were removed due to missing parts - check the logs", failCount))
	}

	ctx.Data["Title"] = ctx.Tr("notifications")
	ctx.Data["Keyword"] = keyword
	ctx.Data["Status"] = status
	ctx.Data["Notifications"] = notifications

	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager
}

// NotificationStatusPost is a route for changing the status of a notification
func NotificationStatusPost(ctx *context.Context) {
	var (
		notificationID = ctx.FormInt64("notification_id")
		statusStr      = ctx.FormString("status")
		status         activities_model.NotificationStatus
	)

	switch statusStr {
	case "read":
		status = activities_model.NotificationStatusRead
	case "unread":
		status = activities_model.NotificationStatusUnread
	case "pinned":
		status = activities_model.NotificationStatusPinned
	default:
		ctx.ServerError("InvalidNotificationStatus", errors.New("Invalid notification status"))
		return
	}

	if _, err := activities_model.SetNotificationStatus(ctx, notificationID, ctx.Doer, status); err != nil {
		ctx.ServerError("SetNotificationStatus", err)
		return
	}

	if !ctx.FormBool("noredirect") {
		url := fmt.Sprintf("%s/notifications?page=%s", setting.AppSubURL, url.QueryEscape(ctx.FormString("page")))
		ctx.Redirect(url, http.StatusSeeOther)
	}

	getNotifications(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Link"] = setting.AppSubURL + "/notifications"
	ctx.Data["SequenceNumber"] = ctx.Req.PostFormValue("sequence-number")

	ctx.HTML(http.StatusOK, tplNotificationDiv)
}

// NotificationPurgePost is a route for 'purging' the list of notifications - marking all unread as read
func NotificationPurgePost(ctx *context.Context) {
	err := activities_model.UpdateNotificationStatuses(ctx, ctx.Doer, activities_model.NotificationStatusUnread, activities_model.NotificationStatusRead)
	if err != nil {
		ctx.ServerError("UpdateNotificationStatuses", err)
		return
	}

	ctx.Redirect(setting.AppSubURL+"/notifications", http.StatusSeeOther)
}

// NotificationSubscriptions returns the list of subscribed issues
func NotificationSubscriptions(ctx *context.Context) {
	page := ctx.FormInt("page")
	if page < 1 {
		page = 1
	}

	sortType := ctx.FormString("sort")
	ctx.Data["SortType"] = sortType

	state := ctx.FormString("state")
	if !util.SliceContainsString([]string{"all", "open", "closed"}, state, true) {
		state = "all"
	}

	ctx.Data["State"] = state
	// default state filter is "all"
	showClosed := optional.None[bool]()
	switch state {
	case "closed":
		showClosed = optional.Some(true)
	case "open":
		showClosed = optional.Some(false)
	}

	issueType := ctx.FormString("issueType")
	// default issue type is no filter
	issueTypeBool := optional.None[bool]()
	switch issueType {
	case "issues":
		issueTypeBool = optional.Some(false)
	case "pulls":
		issueTypeBool = optional.Some(true)
	}
	ctx.Data["IssueType"] = issueType

	var labelIDs []int64
	selectedLabels := ctx.FormString("labels")
	ctx.Data["Labels"] = selectedLabels
	if len(selectedLabels) > 0 && selectedLabels != "0" {
		var err error
		labelIDs, err = base.StringsToInt64s(strings.Split(selectedLabels, ","))
		if err != nil {
			ctx.Flash.Error(ctx.Tr("invalid_data", selectedLabels), true)
		}
	}

	count, err := issues_model.CountIssues(ctx, &issues_model.IssuesOptions{
		SubscriberID: ctx.Doer.ID,
		IsClosed:     showClosed,
		IsPull:       issueTypeBool,
		LabelIDs:     labelIDs,
	})
	if err != nil {
		ctx.ServerError("CountIssues", err)
		return
	}
	issues, err := issues_model.Issues(ctx, &issues_model.IssuesOptions{
		Paginator: &db.ListOptions{
			PageSize: setting.UI.IssuePagingNum,
			Page:     page,
		},
		SubscriberID: ctx.Doer.ID,
		SortType:     sortType,
		IsClosed:     showClosed,
		IsPull:       issueTypeBool,
		LabelIDs:     labelIDs,
	})
	if err != nil {
		ctx.ServerError("Issues", err)
		return
	}

	commitStatuses, lastStatus, err := pull_service.GetIssuesAllCommitStatus(ctx, issues)
	if err != nil {
		ctx.ServerError("GetIssuesAllCommitStatus", err)
		return
	}
	if !ctx.Repo.CanRead(unit.TypeActions) {
		for key := range commitStatuses {
			git_model.CommitStatusesHideActionsURL(ctx, commitStatuses[key])
		}
	}
	ctx.Data["CommitLastStatus"] = lastStatus
	ctx.Data["CommitStatuses"] = commitStatuses
	ctx.Data["Issues"] = issues

	ctx.Data["IssueRefEndNames"], ctx.Data["IssueRefURLs"] = issue_service.GetRefEndNamesAndURLs(issues, "")

	commitStatus, err := pull_service.GetIssuesLastCommitStatus(ctx, issues)
	if err != nil {
		ctx.ServerError("GetIssuesLastCommitStatus", err)
		return
	}
	ctx.Data["CommitStatus"] = commitStatus

	approvalCounts, err := issues.GetApprovalCounts(ctx)
	if err != nil {
		ctx.ServerError("ApprovalCounts", err)
		return
	}
	ctx.Data["ApprovalCounts"] = func(issueID int64, typ string) int64 {
		counts, ok := approvalCounts[issueID]
		if !ok || len(counts) == 0 {
			return 0
		}
		reviewTyp := issues_model.ReviewTypeApprove
		if typ == "reject" {
			reviewTyp = issues_model.ReviewTypeReject
		} else if typ == "waiting" {
			reviewTyp = issues_model.ReviewTypeRequest
		}
		for _, count := range counts {
			if count.Type == reviewTyp {
				return count.Count
			}
		}
		return 0
	}

	ctx.Data["Status"] = 1
	ctx.Data["Title"] = ctx.Tr("notification.subscriptions")

	// redirect to last page if request page is more than total pages
	pager := context.NewPagination(int(count), setting.UI.IssuePagingNum, page, 5)
	if pager.Paginater.Current() < page {
		ctx.Redirect(fmt.Sprintf("/notifications/subscriptions?page=%d", pager.Paginater.Current()))
		return
	}
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplNotificationSubscriptions)
}

// NotificationWatching returns the list of watching repos
func NotificationWatching(ctx *context.Context) {
	page := ctx.FormInt("page")
	if page < 1 {
		page = 1
	}

	keyword := ctx.FormTrim("q")
	ctx.Data["Keyword"] = keyword

	var orderBy db.SearchOrderBy
	ctx.Data["SortType"] = ctx.FormString("sort")
	switch ctx.FormString("sort") {
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
		ctx.Data["SortType"] = "recentupdate"
		orderBy = db.SearchOrderByRecentUpdated
	}

	archived := ctx.FormOptionalBool("archived")
	ctx.Data["IsArchived"] = archived

	fork := ctx.FormOptionalBool("fork")
	ctx.Data["IsFork"] = fork

	mirror := ctx.FormOptionalBool("mirror")
	ctx.Data["IsMirror"] = mirror

	template := ctx.FormOptionalBool("template")
	ctx.Data["IsTemplate"] = template

	private := ctx.FormOptionalBool("private")
	ctx.Data["IsPrivate"] = private

	repos, count, err := repo_model.SearchRepository(ctx, &repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		},
		Actor:              ctx.Doer,
		Keyword:            keyword,
		OrderBy:            orderBy,
		Private:            ctx.IsSigned,
		WatchedByID:        ctx.Doer.ID,
		Collaborate:        optional.Some(false),
		TopicOnly:          ctx.FormBool("topic"),
		IncludeDescription: setting.UI.SearchRepoDescription,
		Archived:           archived,
		Fork:               fork,
		Mirror:             mirror,
		Template:           template,
		IsPrivate:          private,
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}
	total := int(count)
	ctx.Data["Total"] = total
	ctx.Data["Repos"] = repos

	// redirect to last page if request page is more than total pages
	pager := context.NewPagination(total, setting.UI.User.RepoPagingNum, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager

	ctx.Data["Status"] = 2
	ctx.Data["Title"] = ctx.Tr("notification.watching")

	ctx.HTML(http.StatusOK, tplNotificationSubscriptions)
}

// NewAvailable returns the notification counts
func NewAvailable(ctx *context.Context) {
	total, err := db.Count[activities_model.Notification](ctx, activities_model.FindNotificationOptions{
		UserID: ctx.Doer.ID,
		Status: []activities_model.NotificationStatus{activities_model.NotificationStatusUnread},
	})
	if err != nil {
		log.Error("db.Count[activities_model.Notification]", err)
		ctx.JSON(http.StatusOK, structs.NotificationCount{New: 0})
		return
	}

	ctx.JSON(http.StatusOK, structs.NotificationCount{New: total})
}
