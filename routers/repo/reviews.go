// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/git-appraise/repository"
	"github.com/google/git-appraise/review"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplReviews base.TplName = "repo/review/list"
)

// MustAllowReviews check if repository enable reviews
func MustAllowReviews(ctx *context.Context) {
	if !ctx.Repo.Repository.AllowsReviews() {
		ctx.Handle(404, "MustAllowReviews", nil)
		return
	}
}

// Review struct to Issue representation
type Review struct {
	Index       string
	Poster      *models.User
	Title       string
	Labels      []*models.Label
	Milestone   *models.Milestone
	Assignee    *models.User
	NumComments int
	Created     time.Time
	CreatedUnix int64
	Updated     time.Time
	UpdatedUnix int64
	IsClosed    bool
	IsRead      bool
	IsPull      bool
}

// Reviews render issues page
func Reviews(ctx *context.Context) {
	repo, err := repository.NewGitRepo(ctx.Repo.GitRepo.Path)
	if err != nil {
		ctx.Handle(500, "OpenGitRepository", err)
	}
	total := review.ListAll(repo)

	issues := []*Review{}

	for _, review := range total {
		timestamp, _ := strconv.Atoi(review.Request.Timestamp)
		time := time.Unix(int64(timestamp), 0)
		user, err := models.GetUserByEmail(review.Request.Requester)
		read := false
		if review.Resolved != nil && *review.Resolved {
			read = true
		}
		if err != nil {
			user = &models.User{Name: review.Request.Requester}
		}
		issue := Review{
			Index:       review.Revision,
			Title:       strings.Split(review.Request.Description, "\n\n")[0],
			Poster:      user,
			Created:     time,
			CreatedUnix: int64(timestamp),
			IsClosed:    review.Submitted,
			IsRead:      read,
			IsPull:      true,
		}
		if !review.Submitted {
			issues = append(issues, &issue)
		}
	}

	ctx.Data["Title"] = ctx.Tr("repo.reviews")
	ctx.Data["PageIsReviewList"] = true
	// ctx.Data["Page"] = 0
	ctx.Data["Issues"] = issues
	// ctx.Data["Milestones"], err = models.GetMilestonesByRepoID(repo.ID)
	// ctx.Data["Assignees"], err = repo.GetAssignees()
	ctx.Data["IssueStats"] = models.IssueStats{
		OpenCount:   int64(len(issues)),
		ClosedCount: int64(len(total) - len(issues)),
		AllCount:    int64(len(total))}
	// ctx.Data["SelectLabels"] = com.StrTo(selectLabels).MustInt64()
	ctx.Data["ViewType"] = "all"
	ctx.Data["SortType"] = ""
	// ctx.Data["MilestoneID"] = milestoneID
	// ctx.Data["AssigneeID"] = assigneeID
	ctx.Data["IsShowClosed"] = false
	ctx.HTML(200, tplIssues)
}
