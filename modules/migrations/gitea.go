// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
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
	doer        *models.User
	repoOwner   string
	repoName    string
	repo        *models.Repository
	labels      sync.Map
	milestones  sync.Map
	issues      sync.Map
	gitRepo     *git.Repository
	prHeadCache map[string]struct{}
}

// NewGiteaLocalUploader creates an gitea Uploader via gitea API v1
func NewGiteaLocalUploader(doer *models.User, repoOwner, repoName string) *GiteaLocalUploader {
	return &GiteaLocalUploader{
		doer:        doer,
		repoOwner:   repoOwner,
		repoName:    repoName,
		prHeadCache: make(map[string]struct{}),
	}
}

// MaxBatchInsertSize returns the table's max batch insert size
func (g *GiteaLocalUploader) MaxBatchInsertSize(tp string) int {
	switch tp {
	case "issue":
		return models.MaxBatchInsertSize(new(models.Issue))
	case "comment":
		return models.MaxBatchInsertSize(new(models.Comment))
	case "milestone":
		return models.MaxBatchInsertSize(new(models.Milestone))
	case "label":
		return models.MaxBatchInsertSize(new(models.Label))
	case "release":
		return models.MaxBatchInsertSize(new(models.Release))
	case "pullrequest":
		return models.MaxBatchInsertSize(new(models.PullRequest))
	}
	return 10
}

// CreateRepo creates a repository
func (g *GiteaLocalUploader) CreateRepo(repo *base.Repository, opts base.MigrateOptions) error {
	owner, err := models.GetUserByName(g.repoOwner)
	if err != nil {
		return err
	}

	var remoteAddr = repo.CloneURL
	if len(opts.AuthUsername) > 0 {
		u, err := url.Parse(repo.CloneURL)
		if err != nil {
			return err
		}
		u.User = url.UserPassword(opts.AuthUsername, opts.AuthPassword)
		remoteAddr = u.String()
	}

	r, err := models.MigrateRepository(g.doer, owner, models.MigrateRepoOptions{
		Name:                 g.repoName,
		Description:          repo.Description,
		IsMirror:             repo.IsMirror,
		RemoteAddr:           remoteAddr,
		IsPrivate:            repo.IsPrivate,
		Wiki:                 opts.Wiki,
		SyncReleasesWithTags: !opts.Releases, // if didn't get releases, then sync them from tags
	})
	g.repo = r
	if err != nil {
		return err
	}
	g.gitRepo, err = git.OpenRepository(r.RepoPath())
	return err
}

// Close closes this uploader
func (g *GiteaLocalUploader) Close() {
	if g.gitRepo != nil {
		g.gitRepo.Close()
	}
}

// CreateMilestones creates milestones
func (g *GiteaLocalUploader) CreateMilestones(milestones ...*base.Milestone) error {
	var mss = make([]*models.Milestone, 0, len(milestones))
	for _, milestone := range milestones {
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
			IsClosed:     milestone.State == "closed",
			DeadlineUnix: deadline,
		}
		if ms.IsClosed && milestone.Closed != nil {
			ms.ClosedDateUnix = util.TimeStamp(milestone.Closed.Unix())
		}
		mss = append(mss, &ms)
	}

	err := models.InsertMilestones(mss...)
	if err != nil {
		return err
	}

	for _, ms := range mss {
		g.milestones.Store(ms.Name, ms.ID)
	}
	return nil
}

// CreateLabels creates labels
func (g *GiteaLocalUploader) CreateLabels(labels ...*base.Label) error {
	var lbs = make([]*models.Label, 0, len(labels))
	for _, label := range labels {
		lbs = append(lbs, &models.Label{
			RepoID:      g.repo.ID,
			Name:        label.Name,
			Description: label.Description,
			Color:       fmt.Sprintf("#%s", label.Color),
		})
	}

	err := models.NewLabels(lbs...)
	if err != nil {
		return err
	}
	for _, lb := range lbs {
		g.labels.Store(lb.Name, lb)
	}
	return nil
}

// CreateReleases creates releases
func (g *GiteaLocalUploader) CreateReleases(releases ...*base.Release) error {
	var rels = make([]*models.Release, 0, len(releases))
	for _, release := range releases {
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

		rels = append(rels, &rel)
	}
	if err := models.InsertReleases(rels...); err != nil {
		return err
	}

	// sync tags to releases in database
	return models.SyncReleasesWithTags(g.repo, g.gitRepo)
}

// CreateIssues creates issues
func (g *GiteaLocalUploader) CreateIssues(issues ...*base.Issue) error {
	var iss = make([]*models.Issue, 0, len(issues))
	for _, issue := range issues {
		var labels []*models.Label
		for _, label := range issue.Labels {
			lb, ok := g.labels.Load(label.Name)
			if ok {
				labels = append(labels, lb.(*models.Label))
			}
		}

		var milestoneID int64
		if issue.Milestone != "" {
			milestone, ok := g.milestones.Load(issue.Milestone)
			if ok {
				milestoneID = milestone.(int64)
			}
		}

		var is = models.Issue{
			RepoID:      g.repo.ID,
			Repo:        g.repo,
			Index:       issue.Number,
			PosterID:    g.doer.ID,
			Title:       issue.Title,
			Content:     issue.Content,
			IsClosed:    issue.State == "closed",
			IsLocked:    issue.IsLocked,
			MilestoneID: milestoneID,
			Labels:      labels,
			CreatedUnix: util.TimeStamp(issue.Created.Unix()),
		}
		if issue.Closed != nil {
			is.ClosedUnix = util.TimeStamp(issue.Closed.Unix())
		}
		// TODO: add reactions
		iss = append(iss, &is)
	}

	err := models.InsertIssues(iss...)
	if err != nil {
		return err
	}
	for _, is := range iss {
		g.issues.Store(is.Index, is.ID)
	}
	return nil
}

// CreateComments creates comments of issues
func (g *GiteaLocalUploader) CreateComments(comments ...*base.Comment) error {
	var cms = make([]*models.Comment, 0, len(comments))
	for _, comment := range comments {
		var issueID int64
		if issueIDStr, ok := g.issues.Load(comment.IssueIndex); !ok {
			issue, err := models.GetIssueByIndex(g.repo.ID, comment.IssueIndex)
			if err != nil {
				return err
			}
			issueID = issue.ID
			g.issues.Store(comment.IssueIndex, issueID)
		} else {
			issueID = issueIDStr.(int64)
		}

		cms = append(cms, &models.Comment{
			IssueID:     issueID,
			Type:        models.CommentTypeComment,
			PosterID:    g.doer.ID,
			Content:     comment.Content,
			CreatedUnix: util.TimeStamp(comment.Created.Unix()),
		})

		// TODO: Reactions
	}

	return models.InsertIssueComments(cms)
}

// CreatePullRequests creates pull requests
func (g *GiteaLocalUploader) CreatePullRequests(prs ...*base.PullRequest) error {
	var gprs = make([]*models.PullRequest, 0, len(prs))
	for _, pr := range prs {
		gpr, err := g.newPullRequest(pr)
		if err != nil {
			return err
		}
		gprs = append(gprs, gpr)
	}
	if err := models.InsertPullRequests(gprs...); err != nil {
		return err
	}
	for _, pr := range gprs {
		g.issues.Store(pr.Issue.Index, pr.Issue.ID)
	}
	return nil
}

func (g *GiteaLocalUploader) newPullRequest(pr *base.PullRequest) (*models.PullRequest, error) {
	var labels []*models.Label
	for _, label := range pr.Labels {
		lb, ok := g.labels.Load(label.Name)
		if ok {
			labels = append(labels, lb.(*models.Label))
		}
	}

	var milestoneID int64
	if pr.Milestone != "" {
		milestone, ok := g.milestones.Load(pr.Milestone)
		if ok {
			milestoneID = milestone.(int64)
		}
	}

	// download patch file
	resp, err := http.Get(pr.PatchURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	pullDir := filepath.Join(g.repo.RepoPath(), "pulls")
	if err = os.MkdirAll(pullDir, os.ModePerm); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(pullDir, fmt.Sprintf("%d.patch", pr.Number)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return nil, err
	}

	// set head information
	pullHead := filepath.Join(g.repo.RepoPath(), "refs", "pull", fmt.Sprintf("%d", pr.Number))
	if err := os.MkdirAll(pullHead, os.ModePerm); err != nil {
		return nil, err
	}
	p, err := os.Create(filepath.Join(pullHead, "head"))
	if err != nil {
		return nil, err
	}
	defer p.Close()
	_, err = p.WriteString(pr.Head.SHA)
	if err != nil {
		return nil, err
	}

	var head = "unknown repository"
	if pr.IsForkPullRequest() {
		if pr.Head.OwnerName != "" {
			remote := pr.Head.OwnerName
			_, ok := g.prHeadCache[remote]
			if !ok {
				// git remote add
				err := g.gitRepo.AddRemote(remote, pr.Head.CloneURL, true)
				if err != nil {
					log.Error("AddRemote failed: %s", err)
				} else {
					g.prHeadCache[remote] = struct{}{}
					ok = true
				}
			}

			if ok {
				_, err = git.NewCommand("fetch", remote, pr.Head.Ref).RunInDir(g.repo.RepoPath())
				if err != nil {
					log.Error("Fetch branch from %s failed: %v", pr.Head.CloneURL, err)
				} else {
					headBranch := filepath.Join(g.repo.RepoPath(), "refs", "heads", pr.Head.OwnerName, pr.Head.Ref)
					if err := os.MkdirAll(filepath.Dir(headBranch), os.ModePerm); err != nil {
						return nil, err
					}
					b, err := os.Create(headBranch)
					if err != nil {
						return nil, err
					}
					defer b.Close()
					_, err = b.WriteString(pr.Head.SHA)
					if err != nil {
						return nil, err
					}
					head = pr.Head.OwnerName + "/" + pr.Head.Ref
				}
			}
		}
	} else {
		head = pr.Head.Ref
	}

	var pullRequest = models.PullRequest{
		HeadRepoID:   g.repo.ID,
		HeadBranch:   head,
		HeadUserName: g.repoOwner,
		BaseRepoID:   g.repo.ID,
		BaseBranch:   pr.Base.Ref,
		MergeBase:    pr.Base.SHA,
		Index:        pr.Number,
		HasMerged:    pr.Merged,

		Issue: &models.Issue{
			RepoID:      g.repo.ID,
			Repo:        g.repo,
			Title:       pr.Title,
			Index:       pr.Number,
			PosterID:    g.doer.ID,
			Content:     pr.Content,
			MilestoneID: milestoneID,
			IsPull:      true,
			IsClosed:    pr.State == "closed",
			IsLocked:    pr.IsLocked,
			Labels:      labels,
			CreatedUnix: util.TimeStamp(pr.Created.Unix()),
		},
	}

	if pullRequest.Issue.IsClosed && pr.Closed != nil {
		pullRequest.Issue.ClosedUnix = util.TimeStamp(pr.Closed.Unix())
	}
	if pullRequest.HasMerged && pr.MergedTime != nil {
		pullRequest.MergedUnix = util.TimeStamp(pr.MergedTime.Unix())
		pullRequest.MergedCommitID = pr.MergeCommitSHA
		pullRequest.MergerID = g.doer.ID
	}

	// TODO: reactions
	// TODO: assignees

	return &pullRequest, nil
}

// Rollback when migrating failed, this will rollback all the changes.
func (g *GiteaLocalUploader) Rollback() error {
	if g.repo != nil && g.repo.ID > 0 {
		if err := models.DeleteRepository(g.doer, g.repo.OwnerID, g.repo.ID); err != nil {
			return err
		}
	}
	return nil
}
