package user

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplNotification base.TplName = "user/notification/notification"
)

// Notifications is the notifications page
func Notifications(c *context.Context) {
	var status models.NotificationStatus
	switch c.Query("status") {
	case "read":
		status = models.NotificationStatusRead
	default:
		status = models.NotificationStatusUnread
	}

	notifications, err := models.NotificationsForUser(c.User, status)
	if err != nil {
		c.Handle(500, "ErrNotificationsForUser", err)
		return
	}

	title := "Notifications"
	if count := len(notifications); count > 0 {
		title = fmt.Sprintf("(%d) %s", count, title)
	}
	c.Data["Title"] = title
	c.Data["Status"] = status
	c.Data["Notifications"] = notifications
	c.HTML(200, tplNotification)
}
