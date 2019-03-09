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

// GithubDownloader implements a Downloader interface to get repository informations
// from github via APIv3
type GithubDownloaderV3 struct {
	ctx       context.Context
	client    *github.Client
	repoOwner string
	repoName  string
}

// NewGithubDownloaderV3 creates a github Downloader via github v3 API
func NewGithubDownloaderV3(token, repoOwner, repoName string) *GithubDownloaderV3 {
	var downloader = GithubDownloaderV3{
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
			})
		}
		if len(ms) < perPage {
			break
		}
	}
	return milestones, nil
}

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
			if pr.Merged != nil {
				merged = *pr.Merged
			}

			var headRepoName string
			if pr.Head.Repo != nil {
				headRepoName = *pr.Head.Repo.Name
			}

			allPRs = append(allPRs, &base.PullRequest{
				Title:       *pr.Title,
				Number:      int64(*pr.Number),
				PosterName:  *pr.User.Login,
				PosterEmail: email,
				Content:     body,
				Milestone:   milestone,
				State:       *pr.State,
				Created:     *pr.CreatedAt,
				Labels:      labels,
				Merged:      merged,
				Head: base.PullRequestBranch{
					Ref:       *pr.Head.Ref,
					SHA:       *pr.Head.SHA,
					RepoName:  headRepoName,
					OwnerName: *pr.Head.User.Login,
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
