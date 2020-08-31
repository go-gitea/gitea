// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/structs"

	"code.gitea.io/sdk/gitea"
)

var (
	_ base.Downloader        = &GiteaDownloader{}
	_ base.DownloaderFactory = &GiteaDownloaderFactory{}
)

func init() {
	RegisterDownloaderFactory(&GiteaDownloaderFactory{})
}

// GiteaDownloaderFactory defines a gitea downloader factory
type GiteaDownloaderFactory struct {
}

// New returns a Downloader related to this factory according MigrateOptions
func (f *GiteaDownloaderFactory) New(opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	baseURL := u.Scheme + "://" + u.Host
	repoNameSpace := strings.TrimPrefix(u.Path, "/")
	repoNameSpace = strings.TrimSuffix(repoNameSpace, ".git")

	path := strings.Split(repoNameSpace, "/")
	if len(path) < 2 {
		return nil, fmt.Errorf("invalid path")
	}

	//ToDo handle gitea installed in subpath ...
	repoPath := repoNameSpace

	log.Trace("Create gitea downloader. BaseURL: %s RepoName: %s", baseURL, repoNameSpace)

	return NewGiteaDownloader(baseURL, repoPath, opts.AuthUsername, opts.AuthPassword, opts.AuthToken), nil
}

// GitServiceType returns the type of git service
func (f *GiteaDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.GiteaService
}

// GiteaDownloader implements a Downloader interface to get repository information's
type GiteaDownloader struct {
	ctx       context.Context
	client    *gitea.Client
	repoOwner string
	repoName  string
}

// NewGiteaDownloader creates a gitea Downloader via gitea API
//   Use either a username/password, personal token entered into the username field, or anonymous/public access
//   Note: Public access only allows very basic access
func NewGiteaDownloader(baseURL, repoPath, username, password, token string) *GiteaDownloader {
	giteaClient := gitea.NewClient(baseURL, token)
	if token == "" {
		giteaClient.SetBasicAuth(username, password)
	}

	path := strings.Split(repoPath, "/")

	return &GiteaDownloader{
		ctx:       context.Background(),
		client:    giteaClient,
		repoOwner: path[0],
		repoName:  path[1],
	}
}

// SetContext set context
func (g *GiteaDownloader) SetContext(ctx context.Context) {
	g.ctx = ctx
}

// GetRepoInfo returns a repository information
func (g *GiteaDownloader) GetRepoInfo() (*base.Repository, error) {
	if g == nil {
		return nil, errors.New("error: GiteaDownloader is nil")
	}

	repo, err := g.client.GetRepo(g.repoOwner, g.repoName)
	if err != nil {
		return nil, err
	}

	return &base.Repository{
		Name:        repo.Name,
		Owner:       repo.Owner.UserName,
		IsPrivate:   repo.Private,
		Description: repo.Description,
		CloneURL:    repo.CloneURL,
		OriginalURL: repo.HTMLURL,
	}, nil
}

// GetTopics return gitea topics
func (g *GiteaDownloader) GetTopics() ([]string, error) {
	if g == nil {
		return nil, errors.New("error: GiteaDownloader is nil")
	}

	return g.client.ListRepoTopics(g.repoOwner, g.repoName, gitea.ListRepoTopicsOptions{})
}

// GetMilestones returns milestones
func (g *GiteaDownloader) GetMilestones() ([]*base.Milestone, error) {
	if g == nil {
		return nil, errors.New("error: GiteaDownloader is nil")
	}
	var perPage = 50
	var milestones = make([]*base.Milestone, 0, perPage)

	for i := 1; ; i++ {
		ms, err := g.client.ListRepoMilestones(g.repoOwner, g.repoName, gitea.ListMilestoneOption{
			ListOptions: gitea.ListOptions{
				PageSize: perPage,
				Page:     i,
			},
			State: gitea.StateAll,
		})
		if err != nil {
			return nil, err
		}

		for i := range ms {
			// ToDo: expose this info
			// https://github.com/go-gitea/gitea/issues/12655
			createdAT := time.Now()
			var updatedAT *time.Time
			if ms[i].Closed != nil {
				createdAT = *ms[i].Closed
				updatedAT = ms[i].Closed
			}

			milestones = append(milestones, &base.Milestone{
				Title:       ms[i].Title,
				Description: ms[i].Description,
				Deadline:    ms[i].Deadline,
				Created:     createdAT,
				Updated:     updatedAT,
				Closed:      ms[i].Closed,
				State:       string(ms[i].State),
			})
		}
		if len(ms) < perPage {
			break
		}
	}
	return milestones, nil
}

// GetLabels returns labels
func (g *GiteaDownloader) GetLabels() ([]*base.Label, error) {
	if g == nil {
		return nil, errors.New("error: GiteaDownloader is nil")
	}

	var perPage = 50
	var labels = make([]*base.Label, 0, perPage)

	for i := 1; ; i++ {
		ls, err := g.client.ListRepoLabels(g.repoOwner, g.repoName, gitea.ListLabelsOptions{ListOptions: gitea.ListOptions{
			PageSize: perPage,
			Page:     i,
		}})
		if err != nil {
			return nil, err
		}

		for i := range ls {
			labels = append(labels, &base.Label{
				Name:        ls[i].Name,
				Color:       ls[i].Color,
				Description: ls[i].Description,
			})
		}
		if len(ls) < perPage {
			break
		}
	}
	return labels, nil
}

func (g *GiteaDownloader) convertGiteaRelease(rel *gitea.Release) *base.Release {
	r := &base.Release{
		TagName:         rel.TagName,
		TargetCommitish: rel.Target,
		Name:            rel.Title,
		Body:            rel.Note,
		Draft:           rel.IsDraft,
		Prerelease:      rel.IsPrerelease,
		PublisherID:     rel.Publisher.ID,
		PublisherName:   rel.Publisher.UserName,
		PublisherEmail:  rel.Publisher.Email,
		Published:       rel.PublishedAt,
		Created:         rel.CreatedAt,
	}

	for _, asset := range rel.Attachments {
		size := int(asset.Size)
		dlCount := int(asset.DownloadCount)
		r.Assets = append(r.Assets, base.ReleaseAsset{
			ID:            asset.ID,
			Name:          asset.Name,
			Size:          &size,
			DownloadCount: &dlCount,
			Created:       asset.Created,
			DownloadURL:   &asset.DownloadURL,
		})
	}
	return r
}

// GetReleases returns releases
func (g *GiteaDownloader) GetReleases() ([]*base.Release, error) {
	var perPage = 100
	var releases = make([]*base.Release, 0, perPage)
	for i := 1; ; i++ {
		rl, err := g.client.ListReleases(g.repoOwner, g.repoName, gitea.ListReleasesOptions{ListOptions: gitea.ListOptions{
			PageSize: perPage,
			Page:     i,
		}})
		if err != nil {
			return nil, err
		}

		for i := range rl {
			releases = append(releases, g.convertGiteaRelease(rl[i]))
		}
		if len(rl) < perPage {
			break
		}
	}
	return releases, nil
}

// GetAsset returns an asset
func (g *GiteaDownloader) GetAsset(_ string, relID, id int64) (io.ReadCloser, error) {

	asset, err := g.client.GetReleaseAttachment(g.repoOwner, g.repoName, relID, id)
	if err != nil {
		return nil, err
	}
	resp, err := http.Get(asset.DownloadURL)
	if err != nil {
		return nil, err
	}

	// resp.Body is closed by the uploader
	return resp.Body, nil
}

// getIssueReactions
func (g *GiteaDownloader) getIssueReactions(index int64) ([]*base.Reaction, error) {
	var reactions []*base.Reaction
	rl, err := g.client.GetIssueReactions(g.repoOwner, g.repoName, index)
	if err != nil {
		return nil, err
	}

	for _, reaction := range rl {
		reactions = append(reactions, &base.Reaction{
			UserID:   reaction.User.ID,
			UserName: reaction.User.UserName,
			Content:  reaction.Reaction,
		})
	}
	return reactions, nil
}

// GetIssues returns issues according start and limit
func (g *GiteaDownloader) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {

	var allIssues = make([]*base.Issue, 0, perPage)

	issues, err := g.client.ListRepoIssues(g.repoOwner, g.repoName, gitea.ListIssueOption{
		ListOptions: gitea.ListOptions{Page: page, PageSize: perPage},
		State:       gitea.StateAll,
		Type:        gitea.IssueTypeIssue,
	})
	if err != nil {
		return nil, false, fmt.Errorf("error while listing issues: %v", err)
	}
	for _, issue := range issues {

		var labels = make([]*base.Label, 0, len(issue.Labels))
		for i := range issue.Labels {
			labels = append(labels, &base.Label{
				Name:        issue.Labels[i].Name,
				Color:       issue.Labels[i].Color,
				Description: issue.Labels[i].Description,
			})
		}

		var milestone string
		if issue.Milestone != nil {
			milestone = issue.Milestone.Title
		}

		reactions, err := g.getIssueReactions(issue.Index)
		if err != nil {
			return nil, false, fmt.Errorf("error while geting reactions: %v", err)
		}

		allIssues = append(allIssues, &base.Issue{
			Title:       issue.Title,
			Number:      issue.Index,
			PosterID:    issue.Poster.ID,
			PosterName:  issue.Poster.UserName,
			PosterEmail: issue.Poster.Email,
			Content:     issue.Body,
			Milestone:   milestone,
			State:       string(issue.State),
			Created:     issue.Created,
			Updated:     issue.Updated,
			Closed:      issue.Closed,
			Reactions:   reactions,
			Labels:      labels,
			IsLocked:    issue.IsLocked,
		})
	}

	return allIssues, len(issues) == 0, nil
}

// GetComments returns comments according issueNumber
func (g *GiteaDownloader) GetComments(index int64) ([]*base.Comment, error) {

	var perPage = 50
	var allComments = make([]*base.Comment, 0, 100)

	for i := 1; ; i++ {
		comments, err := g.client.ListIssueComments(g.repoOwner, g.repoName, index, gitea.ListIssueCommentOptions{ListOptions: gitea.ListOptions{
			PageSize: perPage,
			Page:     i,
		}})
		if err != nil {
			return nil, fmt.Errorf("error while listing comments: %v", err)
		}
		if len(comments) == 0 {
			break
		}
		for _, comment := range comments {
			rl, err := g.client.GetIssueCommentReactions(g.repoOwner, g.repoName, comment.ID)
			if err != nil {
				return nil, err
			}
			var reactions []*base.Reaction
			for i := range rl {
				reactions = append(reactions, &base.Reaction{
					UserID:   rl[i].User.ID,
					UserName: rl[i].User.UserName,
					Content:  rl[i].Reaction,
				})
			}

			allComments = append(allComments, &base.Comment{
				IssueIndex:  index,
				PosterID:    comment.Poster.ID,
				PosterName:  comment.Poster.UserName,
				PosterEmail: comment.Poster.Email,
				Content:     comment.Body,
				Created:     comment.Created,
				Updated:     comment.Updated,
				Reactions:   reactions,
			})
		}
	}
	return allComments, nil
}

// GetPullRequests returns pull requests according page and perPage
func (g *GiteaDownloader) GetPullRequests(page, perPage int) ([]*base.PullRequest, error) {

	var allPRs = make([]*base.PullRequest, 0, perPage)

	prs, err := g.client.ListRepoPullRequests(g.repoOwner, g.repoName, gitea.ListPullRequestsOptions{
		ListOptions: gitea.ListOptions{
			Page:     page,
			PageSize: perPage,
		},
		State: gitea.StateAll,
	})
	if err != nil {
		return nil, fmt.Errorf("error while listing repos: %v", err)
	}
	for _, pr := range prs {
		var milestone string
		if pr.Milestone != nil {
			milestone = pr.Milestone.Title
		}

		var labels = make([]*base.Label, 0, len(pr.Labels))
		for i := range pr.Labels {
			labels = append(labels, &base.Label{
				Name:        pr.Labels[i].Name,
				Color:       pr.Labels[i].Color,
				Description: pr.Labels[i].Description,
			})
		}

		var (
			headUserName string
			headRepoName string
			headCloneURL string
			headRef      string
			headSHA      string
		)
		if pr.Head != nil {
			if pr.Head.Repository != nil {
				headUserName = pr.Head.Repository.Owner.UserName
				headRepoName = pr.Head.Repository.Name
			}
			headSHA = pr.Head.Sha
			headRef = pr.Head.Ref
		}

		var mergeCommitSHA string
		if pr.MergedCommitID != nil {
			mergeCommitSHA = *pr.MergedCommitID
		}

		reactions, err := g.getIssueReactions(pr.Index)
		if err != nil {
			return nil, fmt.Errorf("error while geting reactions: %v", err)
		}

		var assignees []string
		for i := range pr.Assignees {
			assignees = append(assignees, pr.Assignees[i].UserName)
		}

		createdAt := time.Now()
		if pr.Created != nil {
			createdAt = *pr.Created
		}
		updatedAt := time.Now()
		if pr.Created != nil {
			updatedAt = *pr.Updated
		}

		allPRs = append(allPRs, &base.PullRequest{
			Title:          pr.Title,
			Number:         pr.Index,
			PosterID:       pr.Poster.ID,
			PosterName:     pr.Poster.UserName,
			PosterEmail:    pr.Poster.Email,
			Content:        pr.Body,
			State:          string(pr.State),
			Created:        createdAt,
			Updated:        updatedAt,
			Closed:         pr.Closed,
			Labels:         labels,
			Milestone:      milestone,
			Reactions:      reactions,
			Assignees:      assignees,
			Merged:         pr.HasMerged,
			MergedTime:     pr.Merged,
			MergeCommitSHA: mergeCommitSHA,
			IsLocked:       pr.IsLocked,
			PatchURL:       pr.PatchURL,
			Head: base.PullRequestBranch{
				Ref:       headRef,
				SHA:       headSHA,
				RepoName:  headRepoName,
				OwnerName: headUserName,
				CloneURL:  headCloneURL,
			},
			Base: base.PullRequestBranch{
				Ref:       pr.Base.Ref,
				SHA:       pr.Base.Sha,
				RepoName:  g.repoName,
				OwnerName: g.repoOwner,
			},
		})
	}

	return allPRs, nil
}

// GetReviews returns pull requests review
func (g *GiteaDownloader) GetReviews(index int64) ([]*base.Review, error) {

	var perPage = 50
	var allReviews = make([]*base.Review, 0, perPage)

	for i := 1; ; i++ {
		prl, err := g.client.ListPullReviews(g.repoOwner, g.repoName, index, gitea.ListPullReviewsOptions{ListOptions: gitea.ListOptions{
			Page:     i,
			PageSize: perPage,
		}})
		if err != nil {
			return nil, err
		}

		for _, pr := range prl {

			var reviewComments []*base.ReviewComment
			for ii := 1; ; ii++ {
				rcl, err := g.client.ListPullReviewComments(g.repoOwner, g.repoName, index, pr.ID, gitea.ListPullReviewsCommentsOptions{})
				if err != nil {
					return nil, err
				}
				for i := range rcl {
					line := int(rcl[i].LineNum)
					if rcl[i].OldLineNum > 0 {
						line = int(rcl[i].OldLineNum) * -1
					}

					reviewComments = append(reviewComments, &base.ReviewComment{
						ID:        rcl[i].ID,
						Content:   rcl[i].Body,
						TreePath:  rcl[i].Path,
						DiffHunk:  rcl[i].DiffHunk,
						Position:  line,
						CommitID:  rcl[i].CommitID,
						PosterID:  rcl[i].Reviewer.ID,
						CreatedAt: rcl[i].Created,
						UpdatedAt: rcl[i].Updated,
					})
				}
				if len(rcl) < perPage {
					break
				}
			}

			allReviews = append(allReviews, &base.Review{
				ID:           pr.ID,
				IssueIndex:   index,
				ReviewerID:   pr.Reviewer.ID,
				ReviewerName: pr.Reviewer.UserName,
				Official:     pr.Official,
				CommitID:     pr.CommitID,
				Content:      pr.Body,
				CreatedAt:    pr.Submitted,
				State:        string(pr.State),
				Comments:     reviewComments,
			})
		}

		if len(prl) < perPage {
			break
		}
	}
	return allReviews, nil
}
