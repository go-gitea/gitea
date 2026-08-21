// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitea.dev/modules/git"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/proxy"
	"gitea.dev/modules/structs"

	"github.com/google/go-github/v89/github"
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

	return NewGithubDownloaderV3(ctx, baseURL, opts.AuthUsername, opts.AuthPassword, opts.AuthToken, oldOwner, oldName)
}

// GitServiceType returns the type of git service
func (f *GithubDownloaderV3Factory) GitServiceType() structs.GitServiceType {
	return structs.GithubService
}

// GithubDownloaderV3 implements a Downloader interface to get repository information
// from github via APIv3
type GithubDownloaderV3 struct {
	base.NullDownloader
	clients             []*github.Client
	baseURL             string
	repoOwner           string
	repoName            string
	userName            string
	password            string
	rates               []*github.Rate
	curClientIdx        int
	maxPerPage          int
	SkipReactions       bool
	SkipReviews         bool
	pullRequestMetadata map[int64]githubPullRequestMetadata
	warnedPRMetadata    bool
}

type githubPullRequestMetadata struct {
	closedBy    *base.ExternalUser
	closeReason string
}

type githubPullRequestContext struct {
	requestedReviewers []*base.ExternalUser
}

func githubExternalUser(user *github.User) *base.ExternalUser {
	if user == nil {
		return nil
	}
	return &base.ExternalUser{ID: user.GetID(), Name: user.GetLogin()}
}

func githubExternalUsers(users []*github.User) []*base.ExternalUser {
	result := make([]*base.ExternalUser, 0, len(users))
	for _, user := range users {
		if externalUser := githubExternalUser(user); externalUser != nil {
			result = append(result, externalUser)
		}
	}
	return result
}

// NewGithubDownloaderV3 creates a github Downloader via github v3 API
func NewGithubDownloaderV3(_ context.Context, baseURL, userName, password, token, repoOwner, repoName string) (*GithubDownloaderV3, error) {
	downloader := GithubDownloaderV3{
		userName:            userName,
		baseURL:             baseURL,
		password:            password,
		repoOwner:           repoOwner,
		repoName:            repoName,
		maxPerPage:          100,
		pullRequestMetadata: make(map[int64]githubPullRequestMetadata),
	}

	if token != "" {
		tokens := strings.SplitSeq(token, ",")
		for token := range tokens {
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

			if err := downloader.addClient(client, baseURL); err != nil {
				return nil, err
			}
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
		if err := downloader.addClient(client, baseURL); err != nil {
			return nil, err
		}
	}
	return &downloader, nil
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

func (g *GithubDownloaderV3) addClient(client *http.Client, baseURL string) error {
	opts := []github.ClientOptionsFunc{github.WithHTTPClient(client)}
	if baseURL != "https://github.com" {
		opts = append(opts, github.WithEnterpriseURLs(baseURL, baseURL))
	}
	githubClient, err := github.NewClient(opts...)
	if err != nil {
		return err
	}
	g.clients = append(g.clients, githubClient)
	g.rates = append(g.rates, nil)
	return nil
}

func (g *GithubDownloaderV3) waitAndPickClient(ctx context.Context) {
	var recentIdx int
	var maxRemaining int
	for i := 0; i < len(g.clients); i++ {
		if g.rates[i] == nil { // probe unknown clients once, else their rate never gets learned
			g.curClientIdx = i
			return
		}
		if g.rates[i].Remaining > maxRemaining {
			maxRemaining = g.rates[i].Remaining
			recentIdx = i
		}
	}
	g.curClientIdx = recentIdx

	for g.rates[g.curClientIdx] != nil && g.rates[g.curClientIdx].Remaining <= GithubLimitRateRemaining {
		timer := time.NewTimer(time.Until(g.rates[g.curClientIdx].Reset.Time))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		err := g.RefreshRate(ctx)
		if err != nil {
			log.Error("g.getClient().RateLimit.Get: %s", err)
		}
	}
}

// RefreshRate update the current rate (doesn't count in rate limit)
func (g *GithubDownloaderV3) RefreshRate(ctx context.Context) error {
	rates, _, err := g.getClient().RateLimit.Get(ctx)
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
func (g *GithubDownloaderV3) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	g.waitAndPickClient(ctx)
	gr, resp, err := g.getClient().Repositories.Get(ctx, g.repoOwner, g.repoName)
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
		Website:       gr.GetHomepage(),
		OriginalURL:   gr.GetHTMLURL(),
		CloneURL:      gr.GetCloneURL(),
		DefaultBranch: gr.GetDefaultBranch(),
	}, nil
}

// GetTopics return github topics
func (g *GithubDownloaderV3) GetTopics(ctx context.Context) ([]string, error) {
	g.waitAndPickClient(ctx)
	r, resp, err := g.getClient().Repositories.Get(ctx, g.repoOwner, g.repoName)
	if err != nil {
		return nil, err
	}
	g.setRate(&resp.Rate)
	return r.Topics, nil
}

// GetMilestones returns milestones
func (g *GithubDownloaderV3) GetMilestones(ctx context.Context) ([]*base.Milestone, error) {
	perPage := g.maxPerPage
	milestones := make([]*base.Milestone, 0, perPage)
	for i := 1; ; i++ {
		g.waitAndPickClient(ctx)
		ms, resp, err := g.getClient().Issues.ListMilestones(ctx, g.repoOwner, g.repoName,
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
func (g *GithubDownloaderV3) GetLabels(ctx context.Context) ([]*base.Label, error) {
	perPage := g.maxPerPage
	labels := make([]*base.Label, 0, perPage)
	for i := 1; ; i++ {
		g.waitAndPickClient(ctx)
		ls, resp, err := g.getClient().Issues.ListLabels(ctx, g.repoOwner, g.repoName,
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

func (g *GithubDownloaderV3) convertGithubRelease(ctx context.Context, rel *github.RepositoryRelease) *base.Release {
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

	httpClient := newMigrationHTTPClient()

	for _, asset := range rel.Assets {
		assetID := asset.GetID() // Don't optimize this, for closure we need a local variable TODO: no need to do so in new Golang
		if assetID == 0 {
			continue
		}
		r.Assets = append(r.Assets, &base.ReleaseAsset{
			ID:            asset.GetID(),
			Name:          asset.GetName(),
			Size:          asset.Size,
			DownloadCount: asset.DownloadCount,
			Created:       asset.CreatedAt.Time,
			Updated:       asset.UpdatedAt.Time,
			DownloadFunc: func() (io.ReadCloser, error) {
				g.waitAndPickClient(ctx)
				readCloser, redirectURL, err := g.getClient().Repositories.DownloadReleaseAsset(ctx, g.repoOwner, g.repoName, assetID, nil)
				if err != nil {
					return nil, err
				}
				if err := g.RefreshRate(ctx); err != nil {
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
					!hasBaseURL(redirectURL, "https://objects.githubusercontent.com/") &&
					!hasBaseURL(redirectURL, "https://release-assets.githubusercontent.com/") {
					WarnAndNotice("Unexpected AssetURL for assetID[%d] in %s: %s", asset.GetID(), g, redirectURL)

					return io.NopCloser(strings.NewReader(redirectURL)), nil
				}

				g.waitAndPickClient(ctx)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, redirectURL, nil)
				if err != nil {
					return nil, err
				}
				resp, err := httpClient.Do(req)
				err1 := g.RefreshRate(ctx)
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
func (g *GithubDownloaderV3) GetReleases(ctx context.Context) ([]*base.Release, error) {
	perPage := g.maxPerPage
	releases := make([]*base.Release, 0, perPage)
	for i := 1; ; i++ {
		g.waitAndPickClient(ctx)
		ls, resp, err := g.getClient().Repositories.ListReleases(ctx, g.repoOwner, g.repoName,
			&github.ListOptions{
				Page:    i,
				PerPage: perPage,
			})
		if err != nil {
			return nil, err
		}
		g.setRate(&resp.Rate)

		for _, release := range ls {
			releases = append(releases, g.convertGithubRelease(ctx, release))
		}
		if len(ls) < perPage {
			break
		}
	}
	return releases, nil
}

// GetIssues returns issues according start and limit
func (g *GithubDownloaderV3) GetIssues(ctx context.Context, page, perPage int) ([]*base.Issue, bool, error) {
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

	allIssues := make([]*base.Issue, 0, perPage)
	g.waitAndPickClient(ctx)
	issues, resp, err := g.getClient().Issues.ListByRepo(ctx, g.repoOwner, g.repoName, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %w", err)
	}
	log.Trace("Request get issues %d/%d, but in fact get %d", perPage, page, len(issues))
	g.setRate(&resp.Rate)
	for _, issue := range issues {
		if issue.IsPullRequest() {
			g.pullRequestMetadata[int64(issue.GetNumber())] = githubPullRequestMetadata{
				closedBy:    githubExternalUser(issue.GetClosedBy()),
				closeReason: issue.GetStateReason(),
			}
			continue
		}

		labels := make([]*base.Label, 0, len(issue.Labels))
		for _, l := range issue.Labels {
			labels = append(labels, convertGithubLabel(l))
		}

		// get reactions
		var reactions []*base.Reaction
		if !g.SkipReactions {
			for i := 1; ; i++ {
				g.waitAndPickClient(ctx)
				res, resp, err := g.getClient().Reactions.ListIssueReactions(ctx, g.repoOwner, g.repoName, issue.GetNumber(), &github.ListReactionOptions{
					ListOptions: github.ListOptions{
						Page:    i,
						PerPage: perPage,
					},
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
						Created:  reaction.GetCreatedAt().Time,
					})
				}
			}
		}

		var assignees []string
		for i := range issue.Assignees {
			assignees = append(assignees, issue.Assignees[i].GetLogin())
		}

		allIssues = append(allIssues, &base.Issue{
			Title:         *issue.Title,
			Number:        int64(*issue.Number),
			PosterID:      issue.GetUser().GetID(),
			PosterName:    issue.GetUser().GetLogin(),
			PosterEmail:   issue.GetUser().GetEmail(),
			Content:       issue.GetBody(),
			Milestone:     issue.GetMilestone().GetTitle(),
			State:         issue.GetState(),
			Created:       issue.GetCreatedAt().Time,
			Updated:       issue.GetUpdatedAt().Time,
			Labels:        labels,
			Reactions:     reactions,
			Closed:        issue.ClosedAt.GetTime(),
			ClosedBy:      githubExternalUser(issue.GetClosedBy()),
			CloseReason:   issue.GetStateReason(),
			IsLocked:      issue.GetLocked(),
			Assignees:     assignees,
			AssigneeUsers: githubExternalUsers(issue.Assignees),
			ForeignIndex:  int64(*issue.Number),
		})
	}

	return allIssues, len(issues) < perPage, nil
}

// SupportGetRepoComments return true if it supports get repo comments
func (g *GithubDownloaderV3) SupportGetRepoComments() bool {
	return true
}

// GetComments returns comments according issueNumber
func (g *GithubDownloaderV3) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	comments, err := g.getComments(ctx, commentable)
	return comments, false, err
}

func (g *GithubDownloaderV3) getComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, error) {
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
		g.waitAndPickClient(ctx)
		comments, resp, err := g.getClient().Issues.ListComments(ctx, g.repoOwner, g.repoName, int(commentable.GetForeignIndex()), opt)
		if err != nil {
			return nil, fmt.Errorf("error while listing repos: %w", err)
		}
		g.setRate(&resp.Rate)
		for _, comment := range comments {
			// get reactions
			var reactions []*base.Reaction
			if !g.SkipReactions {
				for i := 1; ; i++ {
					g.waitAndPickClient(ctx)
					res, resp, err := g.getClient().Reactions.ListIssueCommentReactions(ctx, g.repoOwner, g.repoName, comment.GetID(), &github.ListReactionOptions{
						ListOptions: github.ListOptions{
							Page:    i,
							PerPage: g.maxPerPage,
						},
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
							Created:  reaction.GetCreatedAt().Time,
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
func (g *GithubDownloaderV3) GetAllComments(ctx context.Context, page, perPage int) ([]*base.Comment, bool, error) {
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

	g.waitAndPickClient(ctx)
	comments, resp, err := g.getClient().Issues.ListComments(ctx, g.repoOwner, g.repoName, 0, opt)
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
				g.waitAndPickClient(ctx)
				res, resp, err := g.getClient().Reactions.ListIssueCommentReactions(ctx, g.repoOwner, g.repoName, comment.GetID(), &github.ListReactionOptions{
					ListOptions: github.ListOptions{
						Page:    i,
						PerPage: g.maxPerPage,
					},
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
						Created:  reaction.GetCreatedAt().Time,
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
		})
	}

	return allComments, isEnd, nil
}

// GetPullRequests returns pull requests according page and perPage
func (g *GithubDownloaderV3) GetPullRequests(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
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
	allPRs := make([]*base.PullRequest, 0, perPage)
	g.waitAndPickClient(ctx)
	prs, resp, err := g.getClient().PullRequests.List(ctx, g.repoOwner, g.repoName, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %w", err)
	}
	log.Trace("Request get pull requests %d/%d, but in fact get %d", perPage, page, len(prs))
	g.setRate(&resp.Rate)
	for _, pr := range prs {
		metadata, hasMetadata := g.pullRequestMetadata[int64(pr.GetNumber())]
		if !hasMetadata && !g.warnedPRMetadata && (pr.GetState() == "closed" || pr.MergedAt != nil) {
			WarnAndNotice("GitHub issue metadata was not fetched; close/merge actors and close reasons will be omitted from migrated pull requests. Migrate issues alongside pull requests to preserve them.")
			g.warnedPRMetadata = true
		}
		labels := make([]*base.Label, 0, len(pr.Labels))
		for _, l := range pr.Labels {
			labels = append(labels, convertGithubLabel(l))
		}

		// get reactions
		var reactions []*base.Reaction
		if !g.SkipReactions {
			for i := 1; ; i++ {
				g.waitAndPickClient(ctx)
				res, resp, err := g.getClient().Reactions.ListIssueReactions(ctx, g.repoOwner, g.repoName, pr.GetNumber(), &github.ListReactionOptions{
					ListOptions: github.ListOptions{
						Page:    i,
						PerPage: perPage,
					},
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
						Created:  reaction.GetCreatedAt().Time,
					})
				}
			}
		}

		// download patch and saved as tmp file
		g.waitAndPickClient(ctx)

		assignees := make([]string, 0, len(pr.Assignees))
		for _, assignee := range pr.Assignees {
			assignees = append(assignees, assignee.GetLogin())
		}

		converted := &base.PullRequest{
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
			ClosedBy:       metadata.closedBy,
			CloseReason:    metadata.closeReason,
			Labels:         labels,
			Merged:         pr.MergedAt != nil,
			MergeCommitSHA: pr.GetMergeCommitSHA(),
			MergedTime:     pr.MergedAt.GetTime(),
			IsLocked:       pr.GetLocked(),
			Head: base.PullRequestBranch{
				Ref:       pr.GetHead().GetRef(),
				SHA:       pr.GetHead().GetSHA(),
				OwnerName: pr.GetHead().GetUser().GetLogin(),
				RepoName:  pr.GetHead().GetRepo().GetName(),
				CloneURL:  pr.GetHead().GetRepo().GetCloneURL(), // see below for SECURITY related issues here
			},
			Base: base.PullRequestBranch{
				Ref:       pr.GetBase().GetRef(),
				SHA:       pr.GetBase().GetSHA(),
				RepoName:  pr.GetBase().GetRepo().GetName(),
				OwnerName: pr.GetBase().GetUser().GetLogin(),
			},
			PatchURL:      pr.GetPatchURL(), // see below for SECURITY related issues here
			Reactions:     reactions,
			Assignees:     assignees,
			AssigneeUsers: githubExternalUsers(pr.Assignees),
			ForeignIndex:  int64(*pr.Number),
			Context:       githubPullRequestContext{requestedReviewers: githubExternalUsers(pr.RequestedReviewers)},
			IsDraft:       pr.GetDraft(),
		}
		if converted.Merged {
			converted.MergedBy = metadata.closedBy
		}
		allPRs = append(allPRs, converted)

		// SECURITY: Ensure that the PR is safe
		_ = CheckAndEnsureSafePR(allPRs[len(allPRs)-1], g.baseURL, g)
	}

	return allPRs, len(prs) < perPage, nil
}

func convertGithubReview(r *github.PullRequestReview) *base.Review {
	state := r.GetState()
	if state == "DISMISSED" {
		// REST does not retain whether a dismissed review approved or rejected.
		state = base.ReviewStateCommented
	}
	return &base.Review{
		ID:           r.GetID(),
		ReviewerID:   r.GetUser().GetID(),
		ReviewerName: r.GetUser().GetLogin(),
		CommitID:     r.GetCommitID(),
		Content:      r.GetBody(),
		CreatedAt:    r.GetSubmittedAt().Time,
		State:        state,
	}
}

type githubReviewComment struct {
	reviewID       int64
	reviewCommitID string
	comment        *base.ReviewComment
}

func (g *GithubDownloaderV3) convertGithubReviewComments(ctx context.Context, cs []*github.PullRequestComment) ([]githubReviewComment, error) {
	rcs := make([]githubReviewComment, 0, len(cs))
	for _, c := range cs {
		var reactions []*base.Reaction
		if !g.SkipReactions {
			for i := 1; ; i++ {
				g.waitAndPickClient(ctx)
				res, resp, err := g.getClient().Reactions.ListPullRequestCommentReactions(ctx, g.repoOwner, g.repoName, c.GetID(), &github.ListReactionOptions{
					ListOptions: github.ListOptions{
						Page:    i,
						PerPage: g.maxPerPage,
					},
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
						Created:  reaction.GetCreatedAt().Time,
					})
				}
			}
		}

		line := int64(c.GetLine())
		if c.GetSide() == "LEFT" {
			line = -line
		}

		rcs = append(rcs, githubReviewComment{
			reviewID:       c.GetPullRequestReviewID(),
			reviewCommitID: c.GetCommitID(),
			comment: &base.ReviewComment{
				ID:         c.GetID(),
				InReplyTo:  c.GetInReplyTo(),
				Content:    c.GetBody(),
				TreePath:   c.GetPath(),
				DiffHunk:   c.GetDiffHunk(),
				Position:   c.GetPosition(),
				Line:       line,
				CommitID:   c.GetCommitID(),
				PosterID:   c.GetUser().GetID(),
				PosterName: c.GetUser().GetLogin(),
				Reactions:  reactions,
				CreatedAt:  c.GetCreatedAt().Time,
				UpdatedAt:  c.GetUpdatedAt().Time,
			},
		})
	}
	return rcs, nil
}

func groupGithubReviewComments(reviewsByID map[int64]*base.Review, records []githubReviewComment, issueIndex int64) ([]*base.Review, map[int64]bool) {
	recordsByID := make(map[int64]*githubReviewComment, len(records))
	var syntheticReviews []*base.Review
	for i := range records {
		record := &records[i]
		recordsByID[record.comment.ID] = record
		if _, ok := reviewsByID[record.reviewID]; !ok {
			review := &base.Review{
				ID:           record.reviewID,
				IssueIndex:   issueIndex,
				ReviewerID:   record.comment.PosterID,
				ReviewerName: record.comment.PosterName,
				CommitID:     record.reviewCommitID,
				CreatedAt:    record.comment.CreatedAt,
				State:        base.ReviewStateCommented,
			}
			reviewsByID[review.ID] = review
			syntheticReviews = append(syntheticReviews, review)
		}
	}

	vacatedReviews := make(map[int64]bool)
	appended := make(map[int64]bool, len(records))
	for i := range records {
		record := &records[i]
		root, isReply := recordsByID[record.comment.InReplyTo]
		if isReply && !appended[root.comment.ID] {
			rootReview := reviewsByID[root.reviewID]
			rootReview.Comments = append(rootReview.Comments, root.comment)
			appended[root.comment.ID] = true
		}
		if appended[record.comment.ID] {
			continue
		}
		review := reviewsByID[record.reviewID]
		if isReply {
			review = reviewsByID[root.reviewID]
			if review.ID != record.reviewID {
				vacatedReviews[record.reviewID] = true
			}
			record.comment.TreePath = root.comment.TreePath
			record.comment.DiffHunk = root.comment.DiffHunk
			record.comment.Position = root.comment.Position
			record.comment.Line = root.comment.Line
			record.comment.CommitID = root.comment.CommitID
		}
		review.Comments = append(review.Comments, record.comment)
		appended[record.comment.ID] = true
	}
	return syntheticReviews, vacatedReviews
}

var graphqlReactionContents = map[string]string{
	"THUMBS_UP":   "+1",
	"THUMBS_DOWN": "-1",
	"LAUGH":       "laugh",
	"HOORAY":      "hooray",
	"CONFUSED":    "confused",
	"HEART":       "heart",
	"ROCKET":      "rocket",
	"EYES":        "eyes",
}

const reviewReactionsQuery = `query($owner: String!, $name: String!, $number: Int!, $reviewCursor: String) {
  repository(owner: $owner, name: $name) {
    pullRequest(number: $number) {
      reviews(first: 100, after: $reviewCursor) {
        nodes {
          databaseId
          reactions(first: 100) {
            nodes {
              content
              createdAt
              user { login databaseId }
            }
            pageInfo { hasNextPage }
          }
        }
        pageInfo { hasNextPage endCursor }
      }
    }
  }
}`

type githubGraphQLPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type githubGraphQLConnection[T any] struct {
	Nodes    []T                   `json:"nodes"`
	PageInfo githubGraphQLPageInfo `json:"pageInfo"`
}

type githubGraphQLUser struct {
	Login      string `json:"login"`
	DatabaseID int64  `json:"databaseId"`
}

type githubGraphQLError struct {
	Message string `json:"message"`
}

type githubReviewReaction struct {
	Content   string            `json:"content"`
	CreatedAt time.Time         `json:"createdAt"`
	User      githubGraphQLUser `json:"user"`
}

type githubReactionReview struct {
	DatabaseID int64                                         `json:"databaseId"`
	Reactions  githubGraphQLConnection[githubReviewReaction] `json:"reactions"`
}

type githubReviewReactionsResponse struct {
	Errors []githubGraphQLError `json:"errors"`
	Data   struct {
		Repository struct {
			PullRequest struct {
				Reviews githubGraphQLConnection[githubReactionReview] `json:"reviews"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

func (g *GithubDownloaderV3) reviewReactions(ctx context.Context, prNumber int64) map[int64][]*base.Reaction {
	out := make(map[int64][]*base.Reaction)
	graphqlURL, err := url.Parse(g.getClient().BaseURL())
	if err != nil {
		log.Warn("Unable to determine the GitHub GraphQL URL, skipping review reactions: %v", err)
		return out
	}
	if strings.HasSuffix(graphqlURL.Path, "/api/v3/") {
		graphqlURL.Path = strings.TrimSuffix(graphqlURL.Path, "v3/") + "graphql"
	} else {
		graphqlURL.Path = "/graphql"
	}

	var reviewCursor any
	for {
		g.waitAndPickClient(ctx)
		body, err := json.Marshal(map[string]any{
			"query": reviewReactionsQuery,
			"variables": map[string]any{
				"owner":        g.repoOwner,
				"name":         g.repoName,
				"number":       prNumber,
				"reviewCursor": reviewCursor,
			},
		})
		if err != nil {
			log.Warn("Unable to fetch GitHub review reactions, skipping them: %v", err)
			return out
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL.String(), bytes.NewReader(body))
		if err != nil {
			log.Warn("Unable to fetch GitHub review reactions, skipping them: %v", err)
			return out
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := g.getClient().Client().Do(req)
		if err != nil {
			log.Warn("Unable to fetch GitHub review reactions, skipping them: %v", err)
			return out
		}
		var result githubReviewReactionsResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK || len(result.Errors) > 0 {
			log.Warn("Unable to fetch GitHub review reactions of %s/%s PR %d, skipping them: status %d, %v, %v", g.repoOwner, g.repoName, prNumber, resp.StatusCode, err, result.Errors)
			return out
		}

		reviews := result.Data.Repository.PullRequest.Reviews
		for _, review := range reviews.Nodes {
			if review.Reactions.PageInfo.HasNextPage {
				log.Warn("More than 100 reactions on review %d of %s/%s PR %d, dropping the rest", review.DatabaseID, g.repoOwner, g.repoName, prNumber)
			}
			for _, reaction := range review.Reactions.Nodes {
				content, ok := graphqlReactionContents[reaction.Content]
				if !ok {
					continue
				}
				out[review.DatabaseID] = append(out[review.DatabaseID], &base.Reaction{
					UserID:   reaction.User.DatabaseID,
					UserName: reaction.User.Login,
					Content:  content,
					Created:  reaction.CreatedAt,
				})
			}
		}
		if !reviews.PageInfo.HasNextPage {
			return out
		}
		if reviews.PageInfo.EndCursor == "" {
			log.Warn("GitHub returned another review reaction page without a cursor for %s/%s PR %d, dropping the rest", g.repoOwner, g.repoName, prNumber)
			return out
		}
		reviewCursor = reviews.PageInfo.EndCursor
	}
}

// GetReviews returns pull requests review
func (g *GithubDownloaderV3) GetReviews(ctx context.Context, reviewable base.Reviewable) ([]*base.Review, error) {
	allReviews := make([]*base.Review, 0, g.maxPerPage)
	if g.SkipReviews {
		return allReviews, nil
	}
	opt := &github.ListOptions{
		PerPage: g.maxPerPage,
	}
	// Get approve/request change reviews
	reviewsByID := make(map[int64]*base.Review)
	for {
		g.waitAndPickClient(ctx)
		reviews, resp, err := g.getClient().PullRequests.ListReviews(ctx, g.repoOwner, g.repoName, int(reviewable.GetForeignIndex()), opt)
		if err != nil {
			return nil, fmt.Errorf("error while listing repos: %w", err)
		}
		g.setRate(&resp.Rate)
		for _, review := range reviews {
			r := convertGithubReview(review)
			r.IssueIndex = reviewable.GetLocalIndex()
			reviewsByID[review.GetID()] = r
			allReviews = append(allReviews, r)
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	if !g.SkipReactions && len(reviewsByID) > 0 {
		for id, reactions := range g.reviewReactions(ctx, reviewable.GetForeignIndex()) {
			if review, ok := reviewsByID[id]; ok {
				review.Reactions = reactions
			}
		}
	}

	// The PR-wide endpoint includes line and side fields omitted per review.
	opt2 := &github.PullRequestListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: g.maxPerPage},
	}
	var reviewComments []githubReviewComment
	for {
		g.waitAndPickClient(ctx)
		page, resp, err := g.getClient().PullRequests.ListComments(ctx, g.repoOwner, g.repoName, int(reviewable.GetForeignIndex()), opt2)
		if err != nil {
			return nil, fmt.Errorf("error while listing review comments: %w", err)
		}
		g.setRate(&resp.Rate)

		comments, err := g.convertGithubReviewComments(ctx, page)
		if err != nil {
			return nil, err
		}
		reviewComments = append(reviewComments, comments...)
		if resp.NextPage == 0 {
			break
		}
		opt2.Page = resp.NextPage
	}

	syntheticReviews, vacatedReviews := groupGithubReviewComments(reviewsByID, reviewComments, reviewable.GetLocalIndex())
	allReviews = append(allReviews, syntheticReviews...)
	prunedReviews := allReviews[:0]
	for _, review := range allReviews {
		if vacatedReviews[review.ID] && review.State == base.ReviewStateCommented && review.Content == "" && len(review.Comments) == 0 && len(review.Reactions) == 0 {
			continue
		}
		prunedReviews = append(prunedReviews, review)
	}
	allReviews = prunedReviews

	prContext, _ := reviewable.GetContext().(githubPullRequestContext)
	for _, user := range prContext.requestedReviewers {
		r := &base.Review{
			ReviewerID:   user.ID,
			ReviewerName: user.Name,
			State:        base.ReviewStateRequestReview,
			IssueIndex:   reviewable.GetLocalIndex(),
		}
		allReviews = append(allReviews, r)
	}
	// TODO: Handle Team requests
	return allReviews, nil
}

// FormatCloneURL add authentication into remote URLs
func (g *GithubDownloaderV3) FormatCloneURL(opts MigrateOptions, remoteAddr string) (string, error) {
	u, err := url.Parse(remoteAddr)
	if err != nil {
		return "", err
	}
	if len(opts.AuthToken) > 0 {
		// "multiple tokens" are used to benefit more "API rate limit quota"
		// git clone doesn't count for rate limits, so only use the first token.
		// source: https://github.com/orgs/community/discussions/44515
		u.User = url.UserPassword("oauth2", strings.Split(opts.AuthToken, ",")[0])
	}
	return u.String(), nil
}
