package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"net/http"
	"strconv"
)

// AddTimeManual tracks time manually
func AddTimeManual(c *context.Context) {
	hours, err := strconv.ParseInt(c.Req.PostForm.Get("hours"), 10, 64)
	if err != nil {
		hours = 0
	}
	minutes, err := strconv.ParseInt(c.Req.PostForm.Get("minutes"), 10, 64)
	if err != nil {
		minutes = 0
	}
	seconds, err := strconv.ParseInt(c.Req.PostForm.Get("seconds"), 10, 64)
	if err != nil {
		seconds = 0
	}

	totalInSeconds := seconds + minutes*60 + hours*60*60

	if totalInSeconds <= 0 {
		c.Handle(http.StatusInternalServerError, "sum of seconds <= 0", nil)
		return
	}

	issueIndex := c.ParamsInt64("index")
	issue, err := models.GetIssueByIndex(c.Repo.Repository.ID, issueIndex)
	if err != nil {
		c.Handle(http.StatusInternalServerError, "GetIssueByIndex", err)
		return
	}

	if err := models.AddTime(c.User.ID, issue.ID, totalInSeconds); err != nil {
		c.Handle(http.StatusInternalServerError, "AddTime", err)
		return
	}

	url := issue.HTMLURL()
	c.Redirect(url, http.StatusSeeOther)
}
