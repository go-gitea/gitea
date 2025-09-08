// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/structs"

	"github.com/gogs/go-gogs-client"
)

var (
	_ base.Downloader        = &GogsDownloader{}
	_ base.DownloaderFactory = &GogsDownloaderFactory{}
)

func init() {
	RegisterDownloaderFactory(&GogsDownloaderFactory{})
}

// GogsDownloaderFactory defines a gogs downloader factory
type GogsDownloaderFactory struct{}

// New returns a Downloader related to this factory according MigrateOptions
func (f *GogsDownloaderFactory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	baseURL := u.Scheme + "://" + u.Host
	repoNameSpace := strings.TrimSuffix(u.Path, ".git")
	repoNameSpace = strings.Trim(repoNameSpace, "/")

	fields := strings.Split(repoNameSpace, "/")
	if len(fields) < 2 {
		return nil, fmt.Errorf("invalid path: %s", repoNameSpace)
	}

	log.Trace("Create gogs downloader. BaseURL: %s RepoOwner: %s RepoName: %s", baseURL, fields[0], fields[1])
	return NewGogsDownloader(ctx, baseURL, opts.AuthUsername, opts.AuthPassword, opts.AuthToken, fields[0], fields[1]), nil
}

// GitServiceType returns the type of git service
func (f *GogsDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.GogsService
}

// GogsDownloader implements a Downloader interface to get repository information
// from gogs via API
type GogsDownloader struct {
	base.NullDownloader
	baseURL            string
	repoOwner          string
	repoName           string
	userName           string
	password           string
	token              string
	openIssuesFinished bool
	openIssuesPages    int
}

// String implements Stringer
func (g *GogsDownloader) String() string {
	return fmt.Sprintf("migration from gogs server %s %s/%s", g.baseURL, g.repoOwner, g.repoName)
}

func (g *GogsDownloader) LogString() string {
	if g == nil {
		return "<GogsDownloader nil>"
	}
	return fmt.Sprintf("<GogsDownloader %s %s/%s>", g.baseURL, g.repoOwner, g.repoName)
}

// NewGogsDownloader creates a gogs Downloader via gogs API
func NewGogsDownloader(_ context.Context, baseURL, userName, password, token, repoOwner, repoName string) *GogsDownloader {
	downloader := GogsDownloader{
		baseURL:   baseURL,
		userName:  userName,
		password:  password,
		token:     token,
		repoOwner: repoOwner,
		repoName:  repoName,
	}
	return &downloader
}

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (rt roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

func (g *GogsDownloader) client(ctx context.Context) *gogs.Client {
	// Gogs client lacks the context support, so we use a custom transport
	// Then each request uses a dedicated client with its own context
	httpTransport := NewMigrationHTTPTransport()
	gogsClient := gogs.NewClient(g.baseURL, g.token)
	gogsClient.SetHTTPClient(&http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if g.password != "" {
				// Gogs client lacks the support for basic auth, this is the only way to set it
				req.SetBasicAuth(g.userName, g.password)
			}
			return httpTransport.RoundTrip(req.WithContext(ctx))
		}),
	})
	return gogsClient
}

// GetRepoInfo returns a repository information
func (g *GogsDownloader) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	gr, err := g.client(ctx).GetRepo(g.repoOwner, g.repoName)
	if err != nil {
		return nil, err
	}

	// convert gogs repo to stand Repo
	return &base.Repository{
		Owner:         g.repoOwner,
		Name:          g.repoName,
		IsPrivate:     gr.Private,
		Description:   gr.Description,
		CloneURL:      gr.CloneURL,
		OriginalURL:   gr.HTMLURL,
		DefaultBranch: gr.DefaultBranch,
	}, nil
}

// GetMilestones returns milestones
func (g *GogsDownloader) GetMilestones(ctx context.Context) ([]*base.Milestone, error) {
	perPage := 100
	milestones := make([]*base.Milestone, 0, perPage)

	ms, err := g.client(ctx).ListRepoMilestones(g.repoOwner, g.repoName)
	if err != nil {
		return nil, err
	}

	for _, m := range ms {
		milestones = append(milestones, &base.Milestone{
			Title:       m.Title,
			Description: m.Description,
			Deadline:    m.Deadline,
			State:       string(m.State),
			Closed:      m.Closed,
		})
	}

	return milestones, nil
}

// GetLabels returns labels
func (g *GogsDownloader) GetLabels(ctx context.Context) ([]*base.Label, error) {
	perPage := 100
	labels := make([]*base.Label, 0, perPage)
	ls, err := g.client(ctx).ListRepoLabels(g.repoOwner, g.repoName)
	if err != nil {
		return nil, err
	}

	for _, label := range ls {
		labels = append(labels, convertGogsLabel(label))
	}

	return labels, nil
}

// GetIssues returns issues according start and limit, perPage is not supported
func (g *GogsDownloader) GetIssues(ctx context.Context, page, _ int) ([]*base.Issue, bool, error) {
	var state string
	if g.openIssuesFinished {
		state = string(gogs.STATE_CLOSED)
		page -= g.openIssuesPages
	} else {
		state = string(gogs.STATE_OPEN)
		g.openIssuesPages = page
	}

	issues, isEnd, err := g.getIssues(ctx, page, state)
	if err != nil {
		return nil, false, err
	}

	if isEnd {
		if g.openIssuesFinished {
			return issues, true, nil
		}
		g.openIssuesFinished = true
	}

	return issues, false, nil
}

func (g *GogsDownloader) getIssues(ctx context.Context, page int, state string) ([]*base.Issue, bool, error) {
	allIssues := make([]*base.Issue, 0, 10)

	issues, err := g.client(ctx).ListRepoIssues(g.repoOwner, g.repoName, gogs.ListIssueOption{
		Page:  page,
		State: state,
	})
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %w", err)
	}

	for _, issue := range issues {
		if issue.PullRequest != nil {
			continue
		}
		allIssues = append(allIssues, convertGogsIssue(issue))
	}

	return allIssues, len(issues) == 0, nil
}

// GetComments returns comments according issueNumber
func (g *GogsDownloader) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	allComments := make([]*base.Comment, 0, 100)

	comments, err := g.client(ctx).ListIssueComments(g.repoOwner, g.repoName, commentable.GetForeignIndex())
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %w", err)
	}
	for _, comment := range comments {
		if len(comment.Body) == 0 || comment.Poster == nil {
			continue
		}
		allComments = append(allComments, &base.Comment{
			IssueIndex:  commentable.GetLocalIndex(),
			Index:       comment.ID,
			PosterID:    comment.Poster.ID,
			PosterName:  comment.Poster.Login,
			PosterEmail: comment.Poster.Email,
			Content:     comment.Body,
			Created:     comment.Created,
			Updated:     comment.Updated,
		})
	}

	return allComments, true, nil
}

// GetTopics return repository topics
func (g *GogsDownloader) GetTopics(_ context.Context) ([]string, error) {
	return []string{}, nil
}

// FormatCloneURL add authentication into remote URLs
func (g *GogsDownloader) FormatCloneURL(opts MigrateOptions, remoteAddr string) (string, error) {
	if len(opts.AuthToken) > 0 || len(opts.AuthUsername) > 0 {
		u, err := url.Parse(remoteAddr)
		if err != nil {
			return "", err
		}
		if len(opts.AuthToken) != 0 {
			u.User = url.UserPassword(opts.AuthToken, "")
		} else {
			u.User = url.UserPassword(opts.AuthUsername, opts.AuthPassword)
		}
		return u.String(), nil
	}
	return remoteAddr, nil
}

func convertGogsIssue(issue *gogs.Issue) *base.Issue {
	var milestone string
	if issue.Milestone != nil {
		milestone = issue.Milestone.Title
	}
	labels := make([]*base.Label, 0, len(issue.Labels))
	for _, l := range issue.Labels {
		labels = append(labels, convertGogsLabel(l))
	}

	var closed *time.Time
	if issue.State == gogs.STATE_CLOSED {
		// gogs client haven't provide closed, so we use updated instead
		closed = &issue.Updated
	}

	return &base.Issue{
		Title:        issue.Title,
		Number:       issue.Index,
		PosterID:     issue.Poster.ID,
		PosterName:   issue.Poster.Login,
		PosterEmail:  issue.Poster.Email,
		Content:      issue.Body,
		Milestone:    milestone,
		State:        string(issue.State),
		Created:      issue.Created,
		Updated:      issue.Updated,
		Labels:       labels,
		Closed:       closed,
		ForeignIndex: issue.Index,
	}
}

func convertGogsLabel(label *gogs.Label) *base.Label {
	return &base.Label{
		Name:  label.Name,
		Color: label.Color,
	}
}
