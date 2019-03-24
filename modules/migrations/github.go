// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/migrations/base"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var (
	_ base.Downloader = &GithubDownloaderV3{}
)

// GithubDownloaderV3 implements a Downloader interface to get repository informations
// from github via APIv3
type GithubDownloaderV3 struct {
	ctx       context.Context
	client    *github.Client
	repoOwner string
	repoName  string
	token     string
}

// NewGithubDownloaderV3 creates a github Downloader via github v3 API
func NewGithubDownloaderV3(token, repoOwner, repoName string) *GithubDownloaderV3 {
	var downloader = GithubDownloaderV3{
		token:     token,
		ctx:       context.Background(),
		repoOwner: repoOwner,
		repoName:  repoName,
	}

	var client *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		client = oauth2.NewClient(downloader.ctx, ts)
	}
	downloader.client = github.NewClient(client)
	return &downloader
}

// NewGithubDownloaderV3WithClient creates a github Downloader via github v3 API with exsting client
func NewGithubDownloaderV3WithClient(ghClient *github.Client, repoOwner, repoName string) *GithubDownloaderV3 {
	return &GithubDownloaderV3{
		ctx:       context.Background(),
		repoOwner: repoOwner,
		repoName:  repoName,
		client:    ghClient,
	}
}

// GetRepoInfo returns a repository information
func (g *GithubDownloaderV3) GetRepoInfo() (*base.Repository, error) {
	gr, _, err := g.client.Repositories.Get(g.ctx, g.repoOwner, g.repoName)
	if err != nil {
		return nil, fmt.Errorf("GHClient Repostiories Get: %v", err)
	}

	// convert github repo to stand Repo
	return &base.Repository{
		Owner:       g.repoOwner,
		Name:        gr.GetName(),
		IsPrivate:   *gr.Private,
		Description: gr.GetDescription(),
		CloneURL:    gr.GetCloneURL(),
	}, nil
}

// GetMilestones returns milestones
func (g *GithubDownloaderV3) GetMilestones() ([]*base.Milestone, error) {
	var perPage = 100
	var milestones = make([]*base.Milestone, 0, perPage)
	for i := 1; ; i++ {
		ms, _, err := g.client.Issues.ListMilestones(g.ctx, g.repoOwner, g.repoName,
			&github.MilestoneListOptions{
				State: "all",
				ListOptions: github.ListOptions{
					Page:    i,
					PerPage: perPage,
				}})
		if err != nil {
			return nil, err
		}

		for _, m := range ms {
			var desc string
			if m.Description != nil {
				desc = *m.Description
			}
			var state = "open"
			if m.State != nil {
				state = *m.State
			}
			milestones = append(milestones, &base.Milestone{
				Title:       *m.Title,
				Description: desc,
				Deadline:    m.DueOn,
				State:       state,
				Created:     *m.CreatedAt,
				Updated:     m.UpdatedAt,
				Closed:      m.ClosedAt,
			})
		}
		if len(ms) < perPage {
			break
		}
	}
	return milestones, nil
}

func convertGithubLabel(label *github.Label) *base.Label {
	var desc string
	if label.Description != nil {
		desc = *label.Description
	}
	return &base.Label{
		Name:        *label.Name,
		Color:       *label.Color,
		Description: desc,
	}
}

// GetLabels returns labels
func (g *GithubDownloaderV3) GetLabels() ([]*base.Label, error) {
	var perPage = 100
	var labels = make([]*base.Label, 0, perPage)
	for i := 1; ; i++ {
		ls, _, err := g.client.Issues.ListLabels(g.ctx, g.repoOwner, g.repoName,
			&github.ListOptions{
				Page:    i,
				PerPage: perPage,
			})
		if err != nil {
			return nil, err
		}

		for _, label := range ls {
			labels = append(labels, convertGithubLabel(label))
		}
		if len(ls) < perPage {
			break
		}
	}
	return labels, nil
}

func (g *GithubDownloaderV3) convertGithubRelease(rel *github.RepositoryRelease) *base.Release {
	var (
		name string
		desc string
	)
	if rel.Body != nil {
		desc = *rel.Body
	}
	if rel.Name != nil {
		name = *rel.Name
	}

	r := &base.Release{
		TagName:         *rel.TagName,
		TargetCommitish: *rel.TargetCommitish,
		Name:            name,
		Body:            desc,
		Draft:           *rel.Draft,
		Prerelease:      *rel.Prerelease,
		Created:         rel.CreatedAt.Time,
		Published:       rel.PublishedAt.Time,
	}

	for _, asset := range rel.Assets {
		r.Assets = append(r.Assets, base.ReleaseAsset{
			URL:           *asset.BrowserDownloadURL + "?access_token=" + g.token,
			Name:          *asset.Name,
			ContentType:   asset.ContentType,
			Size:          asset.Size,
			DownloadCount: asset.DownloadCount,
			Created:       asset.CreatedAt.Time,
			Updated:       asset.UpdatedAt.Time,
		})
	}
	return r
}

// GetReleases returns releases
func (g *GithubDownloaderV3) GetReleases() ([]*base.Release, error) {
	var perPage = 100
	var releases = make([]*base.Release, 0, perPage)
	for i := 1; ; i++ {
		ls, _, err := g.client.Repositories.ListReleases(g.ctx, g.repoOwner, g.repoName,
			&github.ListOptions{
				Page:    i,
				PerPage: perPage,
			})
		if err != nil {
			return nil, err
		}

		for _, release := range ls {
			releases = append(releases, g.convertGithubRelease(release))
		}
		if len(ls) < perPage {
			break
		}
	}
	return releases, nil
}

func convertGithubReactions(reactions *github.Reactions) *base.Reactions {
	return &base.Reactions{
		TotalCount: *reactions.TotalCount,
		PlusOne:    *reactions.PlusOne,
		MinusOne:   *reactions.MinusOne,
		Laugh:      *reactions.Laugh,
		Confused:   *reactions.Confused,
		Heart:      *reactions.Heart,
		Hooray:     *reactions.Hooray,
	}
}

// GetIssues returns issues according start and limit
func (g *GithubDownloaderV3) GetIssues(start, limit int) ([]*base.Issue, error) {
	var perPage = 100
	opt := &github.IssueListByRepoOptions{
		Sort:      "created",
		Direction: "asc",
		State:     "all",
		ListOptions: github.ListOptions{
			PerPage: perPage,
		},
	}
	var allIssues = make([]*base.Issue, 0, limit)
	for {
		issues, resp, err := g.client.Issues.ListByRepo(g.ctx, g.repoOwner, g.repoName, opt)
		if err != nil {
			return nil, fmt.Errorf("error while listing repos: %v", err)
		}
		for _, issue := range issues {
			if issue.IsPullRequest() {
				continue
			}
			var body string
			if issue.Body != nil {
				body = *issue.Body
			}
			var milestone string
			if issue.Milestone != nil {
				milestone = *issue.Milestone.Title
			}
			var labels = make([]*base.Label, 0, len(issue.Labels))
			for _, l := range issue.Labels {
				labels = append(labels, convertGithubLabel(&l))
			}
			var reactions *base.Reactions
			if issue.Reactions != nil {
				reactions = convertGithubReactions(issue.Reactions)
			}

			var email string
			if issue.User.Email != nil {
				email = *issue.User.Email
			}
			allIssues = append(allIssues, &base.Issue{
				Title:       *issue.Title,
				Number:      int64(*issue.Number),
				PosterName:  *issue.User.Login,
				PosterEmail: email,
				Content:     body,
				Milestone:   milestone,
				State:       *issue.State,
				Created:     *issue.CreatedAt,
				Labels:      labels,
				Reactions:   reactions,
				Closed:      issue.ClosedAt,
				IsLocked:    *issue.Locked,
			})
			if len(allIssues) >= limit {
				return allIssues, nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allIssues, nil
}

// GetComments returns comments according issueNumber
func (g *GithubDownloaderV3) GetComments(issueNumber int64) ([]*base.Comment, error) {
	var allComments = make([]*base.Comment, 0, 100)
	opt := &github.IssueListCommentsOptions{
		Sort:      "created",
		Direction: "asc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	for {
		comments, resp, err := g.client.Issues.ListComments(g.ctx, g.repoOwner, g.repoName, int(issueNumber), opt)
		if err != nil {
			return nil, fmt.Errorf("error while listing repos: %v", err)
		}
		for _, comment := range comments {
			var email string
			if comment.User.Email != nil {
				email = *comment.User.Email
			}
			var reactions *base.Reactions
			if comment.Reactions != nil {
				reactions = convertGithubReactions(comment.Reactions)
			}
			allComments = append(allComments, &base.Comment{
				PosterName:  *comment.User.Login,
				PosterEmail: email,
				Content:     *comment.Body,
				Created:     *comment.CreatedAt,
				Reactions:   reactions,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allComments, nil
}

// GetPullRequests returns pull requests according start and limit
func (g *GithubDownloaderV3) GetPullRequests(start, limit int) ([]*base.PullRequest, error) {
	opt := &github.PullRequestListOptions{
		Sort:      "created",
		Direction: "asc",
		State:     "all",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	var allPRs = make([]*base.PullRequest, 0, 100)
	for {
		prs, resp, err := g.client.PullRequests.List(g.ctx, g.repoOwner, g.repoName, opt)
		if err != nil {
			return nil, fmt.Errorf("error while listing repos: %v", err)
		}
		for _, pr := range prs {
			var body string
			if pr.Body != nil {
				body = *pr.Body
			}
			var milestone string
			if pr.Milestone != nil {
				milestone = *pr.Milestone.Title
			}
			var labels = make([]*base.Label, 0, len(pr.Labels))
			for _, l := range pr.Labels {
				labels = append(labels, convertGithubLabel(l))
			}
			// FIXME: This API missing reactions, we may need another extra request to get reactions
			/*var reactions *Reactions
			if pr.Reactions != nil {
				reactions = convertGithubReactions(pr.Reactions)
			}*/

			var email string
			if pr.User.Email != nil {
				email = *pr.User.Email
			}
			var merged bool
			// ? pr.Merged is not valid, so use MergedAt to test if it's merged
			if pr.MergedAt != nil {
				merged = true
			}

			var headRepoName string
			var cloneURL string
			if pr.Head.Repo != nil {
				headRepoName = *pr.Head.Repo.Name
				cloneURL = *pr.Head.Repo.CloneURL
			}
			var mergeCommitSHA string
			if pr.MergeCommitSHA != nil {
				mergeCommitSHA = *pr.MergeCommitSHA
			}

			allPRs = append(allPRs, &base.PullRequest{
				Title:          *pr.Title,
				Number:         int64(*pr.Number),
				PosterName:     *pr.User.Login,
				PosterEmail:    email,
				Content:        body,
				Milestone:      milestone,
				State:          *pr.State,
				Created:        *pr.CreatedAt,
				Closed:         pr.ClosedAt,
				Labels:         labels,
				Merged:         merged,
				MergeCommitSHA: mergeCommitSHA,
				MergedTime:     pr.MergedAt,
				IsLocked:       pr.ActiveLockReason != nil,
				Head: base.PullRequestBranch{
					Ref:       *pr.Head.Ref,
					SHA:       *pr.Head.SHA,
					RepoName:  headRepoName,
					OwnerName: *pr.Head.User.Login,
					CloneURL:  cloneURL,
				},
				Base: base.PullRequestBranch{
					Ref:       *pr.Base.Ref,
					SHA:       *pr.Base.SHA,
					RepoName:  *pr.Base.Repo.Name,
					OwnerName: *pr.Base.User.Login,
				},
				//Reactions:   reactions,
				PatchURL: *pr.PatchURL,
			})
			if len(allPRs) >= limit {
				return allPRs, nil
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allPRs, nil
}
