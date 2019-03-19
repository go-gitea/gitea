// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	gouuid "github.com/satori/go.uuid"
)

var (
	_ base.Uploader = &GiteaLocalUploader{}
)

// GiteaLocalUploader implements an Uploader to gitea sites
type GiteaLocalUploader struct {
	doer       *models.User
	repoOwner  string
	repoName   string
	repo       *models.Repository
	labels     sync.Map
	milestones sync.Map
	issues     sync.Map
	gitRepo    *git.Repository
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
	g.repo = r
	g.gitRepo, err = git.OpenRepository(r.RepoPath())
	return err
}

// CreateMilestone creates milestone
func (g *GiteaLocalUploader) CreateMilestone(milestone *base.Milestone) error {
	var deadline util.TimeStamp
	if milestone.Deadline != nil {
		deadline = util.TimeStamp(milestone.Deadline.Unix())
	}
	if deadline == 0 {
		deadline = util.TimeStamp(time.Date(9999, 1, 1, 0, 0, 0, 0, setting.UILocation).Unix())
	}
	var ms = models.Milestone{
		RepoID:       g.repo.ID,
		Name:         milestone.Title,
		Content:      milestone.Description,
		IsClosed:     milestone.State == "close",
		DeadlineUnix: deadline,
	}
	if ms.IsClosed && milestone.Closed != nil {
		ms.ClosedDateUnix = util.TimeStamp(milestone.Closed.Unix())
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
		RepoID:      g.repo.ID,
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

// CreateRelease creates release
func (g *GiteaLocalUploader) CreateRelease(release *base.Release) error {
	var rel = models.Release{
		RepoID:       g.repo.ID,
		PublisherID:  g.doer.ID,
		TagName:      release.TagName,
		LowerTagName: strings.ToLower(release.TagName),
		Target:       release.TargetCommitish,
		Title:        release.Name,
		Sha1:         release.TargetCommitish,
		Note:         release.Body,
		IsDraft:      release.Draft,
		IsPrerelease: release.Prerelease,
		IsTag:        false,
		CreatedUnix:  util.TimeStamp(release.Created.Unix()),
	}

	// calc NumCommits
	commit, err := g.gitRepo.GetCommit(rel.TagName)
	if err != nil {
		return fmt.Errorf("GetCommit: %v", err)
	}
	rel.NumCommits, err = commit.CommitsCount()
	if err != nil {
		return fmt.Errorf("CommitsCount: %v", err)
	}

	for _, asset := range release.Assets {
		var attach = models.Attachment{
			UUID:          gouuid.NewV4().String(),
			Name:          asset.Name,
			DownloadCount: int64(*asset.DownloadCount),
			Size:          int64(*asset.Size),
			CreatedUnix:   util.TimeStamp(asset.Created.Unix()),
		}

		// download attachment
		resp, err := http.Get(asset.URL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		localPath := attach.LocalPath()
		if err = os.MkdirAll(path.Dir(localPath), os.ModePerm); err != nil {
			return fmt.Errorf("MkdirAll: %v", err)
		}

		fw, err := os.Create(localPath)
		if err != nil {
			return fmt.Errorf("Create: %v", err)
		}
		defer fw.Close()

		if _, err := io.Copy(fw, resp.Body); err != nil {
			return err
		}

		rel.Attachments = append(rel.Attachments, &attach)
	}

	return models.MigrateRelease(&rel)
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
		RepoID:      g.repo.ID,
		Repo:        g.repo,
		Index:       issue.Number,
		PosterID:    g.doer.ID,
		Title:       issue.Title,
		Content:     fmt.Sprintf("Author: @%s \n\n%s", issue.PosterName, issue.Content),
		IsClosed:    issue.State == "closed",
		MilestoneID: milestoneID,
		CreatedUnix: util.TimeStamp(issue.Created.Unix()),
	}
	if issue.Closed != nil {
		is.ClosedUnix = util.TimeStamp(issue.Closed.Unix())
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
		issue, err := models.GetIssueByIndex(g.repo.ID, issueNumber)
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
		Content: fmt.Sprintf("Author: @%s \n\n%s",
			comment.PosterName,
			comment.Content),
		CreatedUnix: util.TimeStamp(comment.Created.Unix()),
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
		HeadRepoID:   g.repo.ID,
		HeadBranch:   head,
		HeadUserName: g.repoOwner,
		BaseRepoID:   g.repo.ID,
		BaseBranch:   pr.Base.Ref,
		Index:        pr.Number,

		Issue: &models.Issue{
			RepoID:      g.repo.ID,
			Repo:        g.repo,
			Title:       pr.Title,
			Index:       pr.Number,
			PosterID:    g.doer.ID,
			Content:     fmt.Sprintf("Author: @%s \n\n%s", pr.PosterName, pr.Content),
			MilestoneID: milestoneID,
			IsPull:      true,
			IsClosed:    pr.State == "closed",
			CreatedUnix: util.TimeStamp(pr.Created.Unix()),
		},
	}

	if pullRequest.Issue.IsClosed && pr.Closed != nil {
		pullRequest.Issue.ClosedUnix = util.TimeStamp(pr.Closed.Unix())
	}

	// TODO: islocked
	// TODO: reactions
	// TODO: replace internal link github.com/xxx/xxx#<index> to #xxx/xxx#<index>
	// TODO: assignees

	err := models.InsertPullRequest(&pullRequest, labelIDs)
	return err
}
