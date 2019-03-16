// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"sync"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/util"
)

var (
	_ base.Uploader = &GiteaLocalUploader{}
)

// GiteaLocalUploader implements an Uploader to gitea sites
type GiteaLocalUploader struct {
	doer       *models.User
	repoOwner  string
	repoName   string
	repoID     int64
	labels     sync.Map
	milestones sync.Map
	issues     sync.Map
}

// NewGiteaLocalUploader creates an gitea Uploader via gitea API v1
func NewGiteaLocalUploader(doer *models.User, repoOwner, repoName string) *GiteaLocalUploader {
	return &GiteaLocalUploader{
		doer:      doer,
		repoOwner: repoOwner,
		repoName:  repoName,
	}
}

// CreateRepo creates a repository
func (g *GiteaLocalUploader) CreateRepo(repo *base.Repository) error {
	owner, err := models.GetUserByName(g.repoOwner)
	if err != nil {
		return err
	}

	r, err := models.MigrateRepository(g.doer, owner, models.MigrateRepoOptions{
		Name:        g.repoName,
		Description: repo.Description,
		IsMirror:    repo.IsMirror,
		RemoteAddr:  repo.CloneURL,
		IsPrivate:   repo.IsPrivate,
	})
	if err != nil {
		return err
	}
	g.repoID = r.ID
	return nil
}

// CreateMilestone creates milestone
func (g *GiteaLocalUploader) CreateMilestone(milestone *base.Milestone) error {
	var deadline util.TimeStamp
	if milestone.Deadline != nil {
		deadline = util.TimeStamp(milestone.Deadline.Unix())
	}
	var ms = models.Milestone{
		RepoID:   g.repoID,
		Name:     milestone.Title,
		Content:  milestone.Description,
		IsClosed: milestone.State == "close",
		//NumIssues       int
		//NumClosedIssues int
		//Completeness    int  // Percentage(1-100).
		DeadlineUnix: deadline,
		//ClosedDateUnix util.TimeStamp
	}
	err := models.NewMilestone(&ms)

	if err != nil {
		return err
	}
	g.milestones.Store(ms.Name, ms.ID)
	return nil
}

// CreateLabel creates label
func (g *GiteaLocalUploader) CreateLabel(label *base.Label) error {
	var lb = models.Label{
		RepoID:      g.repoID,
		Name:        label.Name,
		Description: label.Description,
		Color:       fmt.Sprintf("#%s", label.Color),
	}
	err := models.NewLabel(&lb)
	if err != nil {
		return err
	}
	g.labels.Store(lb.Name, lb.ID)
	return nil
}

// CreateIssue creates issue
func (g *GiteaLocalUploader) CreateIssue(issue *base.Issue) error {
	var labelIDs []int64
	for _, label := range issue.Labels {
		id, ok := g.labels.Load(label.Name)
		if !ok {
			return fmt.Errorf("Label %s missing when create issue", label.Name)
		}
		labelIDs = append(labelIDs, id.(int64))
	}

	var milestoneID int64
	if issue.Milestone != "" {
		milestone, ok := g.milestones.Load(issue.Milestone)
		if !ok {
			return fmt.Errorf("Milestone %s missing when create issue", issue.Milestone)
		}
		milestoneID = milestone.(int64)
	}

	var is = models.Issue{
		RepoID:      g.repoID,
		Index:       issue.Number,
		PosterID:    g.doer.ID,
		Title:       issue.Title,
		Content:     fmt.Sprintf("Author: @%s Posted at: %s\n\n\n%s", issue.PosterName, issue.Created.Format("02.01.2006 15:04"), issue.Content),
		IsClosed:    issue.State == "closed",
		MilestoneID: milestoneID,
	}

	err := models.InsertIssue(&is, labelIDs)
	if err != nil {
		return err
	}
	g.issues.Store(issue.Number, is.ID)
	// TODO: add reactions
	return err
}

// CreateComment creates comment
func (g *GiteaLocalUploader) CreateComment(issueNumber int64, comment *base.Comment) error {
	var issueID int64
	if issueIDStr, ok := g.issues.Load(issueNumber); !ok {
		issue, err := models.GetIssueByIndex(g.repoID, issueNumber)
		if err != nil {
			return err
		}
		issueID = issue.ID
		g.issues.Store(issueNumber, issueID)
	} else {
		issueID = issueIDStr.(int64)
	}

	var cm = models.Comment{
		IssueID:  issueID,
		Type:     models.CommentTypeComment,
		PosterID: g.doer.ID,
		Content: fmt.Sprintf("Author: @%s Posted at: %s\n\n\n%s",
			comment.PosterName,
			comment.Created.Format("02.01.2006 15:04"),
			comment.Content),
	}
	err := models.InsertComment(&cm)
	// TODO: Reactions
	return err
}

// CreatePullRequest creates pull request
func (g *GiteaLocalUploader) CreatePullRequest(pr *base.PullRequest) error {
	var labelIDs []int64
	for _, label := range pr.Labels {
		id, ok := g.labels.Load(label.Name)
		if !ok {
			return fmt.Errorf("Label %s missing when create issue", label.Name)
		}
		labelIDs = append(labelIDs, id.(int64))
	}

	var milestoneID int64
	if pr.Milestone != "" {
		milestone, ok := g.milestones.Load(pr.Milestone)
		if !ok {
			return fmt.Errorf("Milestone %s missing when create issue", pr.Milestone)
		}
		milestoneID = milestone.(int64)
	}

	var head string
	if pr.Head.OwnerName != "" {
		head = fmt.Sprintf("%s/%s:", pr.Head.OwnerName, pr.Head.RepoName)
	}
	head = head + pr.Head.Ref

	// TODO: creates special branches
	var pullRequest = models.PullRequest{
		HeadRepoID:   g.repoID,
		HeadBranch:   head,
		HeadUserName: g.repoOwner,
		BaseRepoID:   g.repoID,
		BaseBranch:   pr.Base.Ref,
		Index:        pr.Number,

		Issue: &models.Issue{
			RepoID:      g.repoID,
			Title:       pr.Title,
			Content:     fmt.Sprintf("Author: @%s Posted at: %s\n\n\n%s", pr.PosterName, pr.Created.Format("02.01.2006 15:04"), pr.Content),
			MilestoneID: milestoneID,
			IsPull:      true,
			IsClosed:    pr.State == "closed",
		},
	}

	// TODO: assignees

	err := models.InsertPullRequest(&pullRequest, labelIDs)
	return err
}
