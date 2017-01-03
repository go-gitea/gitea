package user

import (
	"fmt"
	"strings"

	"github.com/Unknwon/paginater"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
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

	count, err := models.GetNotificationUnreadCount(c.User)
	if err != nil {
		c.Handle(500, "GetNotificationCount", err)
		return
	}

	c.Data["NotificationUnreadCount"] = count
}

// Notifications is the notifications page
func Notifications(c *context.Context) {
	var (
		keyword = c.Query("q")
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

	notifications, err := models.NotificationsForUser(c.User, status, page, perPage)
	if err != nil {
		c.Handle(500, "ErrNotificationsForUser", err)
		return
	}

	total, err := models.GetNotificationCount(c.User, status)
	if err != nil {
		c.Handle(500, "ErrGetNotificationCount", err)
		return
	}

	title := "Notifications"
	if count := len(notifications); count > 0 {
		title = fmt.Sprintf("(%d) %s", count, title)
	}
	c.Data["Title"] = title
	c.Data["Keyword"] = keyword
	c.Data["Status"] = status
	c.Data["Notifications"] = notifications
	c.Data["Page"] = paginater.New(int(total), perPage, page, 5)
	c.HTML(200, tplNotification)
}
