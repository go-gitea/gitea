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

	"gitea.dev/modules/git"
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

	downloader, err := NewGithubDownloaderV3(ctx, baseURL, opts.AuthUsername, opts.AuthPassword, opts.AuthToken, oldOwner, oldName)
	if err != nil {
		return nil, err
	}
	downloader.SkipReactions = opts.SkipReactions
	downloader.useGraphQL = true
	return downloader, nil
}

// GitServiceType returns the type of git service
func (f *GithubDownloaderV3Factory) GitServiceType() structs.GitServiceType {
	return structs.GithubService
}

// GithubDownloaderV3 implements a Downloader interface to get repository information
// from github via APIv3
type GithubDownloaderV3 struct {
	base.NullDownloader
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
	// issuesCursor is the Link-header `after` cursor for paginating the issues
	// list. The issues endpoint caps classic page-number pagination at ~page 100,
	// so a large repo's issues must be walked by cursor instead. Carried on the
	// downloader (created fresh per sync); reset at the start of each sweep.
	issuesCursor string

	// useGraphQL opts the issues stream into the batched GraphQL fast path
	// (see github_graphql.go), which fetches an issue plus its comments and
	// reactions in one request instead of many. gqlIssuesCursor is that path's
	// pagination cursor and gqlComments caches the comments fetched alongside the
	// issues so the framework's separate comment phase serves them from memory.
	useGraphQL      bool
	gqlIssuesCursor string
	gqlComments     map[int64][]*base.Comment
	gqlCommentsFlat []*base.Comment
	// gqlPRCursor paginates the GraphQL pull-request sweep; gqlReviews caches the
	// reviews (with their inline comments) fetched alongside each PR so the
	// framework's separate review phase serves them from memory. PR issue-comments
	// join gqlComments so the comment phase serves issue and PR comments together.
	gqlPRCursor string
	gqlReviews  map[int64][]*base.Review
	// gqlPointsSpent accumulates GitHub's GraphQL points budget spent this run
	// (benchmark instrumentation; GraphQL is billed on points, not requests/hr).
	gqlPointsSpent int64
}

// NewGithubDownloaderV3 creates a github Downloader via github v3 API
func NewGithubDownloaderV3(_ context.Context, baseURL, userName, password, token, repoOwner, repoName string) (*GithubDownloaderV3, error) {
	downloader := GithubDownloaderV3{
		userName:   userName,
		baseURL:    baseURL,
		password:   password,
		repoOwner:  repoOwner,
		repoName:   repoName,
		maxPerPage: 100,
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
					Base:   newRetryTransport(NewMigrationHTTPTransport()),
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
			Transport: newRetryTransport(transport),
		}
		if err := downloader.addClient(client, baseURL); err != nil {
			return nil, err
		}
	}
	return &downloader, nil
}

// SupportSyncing returns true if it supports syncing an already-migrated repository
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
		if g.rates[i] != nil && g.rates[i].Remaining > maxRemaining {
			maxRemaining = g.rates[i].Remaining
			recentIdx = i
		}
	}
	g.curClientIdx = recentIdx // if no max remain, it will always pick the first client.

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

	httpClient := NewMigrationHTTPClient()

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
	// A one-time migration walks by creation order; a zero time means all issues.
	return g.getIssuesSince(ctx, page, perPage, time.Time{}, "created")
}

// GetNewIssues returns issues updated after the given time, paginated
func (g *GithubDownloaderV3) GetNewIssues(ctx context.Context, page, perPage int, updatedAfter time.Time) ([]*base.Issue, bool, error) {
	// A resumable sync walks by UPDATE order so the max updated_unix already
	// stored is an exact resume point: everything before it is done, everything
	// at/after it still needs syncing. (Walking by creation order would let the
	// updated-based watermark skip older-but-recently-touched issues.)
	if g.useGraphQL {
		// GraphQL fast path: fetches issues + comments + reactions in one request.
		return g.getNewIssuesGraphQL(ctx, page, updatedAfter)
	}
	return g.getIssuesSince(ctx, page, perPage, updatedAfter, "updated")
}

// getIssuesSince returns issues updated after the given time sorted ascending by
// sortField ("created" or "updated"); a zero time returns all issues
func (g *GithubDownloaderV3) getIssuesSince(ctx context.Context, page, perPage int, since time.Time, sortField string) ([]*base.Issue, bool, error) {
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	// Paginate by the Link-header `after` cursor, not a page number: GitHub caps
	// classic page-number pagination on the issues endpoint at ~page 100 (~10k
	// items), silently truncating a large repository. The cursor has no such cap.
	// page<=1 marks the first request of a sweep, so reset the cursor there.
	if page <= 1 {
		g.issuesCursor = ""
	}
	opt := &github.IssueListByRepoOptions{
		Sort:              sortField,
		Direction:         "asc",
		State:             "all",
		Since:             since,
		ListCursorOptions: github.ListCursorOptions{After: g.issuesCursor},
		ListOptions:       github.ListOptions{PerPage: perPage},
	}

	allIssues := make([]*base.Issue, 0, perPage)
	g.waitAndPickClient(ctx)
	issues, resp, err := g.getClient().Issues.ListByRepo(ctx, g.repoOwner, g.repoName, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %w", err)
	}
	log.Trace("Request get issues cursor=%q got %d, next=%q", g.issuesCursor, len(issues), resp.After)
	g.setRate(&resp.Rate)
	g.issuesCursor = resp.After
	for _, issue := range issues {
		if issue.IsPullRequest() {
			continue
		}

		labels := make([]*base.Label, 0, len(issue.Labels))
		for _, l := range issue.Labels {
			labels = append(labels, convertGithubLabel(l))
		}

		reactions, err := g.getIssueReactions(ctx, issue.GetNumber(), perPage)
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

	// End when the cursor is exhausted AND a short page confirms no more results.
	// Both conditions together handle cursor-based and page-based responses.
	isEnd := resp.After == "" && len(issues) < perPage
	return allIssues, isEnd, nil
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
	return g.getCommentsSince(ctx, commentable, nil)
}

// getCommentsSince returns an issue's or pull request's comments; a non-nil
// since returns only those updated at or after it
func (g *GithubDownloaderV3) getCommentsSince(ctx context.Context, commentable base.Commentable, since *time.Time) ([]*base.Comment, error) {
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
	// A one-time migration walks by creation order.
	return g.getAllCommentsSince(ctx, page, perPage, nil, "created")
}

// getAllCommentsSince returns all repository issue and pull request comments
// paginated; a non-nil since returns only those updated at or after it
func (g *GithubDownloaderV3) getAllCommentsSince(ctx context.Context, page, perPage int, since *time.Time, sortField string) ([]*base.Comment, bool, error) {
	var (
		allComments = make([]*base.Comment, 0, perPage)
		asc         = "asc"
	)
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	opt := &github.IssueListCommentsOptions{
		Sort:      &sortField,
		Direction: &asc,
		Since:     since,
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
		basePR, err := g.convertGithubPullRequest(ctx, pr, perPage)
		if err != nil {
			return nil, false, err
		}
		allPRs = append(allPRs, basePR)

		// SECURITY: Ensure that the PR is safe
		_ = CheckAndEnsureSafePR(basePR, g.baseURL, g)
	}

	// Terminate on the Link header, not len(prs) < perPage (see getIssuesSince):
	// a short page mid-results must not be misread as the end of a large backfill.
	return allPRs, resp.NextPage == 0, nil
}

// GetNewPullRequests returns pull requests updated after the given time, paginated.
// The pull request list API has no `since` filter, so it lists by most recently
// updated and stops as soon as a pull request older than updatedAfter appears.
// The search API is deliberately avoided: its results are capped at 1,000 and it
// has a separate, much smaller rate limit.
func (g *GithubDownloaderV3) GetNewPullRequests(ctx context.Context, page, perPage int, updatedAfter time.Time) ([]*base.PullRequest, bool, error) {
	if g.useGraphQL {
		// GraphQL fast path: fetches PRs + comments + reviews + review comments in
		// one batched request instead of the REST per-PR review/comment N+1.
		return g.getNewPullRequestsGraphQL(ctx, page, updatedAfter)
	}
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	opt := &github.PullRequestListOptions{
		Sort:      "updated",
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
		return nil, false, fmt.Errorf("error while listing pull requests: %w", err)
	}
	log.Trace("Request get new pull requests %d/%d, but in fact get %d", perPage, page, len(prs))
	g.setRate(&resp.Rate)
	for _, pr := range prs {
		// Walk ascending by update time and skip what is already synced (older
		// than the watermark). The GitHub pull-request list has no server-side
		// "since" filter, so the already-done head is skipped client-side;
		// paginating to the true end rather than stopping early is what lets a
		// partial sweep resume from the max updated_unix already stored.
		if pr.GetUpdatedAt().Time.Before(updatedAfter) {
			continue
		}
		basePR, err := g.convertGithubPullRequest(ctx, pr, perPage)
		if err != nil {
			return nil, false, err
		}
		allPRs = append(allPRs, basePR)

		// SECURITY: Ensure that the PR is safe
		_ = CheckAndEnsureSafePR(basePR, g.baseURL, g)
	}

	// Terminate on the Link header, not len(prs) < perPage (see getIssuesSince):
	// a short page mid-results must not be misread as the end of a large backfill.
	return allPRs, resp.NextPage == 0, nil
}

func (g *GithubDownloaderV3) convertGithubPullRequest(ctx context.Context, pr *github.PullRequest, perPage int) (*base.PullRequest, error) {
	labels := make([]*base.Label, 0, len(pr.Labels))
	for _, l := range pr.Labels {
		labels = append(labels, convertGithubLabel(l))
	}

	reactions, err := g.getIssueReactions(ctx, pr.GetNumber(), perPage)
	if err != nil {
		return nil, err
	}

	// download patch and saved as tmp file
	g.waitAndPickClient(ctx)

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
			CloneURL:  pr.GetHead().GetRepo().GetCloneURL(), // see below for SECURITY related issues here
		},
		Base: base.PullRequestBranch{
			Ref:       pr.GetBase().GetRef(),
			SHA:       pr.GetBase().GetSHA(),
			RepoName:  pr.GetBase().GetRepo().GetName(),
			OwnerName: pr.GetBase().GetUser().GetLogin(),
		},
		PatchURL:     pr.GetPatchURL(), // see below for SECURITY related issues here
		Reactions:    reactions,
		ForeignIndex: int64(*pr.Number),
		IsDraft:      pr.GetDraft(),
	}, nil
}

// getIssueReactions returns the reactions on an issue or pull request
func (g *GithubDownloaderV3) getIssueReactions(ctx context.Context, number, perPage int) ([]*base.Reaction, error) {
	var reactions []*base.Reaction
	if g.SkipReactions {
		return reactions, nil
	}
	for i := 1; ; i++ {
		g.waitAndPickClient(ctx)
		res, resp, err := g.getClient().Reactions.ListIssueReactions(ctx, g.repoOwner, g.repoName, number, &github.ListReactionOptions{
			ListOptions: github.ListOptions{
				Page:    i,
				PerPage: perPage,
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
			})
		}
	}
	return reactions, nil
}

func convertGithubReview(r *github.PullRequestReview) *base.Review {
	return &base.Review{
		ID:           r.GetID(),
		ReviewerID:   r.GetUser().GetID(),
		ReviewerName: r.GetUser().GetLogin(),
		CommitID:     r.GetCommitID(),
		Content:      r.GetBody(),
		CreatedAt:    r.GetSubmittedAt().Time,
		State:        r.GetState(),
	}
}

func (g *GithubDownloaderV3) convertGithubReviewComments(ctx context.Context, cs []*github.PullRequestComment) ([]*base.ReviewComment, error) {
	rcs := make([]*base.ReviewComment, 0, len(cs))
	for _, c := range cs {
		// get reactions
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
					})
				}
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
			CreatedAt: c.GetCreatedAt().Time,
			UpdatedAt: c.GetUpdatedAt().Time,
		})
	}
	return rcs, nil
}

// GetReviews returns pull requests review
// nilIfZero returns a pointer to t, or nil when t is the zero time. The comment
// APIs take a *time.Time `since`; a pointer to the zero time would be serialized
// as since=0001-01-01, which GitHub rejects with 422, so a zero time (first
// sync, no watermark) must be sent as nil to omit the filter and fetch all.
func nilIfZero(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// GetNewComments returns an issue's or pull request's comments updated at or
// after the given time
func (g *GithubDownloaderV3) GetNewComments(ctx context.Context, commentable base.Commentable, updatedAfter time.Time) ([]*base.Comment, bool, error) {
	comments, err := g.getCommentsSince(ctx, commentable, nilIfZero(updatedAfter))
	return comments, false, err
}

// GetAllNewComments returns all repository comments updated at or after the
// given time, paginated
func (g *GithubDownloaderV3) GetAllNewComments(ctx context.Context, page, perPage int, updatedAfter time.Time) ([]*base.Comment, bool, error) {
	if g.useGraphQL {
		// GraphQL fast path: comments already came back with their issues; serve
		// them from the cache instead of a second round of API calls.
		return g.getCachedComments(page, perPage)
	}
	// A resumable sync walks by UPDATE order so the max comment updated_unix
	// already stored is an exact resume point.
	return g.getAllCommentsSince(ctx, page, perPage, nilIfZero(updatedAfter), "updated")
}

// GetNewReviews returns a pull request's reviews updated at or after the given
// time. GitHub's reviews API has no since filter, so all reviews are refetched.
func (g *GithubDownloaderV3) GetNewReviews(ctx context.Context, reviewable base.Reviewable, updatedAfter time.Time) ([]*base.Review, error) {
	if g.useGraphQL {
		// GraphQL fast path: reviews (and their inline comments) already came back
		// with their pull request; serve them from the cache instead of the REST
		// per-PR ListReviews + per-review ListReviewComments N+1.
		return g.gqlReviews[reviewable.GetForeignIndex()], nil
	}
	return g.GetReviews(ctx, reviewable)
}

func (g *GithubDownloaderV3) GetReviews(ctx context.Context, reviewable base.Reviewable) ([]*base.Review, error) {
	allReviews := make([]*base.Review, 0, g.maxPerPage)
	if g.SkipReviews {
		return allReviews, nil
	}
	opt := &github.ListOptions{
		PerPage: g.maxPerPage,
	}
	// Get approve/request change reviews
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
			// retrieve all review comments
			opt2 := &github.ListOptions{
				PerPage: g.maxPerPage,
			}
			for {
				g.waitAndPickClient(ctx)
				reviewComments, resp, err := g.getClient().PullRequests.ListReviewComments(ctx, g.repoOwner, g.repoName, int(reviewable.GetForeignIndex()), review.GetID(), opt2)
				if err != nil {
					return nil, fmt.Errorf("error while listing repos: %w", err)
				}
				g.setRate(&resp.Rate)

				cs, err := g.convertGithubReviewComments(ctx, reviewComments)
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
	g.waitAndPickClient(ctx)
	reviewers, resp, err := g.getClient().PullRequests.ListReviewers(ctx, g.repoOwner, g.repoName, int(reviewable.GetForeignIndex()))
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
