// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/proxy"
	"code.gitea.io/gitea/modules/structs"

	"github.com/google/go-github/v57/github"
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
type GithubDownloaderV3Factory struct{}

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

	log.Trace("Create github downloader BaseURL: %s %s/%s", baseURL, oldOwner, oldName)

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
	ctx           context.Context
	clients       []*github.Client
	baseURL       string
	repoOwner     string
	repoName      string
	userName      string
	password      string
	rates         []*github.Rate
	curClientIdx  int
	maxPerPage    int
	SkipReactions bool
	SkipReviews   bool
}

// NewGithubDownloaderV3 creates a github Downloader via github v3 API
func NewGithubDownloaderV3(ctx context.Context, baseURL, userName, password, token, repoOwner, repoName string) *GithubDownloaderV3 {
	downloader := GithubDownloaderV3{
		userName:   userName,
		baseURL:    baseURL,
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
			client := &http.Client{
				Transport: &oauth2.Transport{
					Base:   NewMigrationHTTPTransport(),
					Source: oauth2.ReuseTokenSource(nil, ts),
				},
			}

			downloader.addClient(client, baseURL)
		}
	} else {
		transport := NewMigrationHTTPTransport()
		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			req.SetBasicAuth(userName, password)
			return proxy.Proxy()(req)
		}
		client := &http.Client{
			Transport: transport,
		}
		downloader.addClient(client, baseURL)
	}
	return &downloader
}

func (g *GithubDownloaderV3) SupportSyncing() bool {
	return true
}

// String implements Stringer
func (g *GithubDownloaderV3) String() string {
	return fmt.Sprintf("migration from github server %s %s/%s", g.baseURL, g.repoOwner, g.repoName)
}

func (g *GithubDownloaderV3) LogString() string {
	if g == nil {
		return "<GithubDownloaderV3 nil>"
	}
	return fmt.Sprintf("<GithubDownloaderV3 %s %s/%s>", g.baseURL, g.repoOwner, g.repoName)
}

func (g *GithubDownloaderV3) addClient(client *http.Client, baseURL string) {
	githubClient := github.NewClient(client)
	if baseURL != "https://github.com" {
		githubClient, _ = github.NewClient(client).WithEnterpriseURLs(baseURL, baseURL)
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
			timer.Stop()
			return
		case <-timer.C:
		}

		err := g.RefreshRate()
		if err != nil {
			log.Error("g.getClient().RateLimit.Get: %s", err)
		}
	}
}

// RefreshRate update the current rate (doesn't count in rate limit)
func (g *GithubDownloaderV3) RefreshRate() error {
	rates, _, err := g.getClient().RateLimit.Get(g.ctx)
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
	perPage := g.maxPerPage
	milestones := make([]*base.Milestone, 0, perPage)
	for i := 1; ; i++ {
		g.waitAndPickClient()
		ms, resp, err := g.getClient().Issues.ListMilestones(g.ctx, g.repoOwner, g.repoName,
			&github.MilestoneListOptions{
				State: "all",
				ListOptions: github.ListOptions{
					Page:    i,
					PerPage: perPage,
				},
			})
		if err != nil {
			return nil, err
		}
		g.setRate(&resp.Rate)

		for _, m := range ms {
			state := "open"
			if m.State != nil {
				state = *m.State
			}
			milestones = append(milestones, &base.Milestone{
				Title:       m.GetTitle(),
				Description: m.GetDescription(),
				Deadline:    m.DueOn.GetTime(),
				State:       state,
				Created:     m.GetCreatedAt().Time,
				Updated:     m.UpdatedAt.GetTime(),
				Closed:      m.ClosedAt.GetTime(),
				OriginalID:  m.GetID(),
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
		OriginalID:  label.GetID(),
	}
}

// GetLabels returns labels
func (g *GithubDownloaderV3) GetLabels() ([]*base.Label, error) {
	perPage := g.maxPerPage
	labels := make([]*base.Label, 0, perPage)
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
	// GitHub allows commitish to be a reference.
	// In this case, we need to remove the prefix, i.e. convert "refs/heads/main" to "main".
	targetCommitish := strings.TrimPrefix(rel.GetTargetCommitish(), git.BranchPrefix)

	r := &base.Release{
		Name:            rel.GetName(),
		TagName:         rel.GetTagName(),
		TargetCommitish: targetCommitish,
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

	httpClient := NewMigrationHTTPClient()

	for _, asset := range rel.Assets {
		assetID := *asset.ID // Don't optimize this, for closure we need a local variable
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
				readCloser, redirectURL, err := g.getClient().Repositories.DownloadReleaseAsset(g.ctx, g.repoOwner, g.repoName, assetID, nil)
				if err != nil {
					return nil, err
				}
				if err := g.RefreshRate(); err != nil {
					log.Error("g.getClient().RateLimits: %s", err)
				}

				if readCloser != nil {
					return readCloser, nil
				}

				if redirectURL == "" {
					return nil, fmt.Errorf("no release asset found for %d", assetID)
				}

				// Prevent open redirect
				if !hasBaseURL(redirectURL, g.baseURL) &&
					!hasBaseURL(redirectURL, "https://objects.githubusercontent.com/") {
					WarnAndNotice("Unexpected AssetURL for assetID[%d] in %s: %s", asset.GetID(), g, redirectURL)

					return io.NopCloser(strings.NewReader(redirectURL)), nil
				}

				g.waitAndPickClient()
				req, err := http.NewRequestWithContext(g.ctx, "GET", redirectURL, nil)
				if err != nil {
					return nil, err
				}
				resp, err := httpClient.Do(req)
				err1 := g.RefreshRate()
				if err1 != nil {
					log.Error("g.RefreshRate(): %s", err1)
				}
				if err != nil {
					return nil, err
				}
				return resp.Body, nil
			},
		})
	}
	return r
}

// GetReleases returns releases
func (g *GithubDownloaderV3) GetReleases() ([]*base.Release, error) {
	perPage := g.maxPerPage
	releases := make([]*base.Release, 0, perPage)
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
	return g.getIssuesSince(page, perPage, time.Time{}) // set since to empty to get all issues
}

// SupportGetRepoComments return true if it supports get repo comments
func (g *GithubDownloaderV3) SupportGetRepoComments() bool {
	return true
}

// GetComments returns comments according issueNumber
func (g *GithubDownloaderV3) GetComments(commentable base.Commentable) ([]*base.Comment, bool, error) {
	comments, err := g.getCommentsSince(commentable, nil)
	return comments, false, err
}

func (g *GithubDownloaderV3) getCommentsSince(commentable base.Commentable, since *time.Time) ([]*base.Comment, error) {
	var (
		allComments = make([]*base.Comment, 0, g.maxPerPage)
		created     = "created"
		asc         = "asc"
	)
	opt := &github.IssueListCommentsOptions{
		Sort:      &created,
		Direction: &asc,
		Since:     since,
		ListOptions: github.ListOptions{
			PerPage: g.maxPerPage,
		},
	}
	for {
		g.waitAndPickClient()
		comments, resp, err := g.getClient().Issues.ListComments(g.ctx, g.repoOwner, g.repoName, int(commentable.GetForeignIndex()), opt)
		if err != nil {
			return nil, fmt.Errorf("error while listing repos: %w", err)
		}
		g.setRate(&resp.Rate)
		for _, comment := range comments {
			// get reactions
			var reactions []*base.Reaction
			if !g.SkipReactions {
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
			}

			allComments = append(allComments, &base.Comment{
				IssueIndex:  commentable.GetLocalIndex(),
				Index:       comment.GetID(),
				PosterID:    comment.GetUser().GetID(),
				PosterName:  comment.GetUser().GetLogin(),
				PosterEmail: comment.GetUser().GetEmail(),
				Content:     comment.GetBody(),
				Created:     comment.GetCreatedAt().Time,
				Updated:     comment.GetUpdatedAt().Time,
				Reactions:   reactions,
				OriginalID:  comment.GetID(),
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
	return g.getAllCommentsSince(page, perPage, nil)
}

// GetAllCommentsSince returns repository comments since a time.
// If since is nil, it will return all comments.
func (g *GithubDownloaderV3) getAllCommentsSince(page, perPage int, since *time.Time) ([]*base.Comment, bool, error) {
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
		Since:     since,
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
	}

	g.waitAndPickClient()
	comments, resp, err := g.getClient().Issues.ListComments(g.ctx, g.repoOwner, g.repoName, 0, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %w", err)
	}
	isEnd := resp.NextPage == 0

	log.Trace("Request get comments %d/%d, but in fact get %d, next page is %d", perPage, page, len(comments), resp.NextPage)
	g.setRate(&resp.Rate)
	for _, comment := range comments {
		// get reactions
		var reactions []*base.Reaction
		if !g.SkipReactions {
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
		}
		idx := strings.LastIndex(*comment.IssueURL, "/")
		issueIndex, _ := strconv.ParseInt((*comment.IssueURL)[idx+1:], 10, 64)
		allComments = append(allComments, &base.Comment{
			IssueIndex:  issueIndex,
			Index:       comment.GetID(),
			PosterID:    comment.GetUser().GetID(),
			PosterName:  comment.GetUser().GetLogin(),
			PosterEmail: comment.GetUser().GetEmail(),
			Content:     comment.GetBody(),
			Created:     comment.GetCreatedAt().Time,
			Updated:     comment.GetUpdatedAt().Time,
			Reactions:   reactions,
			OriginalID:  comment.GetID(),
		})
	}

	return allComments, isEnd, nil
}

// GetPullRequests returns pull requests according page and perPage
func (g *GithubDownloaderV3) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	return g.GetNewPullRequests(page, perPage, time.Time{})
}

// convertGithubReview converts github review to Gitea review
func (g *GithubDownloaderV3) convertGithubReview(r *github.PullRequestReview) *base.Review {
	return &base.Review{
		ID:           r.GetID(),
		ReviewerID:   r.GetUser().GetID(),
		ReviewerName: r.GetUser().GetLogin(),
		CommitID:     r.GetCommitID(),
		Content:      r.GetBody(),
		CreatedAt:    r.GetSubmittedAt().Time,
		State:        r.GetState(),
		OriginalID:   r.GetID(),
	}
}

func (g *GithubDownloaderV3) convertGithubReviewComments(cs []*github.PullRequestComment) ([]*base.ReviewComment, error) {
	rcs := make([]*base.ReviewComment, 0, len(cs))
	for _, c := range cs {
		// get reactions
		var reactions []*base.Reaction
		if !g.SkipReactions {
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
		}

		rcs = append(rcs, &base.ReviewComment{
			ID:         c.GetID(),
			InReplyTo:  c.GetInReplyTo(),
			Content:    c.GetBody(),
			TreePath:   c.GetPath(),
			DiffHunk:   c.GetDiffHunk(),
			Position:   c.GetPosition(),
			CommitID:   c.GetCommitID(),
			PosterID:   c.GetUser().GetID(),
			Reactions:  reactions,
			CreatedAt:  c.GetCreatedAt().Time,
			UpdatedAt:  c.GetUpdatedAt().Time,
			OriginalID: c.GetID(),
		})
	}
	return rcs, nil
}

// GetReviews returns pull requests review
func (g *GithubDownloaderV3) GetReviews(reviewable base.Reviewable) ([]*base.Review, error) {
	allReviews := make([]*base.Review, 0, g.maxPerPage)
	if g.SkipReviews {
		return allReviews, nil
	}
	opt := &github.ListOptions{
		PerPage: g.maxPerPage,
	}
	// Get approve/request change reviews
	for {
		g.waitAndPickClient()
		reviews, resp, err := g.getClient().PullRequests.ListReviews(g.ctx, g.repoOwner, g.repoName, int(reviewable.GetForeignIndex()), opt)
		if err != nil {
			return nil, fmt.Errorf("error while listing repos: %w", err)
		}
		g.setRate(&resp.Rate)
		for _, review := range reviews {
			r := g.convertGithubReview(review)
			r.IssueIndex = reviewable.GetLocalIndex()
			// retrieve all review comments
			opt2 := &github.ListOptions{
				PerPage: g.maxPerPage,
			}
			for {
				g.waitAndPickClient()
				reviewComments, resp, err := g.getClient().PullRequests.ListReviewComments(g.ctx, g.repoOwner, g.repoName, int(reviewable.GetForeignIndex()), review.GetID(), opt2)
				if err != nil {
					return nil, fmt.Errorf("error while listing repos: %w", err)
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
	// Get requested reviews
	for {
		g.waitAndPickClient()
		reviewers, resp, err := g.getClient().PullRequests.ListReviewers(g.ctx, g.repoOwner, g.repoName, int(reviewable.GetForeignIndex()), opt)
		if err != nil {
			return nil, fmt.Errorf("error while listing repos: %w", err)
		}
		g.setRate(&resp.Rate)
		for _, user := range reviewers.Users {
			r := &base.Review{
				ReviewerID:   user.GetID(),
				ReviewerName: user.GetLogin(),
				State:        base.ReviewStateRequestReview,
				IssueIndex:   reviewable.GetLocalIndex(),
			}
			allReviews = append(allReviews, r)
		}
		// TODO: Handle Team requests
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allReviews, nil
}

// GetNewIssues returns new issues updated after the given time according start and limit
func (g *GithubDownloaderV3) GetNewIssues(page, perPage int, updatedAfter time.Time) ([]*base.Issue, bool, error) {
	return g.getIssuesSince(page, perPage, updatedAfter)
}

// getIssuesSince returns issues given page, perPage and since.
// when since is empty, it will return all issues
func (g *GithubDownloaderV3) getIssuesSince(page, perPage int, since time.Time) ([]*base.Issue, bool, error) {
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	opt := &github.IssueListByRepoOptions{
		Sort:      "created",
		Direction: "asc",
		State:     "all",
		Since:     since,
		ListOptions: github.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
	}

	allIssues := make([]*base.Issue, 0, perPage)
	g.waitAndPickClient()
	issues, resp, err := g.getClient().Issues.ListByRepo(g.ctx, g.repoOwner, g.repoName, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %w", err)
	}
	log.Trace("Request get issues %d/%d, but in fact get %d", perPage, page, len(issues))
	g.setRate(&resp.Rate)
	for _, issue := range issues {
		if issue.IsPullRequest() {
			continue
		}

		labels := make([]*base.Label, 0, len(issue.Labels))
		for _, l := range issue.Labels {
			labels = append(labels, convertGithubLabel(l))
		}

		// get reactions
		reactions, err := g.getIssueReactions(issue.GetNumber(), perPage)
		if err != nil {
			return nil, false, err
		}

		var assignees []string
		for i := range issue.Assignees {
			assignees = append(assignees, issue.Assignees[i].GetLogin())
		}

		allIssues = append(allIssues, &base.Issue{
			Title:        *issue.Title,
			Number:       int64(*issue.Number),
			PosterID:     issue.GetUser().GetID(),
			PosterName:   issue.GetUser().GetLogin(),
			PosterEmail:  issue.GetUser().GetEmail(),
			Content:      issue.GetBody(),
			Milestone:    issue.GetMilestone().GetTitle(),
			State:        issue.GetState(),
			Created:      issue.GetCreatedAt().Time,
			Updated:      issue.GetUpdatedAt().Time,
			Labels:       labels,
			Reactions:    reactions,
			Closed:       issue.ClosedAt.GetTime(),
			IsLocked:     issue.GetLocked(),
			Assignees:    assignees,
			ForeignIndex: int64(*issue.Number),
		})
	}

	return allIssues, len(issues) < perPage, nil
}

// GetNewComments returns comments of an issue or PR after the given time
func (g GithubDownloaderV3) GetNewComments(commentable base.Commentable, updatedAfter time.Time) ([]*base.Comment, bool, error) {
	comments, err := g.getCommentsSince(commentable, &updatedAfter)
	return comments, false, err
}

// GetAllNewComments returns paginated comments after the given time
func (g GithubDownloaderV3) GetAllNewComments(page, perPage int, updatedAfter time.Time) ([]*base.Comment, bool, error) {
	return g.getAllCommentsSince(page, perPage, &updatedAfter)
}

// GetNewPullRequests returns pull requests after the given time according page and perPage
// If `updatedAfter` is zero-valued, it will return all pull requests
func (g *GithubDownloaderV3) GetNewPullRequests(page, perPage int, updatedAfter time.Time) ([]*base.PullRequest, bool, error) {
	// Pulls API doesn't have parameter `since`, so we have to use Search API instead.
	// By specifying `repo:owner/repo is:pr` in the query, we can get all pull requests of the repository.
	// In addition, we can specify `updated:>=YYYY-MM-DDTHH:MM:SS+00:00` to get pull requests updated after the given time.

	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	opt := &github.SearchOptions{
		Sort:  "created",
		Order: "asc",
		ListOptions: github.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
	}

	allPRs := make([]*base.PullRequest, 0, perPage)
	g.waitAndPickClient()

	searchQuery := fmt.Sprintf("repo:%s/%s is:pr", g.repoOwner, g.repoName)
	if !updatedAfter.IsZero() {
		// GitHub requires time to be later than 1970-01-01, so we should skip `updated` part if it's zero.
		// Timezone is denoted by plus/minus UTC offset, rather than 'Z',
		// according to https://docs.github.com/en/search-github/searching-on-github/searching-issues-and-pull-requests#search-by-when-an-issue-or-pull-request-was-created-or-last-updated
		timeStr := updatedAfter.Format("2006-01-02T15:04:05-07:00")
		searchQuery += fmt.Sprintf(" updated:>=%s", timeStr)
	}

	result, resp, err := g.getClient().Search.Issues(g.ctx, searchQuery, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %v", err)
	}
	log.Trace("Request get issues %d/%d, but in fact get %d", perPage, page, len(result.Issues))
	g.setRate(&resp.Rate)
	for _, issue := range result.Issues {
		pr, resp, err := g.getClient().PullRequests.Get(g.ctx, g.repoOwner, g.repoName, issue.GetNumber())
		if err != nil {
			return nil, false, fmt.Errorf("error while getting repo pull request: %v", err)
		}
		g.setRate(&resp.Rate)
		basePR, err := g.convertGithubPullRequest(pr, perPage)
		if err != nil {
			return nil, false, err
		}
		allPRs = append(allPRs, basePR)

		// SECURITY: Ensure that the PR is safe
		_ = CheckAndEnsureSafePR(allPRs[len(allPRs)-1], g.baseURL, g)
	}

	return allPRs, len(result.Issues) < perPage, nil
}

// GetNewReviews returns new pull requests review after the given time
func (g GithubDownloaderV3) GetNewReviews(reviewable base.Reviewable, updatedAfter time.Time) ([]*base.Review, error) {
	// Github does not support since parameter for reviews, so we need to get all reviews
	return g.GetReviews(reviewable)
}

func (g *GithubDownloaderV3) convertGithubPullRequest(pr *github.PullRequest, perPage int) (*base.PullRequest, error) {
	labels := make([]*base.Label, 0, len(pr.Labels))
	for _, l := range pr.Labels {
		labels = append(labels, convertGithubLabel(l))
	}

	// get reactions
	reactions, err := g.getIssueReactions(pr.GetNumber(), perPage)
	if err != nil {
		return nil, err
	}

	// download patch and saved as tmp file
	g.waitAndPickClient()

	return &base.PullRequest{
		Title:          pr.GetTitle(),
		Number:         int64(pr.GetNumber()),
		PosterID:       pr.GetUser().GetID(),
		PosterName:     pr.GetUser().GetLogin(),
		PosterEmail:    pr.GetUser().GetEmail(),
		Content:        pr.GetBody(),
		Milestone:      pr.GetMilestone().GetTitle(),
		State:          pr.GetState(),
		Created:        pr.GetCreatedAt().Time,
		Updated:        pr.GetUpdatedAt().Time,
		Closed:         pr.ClosedAt.GetTime(),
		Labels:         labels,
		Merged:         pr.MergedAt != nil,
		MergeCommitSHA: pr.GetMergeCommitSHA(),
		MergedTime:     pr.MergedAt.GetTime(),
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
		PatchURL:     pr.GetPatchURL(),
		Reactions:    reactions,
		ForeignIndex: int64(*pr.Number),
	}, nil
}

// getIssueReactions returns reactions using Github API
func (g *GithubDownloaderV3) getIssueReactions(number, perPage int) ([]*base.Reaction, error) {
	var reactions []*base.Reaction
	if !g.SkipReactions {
		for i := 1; ; i++ {
			g.waitAndPickClient()
			res, resp, err := g.getClient().Reactions.ListIssueReactions(g.ctx, g.repoOwner, g.repoName, number, &github.ListOptions{
				Page:    i,
				PerPage: perPage,
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
	}
	return reactions, nil
}
