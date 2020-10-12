// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/structs"

	"github.com/xanzy/go-gitlab"
)

var (
	_ base.Downloader        = &GitlabDownloader{}
	_ base.DownloaderFactory = &GitlabDownloaderFactory{}
)

func init() {
	RegisterDownloaderFactory(&GitlabDownloaderFactory{})
}

// GitlabDownloaderFactory defines a gitlab downloader factory
type GitlabDownloaderFactory struct {
}

// Match returns true if the migration remote URL matched this downloader factory
func (f *GitlabDownloaderFactory) Match(opts base.MigrateOptions) (bool, error) {
	var matched bool

	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return false, err
	}
	if strings.EqualFold(u.Host, "gitlab.com") && opts.AuthUsername != "" {
		matched = true
	}

	return matched, nil
}

// New returns a Downloader related to this factory according MigrateOptions
func (f *GitlabDownloaderFactory) New(opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	baseURL := u.Scheme + "://" + u.Host
	repoNameSpace := strings.TrimPrefix(u.Path, "/")
	repoNameSpace = strings.TrimSuffix(repoNameSpace, ".git")

	log.Trace("Create gitlab downloader. BaseURL: %s RepoName: %s", baseURL, repoNameSpace)

	return NewGitlabDownloader(baseURL, repoNameSpace, opts.AuthUsername, opts.AuthPassword, opts.AuthToken), nil
}

// GitServiceType returns the type of git service
func (f *GitlabDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.GitlabService
}

// GitlabDownloader implements a Downloader interface to get repository informations
// from gitlab via go-gitlab
// - issueCount is incremented in GetIssues() to ensure PR and Issue numbers do not overlap,
// because Gitlab has individual Issue and Pull Request numbers.
// - issueSeen, working alongside issueCount, is checked in GetComments() to see whether we
// need to fetch the Issue or PR comments, as Gitlab stores them separately.
type GitlabDownloader struct {
	ctx             context.Context
	client          *gitlab.Client
	repoID          int
	repoName        string
	issueCount      int64
	fetchPRcomments bool
}

// NewGitlabDownloader creates a gitlab Downloader via gitlab API
//   Use either a username/password, personal token entered into the username field, or anonymous/public access
//   Note: Public access only allows very basic access
func NewGitlabDownloader(baseURL, repoPath, username, password, token string) *GitlabDownloader {
	gitlabClient, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseURL))
	// Only use basic auth if token is blank and password is NOT
	// Basic auth will fail with empty strings, but empty token will allow anonymous public API usage
	if token == "" && password != "" {
		gitlabClient, err = gitlab.NewBasicAuthClient(username, password, gitlab.WithBaseURL(baseURL))
	}

	if err != nil {
		log.Trace("Error logging into gitlab: %v", err)
		return nil
	}

	// Grab and store project/repo ID here, due to issues using the URL escaped path
	gr, _, err := gitlabClient.Projects.GetProject(repoPath, nil, nil)
	if err != nil {
		log.Trace("Error retrieving project: %v", err)
		return nil
	}

	if gr == nil {
		log.Trace("Error getting project, project is nil")
		return nil
	}

	return &GitlabDownloader{
		ctx:      context.Background(),
		client:   gitlabClient,
		repoID:   gr.ID,
		repoName: gr.Name,
	}
}

// SetContext set context
func (g *GitlabDownloader) SetContext(ctx context.Context) {
	g.ctx = ctx
}

// GetRepoInfo returns a repository information
func (g *GitlabDownloader) GetRepoInfo() (*base.Repository, error) {
	if g == nil {
		return nil, errors.New("error: GitlabDownloader is nil")
	}

	gr, _, err := g.client.Projects.GetProject(g.repoID, nil, nil)
	if err != nil {
		return nil, err
	}

	var private bool
	switch gr.Visibility {
	case gitlab.InternalVisibility:
		private = true
	case gitlab.PrivateVisibility:
		private = true
	}

	var owner string
	if gr.Owner == nil {
		log.Trace("gr.Owner is nil, trying to get owner from Namespace")
		if gr.Namespace != nil && gr.Namespace.Kind == "user" {
			owner = gr.Namespace.Path
		}
	} else {
		owner = gr.Owner.Username
	}

	// convert gitlab repo to stand Repo
	return &base.Repository{
		Owner:         owner,
		Name:          gr.Name,
		IsPrivate:     private,
		Description:   gr.Description,
		OriginalURL:   gr.WebURL,
		CloneURL:      gr.HTTPURLToRepo,
		DefaultBranch: gr.DefaultBranch,
	}, nil
}

// GetTopics return gitlab topics
func (g *GitlabDownloader) GetTopics() ([]string, error) {
	if g == nil {
		return nil, errors.New("error: GitlabDownloader is nil")
	}

	gr, _, err := g.client.Projects.GetProject(g.repoID, nil, nil)
	if err != nil {
		return nil, err
	}
	return gr.TagList, err
}

// GetMilestones returns milestones
func (g *GitlabDownloader) GetMilestones() ([]*base.Milestone, error) {
	if g == nil {
		return nil, errors.New("error: GitlabDownloader is nil")
	}
	var perPage = 100
	var state = "all"
	var milestones = make([]*base.Milestone, 0, perPage)
	for i := 1; ; i++ {
		ms, _, err := g.client.Milestones.ListMilestones(g.repoID, &gitlab.ListMilestonesOptions{
			State: &state,
			ListOptions: gitlab.ListOptions{
				Page:    i,
				PerPage: perPage,
			}}, nil)
		if err != nil {
			return nil, err
		}

		for _, m := range ms {
			var desc string
			if m.Description != "" {
				desc = m.Description
			}
			var state = "open"
			var closedAt *time.Time
			if m.State != "" {
				state = m.State
				if state == "closed" {
					closedAt = m.UpdatedAt
				}
			}

			var deadline *time.Time
			if m.DueDate != nil {
				deadlineParsed, err := time.Parse("2006-01-02", m.DueDate.String())
				if err != nil {
					log.Trace("Error parsing Milestone DueDate time")
					deadline = nil
				} else {
					deadline = &deadlineParsed
				}
			}

			milestones = append(milestones, &base.Milestone{
				Title:       m.Title,
				Description: desc,
				Deadline:    deadline,
				State:       state,
				Created:     *m.CreatedAt,
				Updated:     m.UpdatedAt,
				Closed:      closedAt,
			})
		}
		if len(ms) < perPage {
			break
		}
	}
	return milestones, nil
}

func (g *GitlabDownloader) normalizeColor(val string) string {
	val = strings.TrimLeft(val, "#")
	val = strings.ToLower(val)
	if len(val) == 3 {
		c := []rune(val)
		val = fmt.Sprintf("%c%c%c%c%c%c", c[0], c[0], c[1], c[1], c[2], c[2])
	}
	if len(val) != 6 {
		return ""
	}
	return val
}

// GetLabels returns labels
func (g *GitlabDownloader) GetLabels() ([]*base.Label, error) {
	if g == nil {
		return nil, errors.New("error: GitlabDownloader is nil")
	}
	var perPage = 100
	var labels = make([]*base.Label, 0, perPage)
	for i := 1; ; i++ {
		ls, _, err := g.client.Labels.ListLabels(g.repoID, &gitlab.ListLabelsOptions{
			Page:    i,
			PerPage: perPage,
		}, nil)
		if err != nil {
			return nil, err
		}
		for _, label := range ls {
			baseLabel := &base.Label{
				Name:        label.Name,
				Color:       g.normalizeColor(label.Color),
				Description: label.Description,
			}
			labels = append(labels, baseLabel)
		}
		if len(ls) < perPage {
			break
		}
	}
	return labels, nil
}

func (g *GitlabDownloader) convertGitlabRelease(rel *gitlab.Release) *base.Release {

	r := &base.Release{
		TagName:         rel.TagName,
		TargetCommitish: rel.Commit.ID,
		Name:            rel.Name,
		Body:            rel.Description,
		Created:         *rel.CreatedAt,
		PublisherID:     int64(rel.Author.ID),
		PublisherName:   rel.Author.Username,
	}

	for k, asset := range rel.Assets.Links {
		r.Assets = append(r.Assets, base.ReleaseAsset{
			URL:         asset.URL,
			Name:        asset.Name,
			ContentType: &rel.Assets.Sources[k].Format,
		})
	}
	return r
}

// GetReleases returns releases
func (g *GitlabDownloader) GetReleases() ([]*base.Release, error) {
	var perPage = 100
	var releases = make([]*base.Release, 0, perPage)
	for i := 1; ; i++ {
		ls, _, err := g.client.Releases.ListReleases(g.repoID, &gitlab.ListReleasesOptions{
			Page:    i,
			PerPage: perPage,
		}, nil)
		if err != nil {
			return nil, err
		}

		for _, release := range ls {
			releases = append(releases, g.convertGitlabRelease(release))
		}
		if len(ls) < perPage {
			break
		}
	}
	return releases, nil
}

// GetIssues returns issues according start and limit
//   Note: issue label description and colors are not supported by the go-gitlab library at this time
//   TODO: figure out how to transfer issue reactions
func (g *GitlabDownloader) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	state := "all"
	sort := "asc"

	opt := &gitlab.ListProjectIssuesOptions{
		State: &state,
		Sort:  &sort,
		ListOptions: gitlab.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
	}

	var allIssues = make([]*base.Issue, 0, perPage)

	issues, _, err := g.client.Issues.ListProjectIssues(g.repoID, opt, nil)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing issues: %v", err)
	}
	for _, issue := range issues {

		var labels = make([]*base.Label, 0, len(issue.Labels))
		for _, l := range issue.Labels {
			labels = append(labels, &base.Label{
				Name: l,
			})
		}

		var milestone string
		if issue.Milestone != nil {
			milestone = issue.Milestone.Title
		}

		allIssues = append(allIssues, &base.Issue{
			Title:      issue.Title,
			Number:     int64(issue.IID),
			PosterID:   int64(issue.Author.ID),
			PosterName: issue.Author.Username,
			Content:    issue.Description,
			Milestone:  milestone,
			State:      issue.State,
			Created:    *issue.CreatedAt,
			Labels:     labels,
			Closed:     issue.ClosedAt,
			IsLocked:   issue.DiscussionLocked,
			Updated:    *issue.UpdatedAt,
		})

		// increment issueCount, to be used in GetPullRequests()
		g.issueCount++
	}

	return allIssues, len(issues) < perPage, nil
}

// GetComments returns comments according issueNumber
func (g *GitlabDownloader) GetComments(issueNumber int64) ([]*base.Comment, error) {
	var allComments = make([]*base.Comment, 0, 100)

	var page = 1
	var realIssueNumber int64

	for {
		var comments []*gitlab.Discussion
		var resp *gitlab.Response
		var err error
		// fetchPRcomments decides whether to fetch Issue or PR comments
		if !g.fetchPRcomments {
			realIssueNumber = issueNumber
			comments, resp, err = g.client.Discussions.ListIssueDiscussions(g.repoID, int(realIssueNumber), &gitlab.ListIssueDiscussionsOptions{
				Page:    page,
				PerPage: 100,
			}, nil)
		} else {
			// If this is a PR, we need to figure out the Gitlab/original PR ID to be passed below
			realIssueNumber = issueNumber - g.issueCount
			comments, resp, err = g.client.Discussions.ListMergeRequestDiscussions(g.repoID, int(realIssueNumber), &gitlab.ListMergeRequestDiscussionsOptions{
				Page:    page,
				PerPage: 100,
			}, nil)
		}

		if err != nil {
			return nil, fmt.Errorf("error while listing comments: %v %v", g.repoID, err)
		}
		for _, comment := range comments {
			// Flatten comment threads
			if !comment.IndividualNote {
				for _, note := range comment.Notes {
					allComments = append(allComments, &base.Comment{
						IssueIndex:  realIssueNumber,
						PosterID:    int64(note.Author.ID),
						PosterName:  note.Author.Username,
						PosterEmail: note.Author.Email,
						Content:     note.Body,
						Created:     *note.CreatedAt,
					})
				}
			} else {
				c := comment.Notes[0]
				allComments = append(allComments, &base.Comment{
					IssueIndex:  realIssueNumber,
					PosterID:    int64(c.Author.ID),
					PosterName:  c.Author.Username,
					PosterEmail: c.Author.Email,
					Content:     c.Body,
					Created:     *c.CreatedAt,
				})
			}

		}
		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}
	return allComments, nil
}

// GetPullRequests returns pull requests according page and perPage
func (g *GitlabDownloader) GetPullRequests(page, perPage int) ([]*base.PullRequest, error) {

	opt := &gitlab.ListProjectMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
	}

	// Set fetchPRcomments to true here, so PR comments are fetched instead of Issue comments
	g.fetchPRcomments = true

	var allPRs = make([]*base.PullRequest, 0, perPage)

	prs, _, err := g.client.MergeRequests.ListProjectMergeRequests(g.repoID, opt, nil)
	if err != nil {
		return nil, fmt.Errorf("error while listing merge requests: %v", err)
	}
	for _, pr := range prs {

		var labels = make([]*base.Label, 0, len(pr.Labels))
		for _, l := range pr.Labels {
			labels = append(labels, &base.Label{
				Name: l,
			})
		}

		var merged bool
		if pr.State == "merged" {
			merged = true
			pr.State = "closed"
		}

		var mergeTime = pr.MergedAt
		if merged && pr.MergedAt == nil {
			mergeTime = pr.UpdatedAt
		}

		var closeTime = pr.ClosedAt
		if merged && pr.ClosedAt == nil {
			closeTime = pr.UpdatedAt
		}

		var locked bool
		if pr.State == "locked" {
			locked = true
		}

		var milestone string
		if pr.Milestone != nil {
			milestone = pr.Milestone.Title
		}

		// Add the PR ID to the Issue Count because PR and Issues share ID space in Gitea
		newPRNumber := g.issueCount + int64(pr.IID)

		allPRs = append(allPRs, &base.PullRequest{
			Title:          pr.Title,
			Number:         newPRNumber,
			OriginalNumber: int64(pr.IID),
			PosterName:     pr.Author.Username,
			PosterID:       int64(pr.Author.ID),
			Content:        pr.Description,
			Milestone:      milestone,
			State:          pr.State,
			Created:        *pr.CreatedAt,
			Closed:         closeTime,
			Labels:         labels,
			Merged:         merged,
			MergeCommitSHA: pr.MergeCommitSHA,
			MergedTime:     mergeTime,
			IsLocked:       locked,
			Head: base.PullRequestBranch{
				Ref:       pr.SourceBranch,
				SHA:       pr.SHA,
				RepoName:  g.repoName,
				OwnerName: pr.Author.Username,
				CloneURL:  pr.WebURL,
			},
			Base: base.PullRequestBranch{
				Ref:       pr.TargetBranch,
				SHA:       pr.DiffRefs.BaseSha,
				RepoName:  g.repoName,
				OwnerName: pr.Author.Username,
			},
			PatchURL: pr.WebURL + ".patch",
		})
	}

	return allPRs, nil
}

// GetReviews returns pull requests review
func (g *GitlabDownloader) GetReviews(pullRequestNumber int64) ([]*base.Review, error) {

	state, _, err := g.client.MergeRequestApprovals.GetApprovalState(g.repoID, int(pullRequestNumber))
	if err != nil {
		return nil, err
	}

	// GitLab's Approvals are equivalent to Gitea's approve reviews
	approvers := make(map[int]string)
	for i := range state.Rules {
		for u := range state.Rules[i].ApprovedBy {
			approvers[state.Rules[i].ApprovedBy[u].ID] = state.Rules[i].ApprovedBy[u].Username
		}
	}

	var reviews = make([]*base.Review, 0, len(approvers))
	for id, name := range approvers {
		reviews = append(reviews, &base.Review{
			ReviewerID:   int64(id),
			ReviewerName: name,
			// GitLab API doesn't return a creation date
			CreatedAt: time.Now(),
			// All we get are approvals
			State: base.ReviewStateApproved,
		})
	}

	return reviews, nil
}
