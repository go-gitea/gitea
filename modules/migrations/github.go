// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/proxy"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"
)

var (
	_ base.Downloader        = &GithubDownloaderV3{}
	_ base.DownloaderFactory = &GithubDownloaderV3Factory{}
	// GithubLimitRateRemaining limit to wait for new rate to apply
	GithubLimitRateRemaining = 0
)

func init() {
	RegisterDownloaderFactory(&GithubDownloaderV3Factory{})
}

// GithubDownloaderV3Factory defines a github downloader v3 factory
type GithubDownloaderV3Factory struct {
}

// New returns a Downloader related to this factory according MigrateOptions
func (f *GithubDownloaderV3Factory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	baseURL := u.Scheme + "://" + u.Host
	fields := strings.Split(u.Path, "/")
	oldOwner := fields[1]
	oldName := strings.TrimSuffix(fields[2], ".git")

	log.Trace("Create github downloader: %s/%s", oldOwner, oldName)

	return NewGithubDownloaderV3(ctx, baseURL, opts.AuthUsername, opts.AuthPassword, opts.AuthToken, oldOwner, oldName), nil
}

// GitServiceType returns the type of git service
func (f *GithubDownloaderV3Factory) GitServiceType() structs.GitServiceType {
	return structs.GithubService
}

// GithubDownloaderV3 implements a Downloader interface to get repository information
// from github via APIv3
type GithubDownloaderV3 struct {
	base.NullDownloader
	ctx          context.Context
	clients      []*github.Client
	repoOwner    string
	repoName     string
	userName     string
	password     string
	rates        []*github.Rate
	curClientIdx int
	maxPerPage   int
}

// NewGithubDownloaderV3 creates a github Downloader via github v3 API
func NewGithubDownloaderV3(ctx context.Context, baseURL, userName, password, token, repoOwner, repoName string) *GithubDownloaderV3 {
	var downloader = GithubDownloaderV3{
		userName:   userName,
		password:   password,
		ctx:        ctx,
		repoOwner:  repoOwner,
		repoName:   repoName,
		maxPerPage: 100,
	}

	if token != "" {
		tokens := strings.Split(token, ",")
		for _, token := range tokens {
			token = strings.TrimSpace(token)
			ts := oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: token},
			)
			var client = &http.Client{
				Transport: &oauth2.Transport{
					Base: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: setting.Migrations.SkipTLSVerify},
						Proxy: func(req *http.Request) (*url.URL, error) {
							return proxy.Proxy()(req)
						},
					},
					Source: oauth2.ReuseTokenSource(nil, ts),
				},
			}

			downloader.addClient(client, baseURL)
		}
	} else {
		var client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: setting.Migrations.SkipTLSVerify},
				Proxy: func(req *http.Request) (*url.URL, error) {
					req.SetBasicAuth(userName, password)
					return proxy.Proxy()(req)
				},
			},
		}
		downloader.addClient(client, baseURL)
	}
	return &downloader
}

func (g *GithubDownloaderV3) addClient(client *http.Client, baseURL string) {
	githubClient := github.NewClient(client)
	if baseURL != "https://github.com" {
		githubClient, _ = github.NewEnterpriseClient(baseURL, baseURL, client)
	}
	g.clients = append(g.clients, githubClient)
	g.rates = append(g.rates, nil)
}

// SetContext set context
func (g *GithubDownloaderV3) SetContext(ctx context.Context) {
	g.ctx = ctx
}

func (g *GithubDownloaderV3) waitAndPickClient() {
	var recentIdx int
	var maxRemaining int
	for i := 0; i < len(g.clients); i++ {
		if g.rates[i] != nil && g.rates[i].Remaining > maxRemaining {
			maxRemaining = g.rates[i].Remaining
			recentIdx = i
		}
	}
	g.curClientIdx = recentIdx // if no max remain, it will always pick the first client.

	for g.rates[g.curClientIdx] != nil && g.rates[g.curClientIdx].Remaining <= GithubLimitRateRemaining {
		timer := time.NewTimer(time.Until(g.rates[g.curClientIdx].Reset.Time))
		select {
		case <-g.ctx.Done():
			util.StopTimer(timer)
			return
		case <-timer.C:
		}

		err := g.RefreshRate()
		if err != nil {
			log.Error("g.getClient().RateLimits: %s", err)
		}
	}
}

// RefreshRate update the current rate (doesn't count in rate limit)
func (g *GithubDownloaderV3) RefreshRate() error {
	rates, _, err := g.getClient().RateLimits(g.ctx)
	if err != nil {
		// if rate limit is not enabled, ignore it
		if strings.Contains(err.Error(), "404") {
			g.setRate(nil)
			return nil
		}
		return err
	}

	g.setRate(rates.GetCore())
	return nil
}

func (g *GithubDownloaderV3) getClient() *github.Client {
	return g.clients[g.curClientIdx]
}

func (g *GithubDownloaderV3) setRate(rate *github.Rate) {
	g.rates[g.curClientIdx] = rate
}

// GetRepoInfo returns a repository information
func (g *GithubDownloaderV3) GetRepoInfo() (*base.Repository, error) {
	g.waitAndPickClient()
	gr, resp, err := g.getClient().Repositories.Get(g.ctx, g.repoOwner, g.repoName)
	if err != nil {
		return nil, err
	}
	g.setRate(&resp.Rate)

	// convert github repo to stand Repo
	return &base.Repository{
		Owner:         g.repoOwner,
		Name:          gr.GetName(),
		IsPrivate:     gr.GetPrivate(),
		Description:   gr.GetDescription(),
		OriginalURL:   gr.GetHTMLURL(),
		CloneURL:      gr.GetCloneURL(),
		DefaultBranch: gr.GetDefaultBranch(),
	}, nil
}

// GetTopics return github topics
func (g *GithubDownloaderV3) GetTopics() ([]string, error) {
	g.waitAndPickClient()
	r, resp, err := g.getClient().Repositories.Get(g.ctx, g.repoOwner, g.repoName)
	if err != nil {
		return nil, err
	}
	g.setRate(&resp.Rate)
	return r.Topics, nil
}

// GetMilestones returns milestones
func (g *GithubDownloaderV3) GetMilestones() ([]*base.Milestone, error) {
	var perPage = g.maxPerPage
	var milestones = make([]*base.Milestone, 0, perPage)
	for i := 1; ; i++ {
		g.waitAndPickClient()
		ms, resp, err := g.getClient().Issues.ListMilestones(g.ctx, g.repoOwner, g.repoName,
			&github.MilestoneListOptions{
				State: "all",
				ListOptions: github.ListOptions{
					Page:    i,
					PerPage: perPage,
				}})
		if err != nil {
			return nil, err
		}
		g.setRate(&resp.Rate)

		for _, m := range ms {
			var state = "open"
			if m.State != nil {
				state = *m.State
			}
			milestones = append(milestones, &base.Milestone{
				Title:       m.GetTitle(),
				Description: m.GetDescription(),
				Deadline:    m.DueOn,
				State:       state,
				Created:     m.GetCreatedAt(),
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
	return &base.Label{
		Name:        label.GetName(),
		Color:       label.GetColor(),
		Description: label.GetDescription(),
	}
}

// GetLabels returns labels
func (g *GithubDownloaderV3) GetLabels() ([]*base.Label, error) {
	var perPage = g.maxPerPage
	var labels = make([]*base.Label, 0, perPage)
	for i := 1; ; i++ {
		g.waitAndPickClient()
		ls, resp, err := g.getClient().Issues.ListLabels(g.ctx, g.repoOwner, g.repoName,
			&github.ListOptions{
				Page:    i,
				PerPage: perPage,
			})
		if err != nil {
			return nil, err
		}
		g.setRate(&resp.Rate)

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
	r := &base.Release{
		Name:            rel.GetName(),
		TagName:         rel.GetTagName(),
		TargetCommitish: rel.GetTargetCommitish(),
		Draft:           rel.GetDraft(),
		Prerelease:      rel.GetPrerelease(),
		Created:         rel.GetCreatedAt().Time,
		PublisherID:     rel.GetAuthor().GetID(),
		PublisherName:   rel.GetAuthor().GetLogin(),
		PublisherEmail:  rel.GetAuthor().GetEmail(),
		Body:            rel.GetBody(),
	}

	if rel.PublishedAt != nil {
		r.Published = rel.PublishedAt.Time
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: setting.Migrations.SkipTLSVerify},
			Proxy:           proxy.Proxy(),
		},
	}

	for _, asset := range rel.Assets {
		var assetID = *asset.ID // Don't optimize this, for closure we need a local variable
		r.Assets = append(r.Assets, &base.ReleaseAsset{
			ID:            asset.GetID(),
			Name:          asset.GetName(),
			ContentType:   asset.ContentType,
			Size:          asset.Size,
			DownloadCount: asset.DownloadCount,
			Created:       asset.CreatedAt.Time,
			Updated:       asset.UpdatedAt.Time,
			DownloadFunc: func() (io.ReadCloser, error) {
				g.waitAndPickClient()
				asset, redirectURL, err := g.getClient().Repositories.DownloadReleaseAsset(g.ctx, g.repoOwner, g.repoName, assetID, nil)
				if err != nil {
					return nil, err
				}
				if err := g.RefreshRate(); err != nil {
					log.Error("g.getClient().RateLimits: %s", err)
				}
				if asset == nil {
					if redirectURL != "" {
						g.waitAndPickClient()
						req, err := http.NewRequestWithContext(g.ctx, "GET", redirectURL, nil)
						if err != nil {
							return nil, err
						}
						resp, err := httpClient.Do(req)
						err1 := g.RefreshRate()
						if err1 != nil {
							log.Error("g.getClient().RateLimits: %s", err1)
						}
						if err != nil {
							return nil, err
						}
						return resp.Body, nil
					}
					return nil, fmt.Errorf("No release asset found for %d", assetID)
				}
				return asset, nil
			},
		})
	}
	return r
}

// GetReleases returns releases
func (g *GithubDownloaderV3) GetReleases() ([]*base.Release, error) {
	var perPage = g.maxPerPage
	var releases = make([]*base.Release, 0, perPage)
	for i := 1; ; i++ {
		g.waitAndPickClient()
		ls, resp, err := g.getClient().Repositories.ListReleases(g.ctx, g.repoOwner, g.repoName,
			&github.ListOptions{
				Page:    i,
				PerPage: perPage,
			})
		if err != nil {
			return nil, err
		}
		g.setRate(&resp.Rate)

		for _, release := range ls {
			releases = append(releases, g.convertGithubRelease(release))
		}
		if len(ls) < perPage {
			break
		}
	}
	return releases, nil
}

// GetIssues returns issues according start and limit
func (g *GithubDownloaderV3) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	opt := &github.IssueListByRepoOptions{
		Sort:      "created",
		Direction: "asc",
		State:     "all",
		ListOptions: github.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
	}

	var allIssues = make([]*base.Issue, 0, perPage)
	g.waitAndPickClient()
	issues, resp, err := g.getClient().Issues.ListByRepo(g.ctx, g.repoOwner, g.repoName, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %v", err)
	}
	log.Trace("Request get issues %d/%d, but in fact get %d", perPage, page, len(issues))
	g.setRate(&resp.Rate)
	for _, issue := range issues {
		if issue.IsPullRequest() {
			continue
		}

		var labels = make([]*base.Label, 0, len(issue.Labels))
		for _, l := range issue.Labels {
			labels = append(labels, convertGithubLabel(l))
		}

		// get reactions
		var reactions []*base.Reaction
		for i := 1; ; i++ {
			g.waitAndPickClient()
			res, resp, err := g.getClient().Reactions.ListIssueReactions(g.ctx, g.repoOwner, g.repoName, issue.GetNumber(), &github.ListOptions{
				Page:    i,
				PerPage: perPage,
			})
			if err != nil {
				return nil, false, err
			}
			g.setRate(&resp.Rate)
			if len(res) == 0 {
				break
			}
			for _, reaction := range res {
				reactions = append(reactions, &base.Reaction{
					UserID:   reaction.User.GetID(),
					UserName: reaction.User.GetLogin(),
					Content:  reaction.GetContent(),
				})
			}
		}

		var assignees []string
		for i := range issue.Assignees {
			assignees = append(assignees, issue.Assignees[i].GetLogin())
		}

		allIssues = append(allIssues, &base.Issue{
			Title:       *issue.Title,
			Number:      int64(*issue.Number),
			PosterID:    issue.GetUser().GetID(),
			PosterName:  issue.GetUser().GetLogin(),
			PosterEmail: issue.GetUser().GetEmail(),
			Content:     issue.GetBody(),
			Milestone:   issue.GetMilestone().GetTitle(),
			State:       issue.GetState(),
			Created:     issue.GetCreatedAt(),
			Updated:     issue.GetUpdatedAt(),
			Labels:      labels,
			Reactions:   reactions,
			Closed:      issue.ClosedAt,
			IsLocked:    issue.GetLocked(),
			Assignees:   assignees,
			Context:     base.BasicIssueContext(*issue.Number),
		})
	}

	return allIssues, len(issues) < perPage, nil
}

// SupportGetRepoComments return true if it supports get repo comments
func (g *GithubDownloaderV3) SupportGetRepoComments() bool {
	return true
}

// GetComments returns comments according issueNumber
func (g *GithubDownloaderV3) GetComments(opts base.GetCommentOptions) ([]*base.Comment, bool, error) {
	if opts.Context != nil {
		comments, err := g.getComments(opts.Context)
		return comments, false, err
	}

	return g.GetAllComments(opts.Page, opts.PageSize)
}

func (g *GithubDownloaderV3) getComments(issueContext base.IssueContext) ([]*base.Comment, error) {
	var (
		allComments = make([]*base.Comment, 0, g.maxPerPage)
		created     = "created"
		asc         = "asc"
	)
	opt := &github.IssueListCommentsOptions{
		Sort:      &created,
		Direction: &asc,
		ListOptions: github.ListOptions{
			PerPage: g.maxPerPage,
		},
	}
	for {
		g.waitAndPickClient()
		comments, resp, err := g.getClient().Issues.ListComments(g.ctx, g.repoOwner, g.repoName, int(issueContext.ForeignID()), opt)
		if err != nil {
			return nil, fmt.Errorf("error while listing repos: %v", err)
		}
		g.setRate(&resp.Rate)
		for _, comment := range comments {
			// get reactions
			var reactions []*base.Reaction
			for i := 1; ; i++ {
				g.waitAndPickClient()
				res, resp, err := g.getClient().Reactions.ListIssueCommentReactions(g.ctx, g.repoOwner, g.repoName, comment.GetID(), &github.ListOptions{
					Page:    i,
					PerPage: g.maxPerPage,
				})
				if err != nil {
					return nil, err
				}
				g.setRate(&resp.Rate)
				if len(res) == 0 {
					break
				}
				for _, reaction := range res {
					reactions = append(reactions, &base.Reaction{
						UserID:   reaction.User.GetID(),
						UserName: reaction.User.GetLogin(),
						Content:  reaction.GetContent(),
					})
				}
			}

			allComments = append(allComments, &base.Comment{
				IssueIndex:  issueContext.LocalID(),
				PosterID:    comment.GetUser().GetID(),
				PosterName:  comment.GetUser().GetLogin(),
				PosterEmail: comment.GetUser().GetEmail(),
				Content:     comment.GetBody(),
				Created:     comment.GetCreatedAt(),
				Updated:     comment.GetUpdatedAt(),
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

// GetAllComments returns repository comments according page and perPageSize
func (g *GithubDownloaderV3) GetAllComments(page, perPage int) ([]*base.Comment, bool, error) {
	var (
		allComments = make([]*base.Comment, 0, perPage)
		created     = "created"
		asc         = "asc"
	)
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	opt := &github.IssueListCommentsOptions{
		Sort:      &created,
		Direction: &asc,
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
	}

	g.waitAndPickClient()
	comments, resp, err := g.getClient().Issues.ListComments(g.ctx, g.repoOwner, g.repoName, 0, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %v", err)
	}
	var isEnd = resp.NextPage == 0

	log.Trace("Request get comments %d/%d, but in fact get %d, next page is %d", perPage, page, len(comments), resp.NextPage)
	g.setRate(&resp.Rate)
	for _, comment := range comments {
		// get reactions
		var reactions []*base.Reaction
		for i := 1; ; i++ {
			g.waitAndPickClient()
			res, resp, err := g.getClient().Reactions.ListIssueCommentReactions(g.ctx, g.repoOwner, g.repoName, comment.GetID(), &github.ListOptions{
				Page:    i,
				PerPage: g.maxPerPage,
			})
			if err != nil {
				return nil, false, err
			}
			g.setRate(&resp.Rate)
			if len(res) == 0 {
				break
			}
			for _, reaction := range res {
				reactions = append(reactions, &base.Reaction{
					UserID:   reaction.User.GetID(),
					UserName: reaction.User.GetLogin(),
					Content:  reaction.GetContent(),
				})
			}
		}
		idx := strings.LastIndex(*comment.IssueURL, "/")
		issueIndex, _ := strconv.ParseInt((*comment.IssueURL)[idx+1:], 10, 64)
		allComments = append(allComments, &base.Comment{
			IssueIndex:  issueIndex,
			PosterID:    comment.GetUser().GetID(),
			PosterName:  comment.GetUser().GetLogin(),
			PosterEmail: comment.GetUser().GetEmail(),
			Content:     comment.GetBody(),
			Created:     comment.GetCreatedAt(),
			Updated:     comment.GetUpdatedAt(),
			Reactions:   reactions,
		})
	}

	return allComments, isEnd, nil
}

// GetPullRequests returns pull requests according page and perPage
func (g *GithubDownloaderV3) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	opt := &github.PullRequestListOptions{
		Sort:      "created",
		Direction: "asc",
		State:     "all",
		ListOptions: github.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
	}
	var allPRs = make([]*base.PullRequest, 0, perPage)
	g.waitAndPickClient()
	prs, resp, err := g.getClient().PullRequests.List(g.ctx, g.repoOwner, g.repoName, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %v", err)
	}
	log.Trace("Request get pull requests %d/%d, but in fact get %d", perPage, page, len(prs))
	g.setRate(&resp.Rate)
	for _, pr := range prs {
		var labels = make([]*base.Label, 0, len(pr.Labels))
		for _, l := range pr.Labels {
			labels = append(labels, convertGithubLabel(l))
		}

		// get reactions
		var reactions []*base.Reaction
		for i := 1; ; i++ {
			g.waitAndPickClient()
			res, resp, err := g.getClient().Reactions.ListIssueReactions(g.ctx, g.repoOwner, g.repoName, pr.GetNumber(), &github.ListOptions{
				Page:    i,
				PerPage: perPage,
			})
			if err != nil {
				return nil, false, err
			}
			g.setRate(&resp.Rate)
			if len(res) == 0 {
				break
			}
			for _, reaction := range res {
				reactions = append(reactions, &base.Reaction{
					UserID:   reaction.User.GetID(),
					UserName: reaction.User.GetLogin(),
					Content:  reaction.GetContent(),
				})
			}
		}

		// download patch and saved as tmp file
		g.waitAndPickClient()

		allPRs = append(allPRs, &base.PullRequest{
			Title:          pr.GetTitle(),
			Number:         int64(pr.GetNumber()),
			PosterID:       pr.GetUser().GetID(),
			PosterName:     pr.GetUser().GetLogin(),
			PosterEmail:    pr.GetUser().GetEmail(),
			Content:        pr.GetBody(),
			Milestone:      pr.GetMilestone().GetTitle(),
			State:          pr.GetState(),
			Created:        pr.GetCreatedAt(),
			Updated:        pr.GetUpdatedAt(),
			Closed:         pr.ClosedAt,
			Labels:         labels,
			Merged:         pr.MergedAt != nil,
			MergeCommitSHA: pr.GetMergeCommitSHA(),
			MergedTime:     pr.MergedAt,
			IsLocked:       pr.ActiveLockReason != nil,
			Head: base.PullRequestBranch{
				Ref:       pr.GetHead().GetRef(),
				SHA:       pr.GetHead().GetSHA(),
				OwnerName: pr.GetHead().GetUser().GetLogin(),
				RepoName:  pr.GetHead().GetRepo().GetName(),
				CloneURL:  pr.GetHead().GetRepo().GetCloneURL(),
			},
			Base: base.PullRequestBranch{
				Ref:       pr.GetBase().GetRef(),
				SHA:       pr.GetBase().GetSHA(),
				RepoName:  pr.GetBase().GetRepo().GetName(),
				OwnerName: pr.GetBase().GetUser().GetLogin(),
			},
			PatchURL:  pr.GetPatchURL(),
			Reactions: reactions,
			Context:   base.BasicIssueContext(*pr.Number),
		})
	}

	return allPRs, len(prs) < perPage, nil
}

func convertGithubReview(r *github.PullRequestReview) *base.Review {
	return &base.Review{
		ID:           r.GetID(),
		ReviewerID:   r.GetUser().GetID(),
		ReviewerName: r.GetUser().GetLogin(),
		CommitID:     r.GetCommitID(),
		Content:      r.GetBody(),
		CreatedAt:    r.GetSubmittedAt(),
		State:        r.GetState(),
	}
}

func (g *GithubDownloaderV3) convertGithubReviewComments(cs []*github.PullRequestComment) ([]*base.ReviewComment, error) {
	var rcs = make([]*base.ReviewComment, 0, len(cs))
	for _, c := range cs {
		// get reactions
		var reactions []*base.Reaction
		for i := 1; ; i++ {
			g.waitAndPickClient()
			res, resp, err := g.getClient().Reactions.ListPullRequestCommentReactions(g.ctx, g.repoOwner, g.repoName, c.GetID(), &github.ListOptions{
				Page:    i,
				PerPage: g.maxPerPage,
			})
			if err != nil {
				return nil, err
			}
			g.setRate(&resp.Rate)
			if len(res) == 0 {
				break
			}
			for _, reaction := range res {
				reactions = append(reactions, &base.Reaction{
					UserID:   reaction.User.GetID(),
					UserName: reaction.User.GetLogin(),
					Content:  reaction.GetContent(),
				})
			}
		}

		rcs = append(rcs, &base.ReviewComment{
			ID:        c.GetID(),
			InReplyTo: c.GetInReplyTo(),
			Content:   c.GetBody(),
			TreePath:  c.GetPath(),
			DiffHunk:  c.GetDiffHunk(),
			Position:  c.GetPosition(),
			CommitID:  c.GetCommitID(),
			PosterID:  c.GetUser().GetID(),
			Reactions: reactions,
			CreatedAt: c.GetCreatedAt(),
			UpdatedAt: c.GetUpdatedAt(),
		})
	}
	return rcs, nil
}

// GetReviews returns pull requests review
func (g *GithubDownloaderV3) GetReviews(context base.IssueContext) ([]*base.Review, error) {
	var allReviews = make([]*base.Review, 0, g.maxPerPage)
	opt := &github.ListOptions{
		PerPage: g.maxPerPage,
	}
	for {
		g.waitAndPickClient()
		reviews, resp, err := g.getClient().PullRequests.ListReviews(g.ctx, g.repoOwner, g.repoName, int(context.ForeignID()), opt)
		if err != nil {
			return nil, fmt.Errorf("error while listing repos: %v", err)
		}
		g.setRate(&resp.Rate)
		for _, review := range reviews {
			r := convertGithubReview(review)
			r.IssueIndex = context.LocalID()
			// retrieve all review comments
			opt2 := &github.ListOptions{
				PerPage: g.maxPerPage,
			}
			for {
				g.waitAndPickClient()
				reviewComments, resp, err := g.getClient().PullRequests.ListReviewComments(g.ctx, g.repoOwner, g.repoName, int(context.ForeignID()), review.GetID(), opt2)
				if err != nil {
					return nil, fmt.Errorf("error while listing repos: %v", err)
				}
				g.setRate(&resp.Rate)

				cs, err := g.convertGithubReviewComments(reviewComments)
				if err != nil {
					return nil, err
				}
				r.Comments = append(r.Comments, cs...)
				if resp.NextPage == 0 {
					break
				}
				opt2.Page = resp.NextPage
			}
			allReviews = append(allReviews, r)
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allReviews, nil
}
