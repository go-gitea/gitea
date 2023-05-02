// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
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
type GitlabDownloaderFactory struct{}

// New returns a Downloader related to this factory according MigrateOptions
func (f *GitlabDownloaderFactory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	baseURL := u.Scheme + "://" + u.Host
	repoNameSpace := strings.TrimPrefix(u.Path, "/")
	repoNameSpace = strings.TrimSuffix(repoNameSpace, ".git")

	log.Trace("Create gitlab downloader. BaseURL: %s RepoName: %s", baseURL, repoNameSpace)

	return NewGitlabDownloader(ctx, baseURL, repoNameSpace, opts.AuthUsername, opts.AuthPassword, opts.AuthToken)
}

// GitServiceType returns the type of git service
func (f *GitlabDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.GitlabService
}

// GitlabDownloader implements a Downloader interface to get repository information
// from gitlab via go-gitlab
// - issueCount is incremented in GetIssues() to ensure PR and Issue numbers do not overlap,
// because Gitlab has individual Issue and Pull Request numbers.
type GitlabDownloader struct {
	base.NullDownloader
	ctx        context.Context
	client     *gitlab.Client
	baseURL    string
	repoID     int
	repoName   string
	issueCount int64
	maxPerPage int
}

// NewGitlabDownloader creates a gitlab Downloader via gitlab API
//
//	Use either a username/password, personal token entered into the username field, or anonymous/public access
//	Note: Public access only allows very basic access
func NewGitlabDownloader(ctx context.Context, baseURL, repoPath, username, password, token string) (*GitlabDownloader, error) {
	gitlabClient, err := gitlab.NewClient(token, gitlab.WithBaseURL(baseURL), gitlab.WithHTTPClient(NewMigrationHTTPClient()))
	// Only use basic auth if token is blank and password is NOT
	// Basic auth will fail with empty strings, but empty token will allow anonymous public API usage
	if token == "" && password != "" {
		gitlabClient, err = gitlab.NewBasicAuthClient(username, password, gitlab.WithBaseURL(baseURL), gitlab.WithHTTPClient(NewMigrationHTTPClient()))
	}

	if err != nil {
		log.Trace("Error logging into gitlab: %v", err)
		return nil, err
	}

	// split namespace and subdirectory
	pathParts := strings.Split(strings.Trim(repoPath, "/"), "/")
	var resp *gitlab.Response
	u, _ := url.Parse(baseURL)
	for len(pathParts) >= 2 {
		_, resp, err = gitlabClient.Version.GetVersion()
		if err == nil || resp != nil && resp.StatusCode == http.StatusUnauthorized {
			err = nil // if no authentication given, this still should work
			break
		}

		u.Path = path.Join(u.Path, pathParts[0])
		baseURL = u.String()
		pathParts = pathParts[1:]
		_ = gitlab.WithBaseURL(baseURL)(gitlabClient)
		repoPath = strings.Join(pathParts, "/")
	}
	if err != nil {
		log.Trace("Error could not get gitlab version: %v", err)
		return nil, err
	}

	log.Trace("gitlab downloader: use BaseURL: '%s' and RepoPath: '%s'", baseURL, repoPath)

	// Grab and store project/repo ID here, due to issues using the URL escaped path
	gr, _, err := gitlabClient.Projects.GetProject(repoPath, nil, nil, gitlab.WithContext(ctx))
	if err != nil {
		log.Trace("Error retrieving project: %v", err)
		return nil, err
	}

	if gr == nil {
		log.Trace("Error getting project, project is nil")
		return nil, errors.New("Error getting project, project is nil")
	}

	return &GitlabDownloader{
		ctx:        ctx,
		client:     gitlabClient,
		baseURL:    baseURL,
		repoID:     gr.ID,
		repoName:   gr.Name,
		maxPerPage: 100,
	}, nil
}

// String implements Stringer
func (g *GitlabDownloader) String() string {
	return fmt.Sprintf("migration from gitlab server %s [%d]/%s", g.baseURL, g.repoID, g.repoName)
}

// ColorFormat provides a basic color format for a GitlabDownloader
func (g *GitlabDownloader) ColorFormat(s fmt.State) {
	if g == nil {
		log.ColorFprintf(s, "<nil: GitlabDownloader>")
		return
	}
	log.ColorFprintf(s, "migration from gitlab server %s [%d]/%s", g.baseURL, g.repoID, g.repoName)
}

// SetContext set context
func (g *GitlabDownloader) SetContext(ctx context.Context) {
	g.ctx = ctx
}

// GetRepoInfo returns a repository information
func (g *GitlabDownloader) GetRepoInfo() (*base.Repository, error) {
	gr, _, err := g.client.Projects.GetProject(g.repoID, nil, nil, gitlab.WithContext(g.ctx))
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
	gr, _, err := g.client.Projects.GetProject(g.repoID, nil, nil, gitlab.WithContext(g.ctx))
	if err != nil {
		return nil, err
	}
	return gr.TagList, err
}

// GetMilestones returns milestones
func (g *GitlabDownloader) GetMilestones() ([]*base.Milestone, error) {
	perPage := g.maxPerPage
	state := "all"
	milestones := make([]*base.Milestone, 0, perPage)
	for i := 1; ; i++ {
		ms, _, err := g.client.Milestones.ListMilestones(g.repoID, &gitlab.ListMilestonesOptions{
			State: &state,
			ListOptions: gitlab.ListOptions{
				Page:    i,
				PerPage: perPage,
			},
		}, nil, gitlab.WithContext(g.ctx))
		if err != nil {
			return nil, err
		}

		for _, m := range ms {
			var desc string
			if m.Description != "" {
				desc = m.Description
			}
			state := "open"
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

func (g *GitlabDownloader) resolvePriorityMap(ls []*gitlab.Label) map[int]base.LabelPriority {
	priorities := make(map[int]base.LabelPriority)
	var max, min int
	// Find max and min priorities
	for _, l := range ls {
		if max < l.Priority {
			max = l.Priority
		}
		if min > l.Priority || min == 0 {
			min = l.Priority
		}
	}
	// Create map
	for _, l := range ls {
		if l.Priority == 0 {
			continue
		}
		if l.Priority == max {
			priorities[l.Priority] = base.LabelPriorityCritical
		} else if l.Priority == min {
			priorities[l.Priority] = base.LabelPriorityMedium
		} else {
			priorities[l.Priority] = base.LabelPriorityHigh
		}
	}
	return priorities
}

// GetLabels returns labels
func (g *GitlabDownloader) GetLabels() ([]*base.Label, error) {
	perPage := g.maxPerPage
	labels := make([]*base.Label, 0, perPage)
	for i := 1; ; i++ {
		ls, _, err := g.client.Labels.ListLabels(g.repoID, &gitlab.ListLabelsOptions{ListOptions: gitlab.ListOptions{
			Page:    i,
			PerPage: perPage,
		}}, nil, gitlab.WithContext(g.ctx))
		if err != nil {
			return nil, err
		}
		pm := g.resolvePriorityMap(ls)
		for _, label := range ls {
			baseLabel := &base.Label{
				Name:        label.Name,
				Color:       g.normalizeColor(label.Color),
				Description: label.Description,
			}
			if label.Priority != 0 {
				baseLabel.Priority = pm[label.Priority]
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
	var zero int
	r := &base.Release{
		TagName:         rel.TagName,
		TargetCommitish: rel.Commit.ID,
		Name:            rel.Name,
		Body:            rel.Description,
		Created:         *rel.CreatedAt,
		PublisherID:     int64(rel.Author.ID),
		PublisherName:   rel.Author.Username,
	}

	httpClient := NewMigrationHTTPClient()

	for k, asset := range rel.Assets.Links {
		r.Assets = append(r.Assets, &base.ReleaseAsset{
			ID:            int64(asset.ID),
			Name:          asset.Name,
			ContentType:   &rel.Assets.Sources[k].Format,
			Size:          &zero,
			DownloadCount: &zero,
			DownloadFunc: func() (io.ReadCloser, error) {
				link, _, err := g.client.ReleaseLinks.GetReleaseLink(g.repoID, rel.TagName, asset.ID, gitlab.WithContext(g.ctx))
				if err != nil {
					return nil, err
				}

				if !hasBaseURL(link.URL, g.baseURL) {
					WarnAndNotice("Unexpected AssetURL for assetID[%d] in %s: %s", asset.ID, g, link.URL)
					return io.NopCloser(strings.NewReader(link.URL)), nil
				}

				req, err := http.NewRequest("GET", link.URL, nil)
				if err != nil {
					return nil, err
				}
				req = req.WithContext(g.ctx)
				resp, err := httpClient.Do(req)
				if err != nil {
					return nil, err
				}

				// resp.Body is closed by the uploader
				return resp.Body, nil
			},
		})
	}
	return r
}

// GetReleases returns releases
func (g *GitlabDownloader) GetReleases() ([]*base.Release, error) {
	perPage := g.maxPerPage
	releases := make([]*base.Release, 0, perPage)
	for i := 1; ; i++ {
		ls, _, err := g.client.Releases.ListReleases(g.repoID, &gitlab.ListReleasesOptions{
			ListOptions: gitlab.ListOptions{
				Page:    i,
				PerPage: perPage,
			},
		}, nil, gitlab.WithContext(g.ctx))
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

type gitlabIssueContext struct {
	IsMergeRequest bool
}

// GetIssues returns issues according start and limit
//
//	Note: issue label description and colors are not supported by the go-gitlab library at this time
func (g *GitlabDownloader) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	state := "all"
	sort := "asc"

	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}

	opt := &gitlab.ListProjectIssuesOptions{
		State: &state,
		Sort:  &sort,
		ListOptions: gitlab.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
	}

	allIssues := make([]*base.Issue, 0, perPage)

	issues, _, err := g.client.Issues.ListProjectIssues(g.repoID, opt, nil, gitlab.WithContext(g.ctx))
	if err != nil {
		return nil, false, fmt.Errorf("error while listing issues: %w", err)
	}
	for _, issue := range issues {

		labels := make([]*base.Label, 0, len(issue.Labels))
		for _, l := range issue.Labels {
			labels = append(labels, &base.Label{
				Name: l,
			})
		}

		var milestone string
		if issue.Milestone != nil {
			milestone = issue.Milestone.Title
		}

		var reactions []*base.Reaction
		awardPage := 1
		for {
			awards, _, err := g.client.AwardEmoji.ListIssueAwardEmoji(g.repoID, issue.IID, &gitlab.ListAwardEmojiOptions{Page: awardPage, PerPage: perPage}, gitlab.WithContext(g.ctx))
			if err != nil {
				return nil, false, fmt.Errorf("error while listing issue awards: %w", err)
			}

			for i := range awards {
				reactions = append(reactions, g.awardToReaction(awards[i]))
			}

			if len(awards) < perPage {
				break
			}

			awardPage++
		}

		allIssues = append(allIssues, &base.Issue{
			Title:        issue.Title,
			Number:       int64(issue.IID),
			PosterID:     int64(issue.Author.ID),
			PosterName:   issue.Author.Username,
			Content:      issue.Description,
			Milestone:    milestone,
			State:        issue.State,
			Created:      *issue.CreatedAt,
			Labels:       labels,
			Reactions:    reactions,
			Closed:       issue.ClosedAt,
			IsLocked:     issue.DiscussionLocked,
			Updated:      *issue.UpdatedAt,
			ForeignIndex: int64(issue.IID),
			Context:      gitlabIssueContext{IsMergeRequest: false},
		})

		// increment issueCount, to be used in GetPullRequests()
		g.issueCount++
	}

	return allIssues, len(issues) < perPage, nil
}

// GetComments returns comments according issueNumber
// TODO: figure out how to transfer comment reactions
func (g *GitlabDownloader) GetComments(commentable base.Commentable) ([]*base.Comment, bool, error) {
	context, ok := commentable.GetContext().(gitlabIssueContext)
	if !ok {
		return nil, false, fmt.Errorf("unexpected context: %+v", commentable.GetContext())
	}

	allComments := make([]*base.Comment, 0, g.maxPerPage)

	page := 1

	for {
		var comments []*gitlab.Discussion
		var resp *gitlab.Response
		var err error
		if !context.IsMergeRequest {
			comments, resp, err = g.client.Discussions.ListIssueDiscussions(g.repoID, int(commentable.GetForeignIndex()), &gitlab.ListIssueDiscussionsOptions{
				Page:    page,
				PerPage: g.maxPerPage,
			}, nil, gitlab.WithContext(g.ctx))
		} else {
			comments, resp, err = g.client.Discussions.ListMergeRequestDiscussions(g.repoID, int(commentable.GetForeignIndex()), &gitlab.ListMergeRequestDiscussionsOptions{
				Page:    page,
				PerPage: g.maxPerPage,
			}, nil, gitlab.WithContext(g.ctx))
		}

		if err != nil {
			return nil, false, fmt.Errorf("error while listing comments: %v %w", g.repoID, err)
		}
		for _, comment := range comments {
			// Flatten comment threads
			if !comment.IndividualNote {
				for _, note := range comment.Notes {
					allComments = append(allComments, &base.Comment{
						IssueIndex:  commentable.GetLocalIndex(),
						Index:       int64(note.ID),
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
					IssueIndex:  commentable.GetLocalIndex(),
					Index:       int64(c.ID),
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
	return allComments, true, nil
}

// GetPullRequests returns pull requests according page and perPage
func (g *GitlabDownloader) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}

	opt := &gitlab.ListProjectMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
	}

	allPRs := make([]*base.PullRequest, 0, perPage)

	prs, _, err := g.client.MergeRequests.ListProjectMergeRequests(g.repoID, opt, nil, gitlab.WithContext(g.ctx))
	if err != nil {
		return nil, false, fmt.Errorf("error while listing merge requests: %w", err)
	}
	for _, pr := range prs {

		labels := make([]*base.Label, 0, len(pr.Labels))
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

		mergeTime := pr.MergedAt
		if merged && pr.MergedAt == nil {
			mergeTime = pr.UpdatedAt
		}

		closeTime := pr.ClosedAt
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

		var reactions []*base.Reaction
		awardPage := 1
		for {
			awards, _, err := g.client.AwardEmoji.ListMergeRequestAwardEmoji(g.repoID, pr.IID, &gitlab.ListAwardEmojiOptions{Page: awardPage, PerPage: perPage}, gitlab.WithContext(g.ctx))
			if err != nil {
				return nil, false, fmt.Errorf("error while listing merge requests awards: %w", err)
			}

			for i := range awards {
				reactions = append(reactions, g.awardToReaction(awards[i]))
			}

			if len(awards) < perPage {
				break
			}

			awardPage++
		}

		// Add the PR ID to the Issue Count because PR and Issues share ID space in Gitea
		newPRNumber := g.issueCount + int64(pr.IID)

		allPRs = append(allPRs, &base.PullRequest{
			Title:          pr.Title,
			Number:         newPRNumber,
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
			Reactions:      reactions,
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
			PatchURL:     pr.WebURL + ".patch",
			ForeignIndex: int64(pr.IID),
			Context:      gitlabIssueContext{IsMergeRequest: true},
		})

		// SECURITY: Ensure that the PR is safe
		_ = CheckAndEnsureSafePR(allPRs[len(allPRs)-1], g.baseURL, g)
	}

	return allPRs, len(prs) < perPage, nil
}

// GetReviews returns pull requests review
func (g *GitlabDownloader) GetReviews(reviewable base.Reviewable) ([]*base.Review, error) {
	approvals, resp, err := g.client.MergeRequestApprovals.GetConfiguration(g.repoID, int(reviewable.GetForeignIndex()), gitlab.WithContext(g.ctx))
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			log.Error(fmt.Sprintf("GitlabDownloader: while migrating a error occurred: '%s'", err.Error()))
			return []*base.Review{}, nil
		}
		return nil, err
	}

	var createdAt time.Time
	if approvals.CreatedAt != nil {
		createdAt = *approvals.CreatedAt
	} else if approvals.UpdatedAt != nil {
		createdAt = *approvals.UpdatedAt
	} else {
		createdAt = time.Now()
	}

	reviews := make([]*base.Review, 0, len(approvals.ApprovedBy))
	for _, user := range approvals.ApprovedBy {
		reviews = append(reviews, &base.Review{
			IssueIndex:   reviewable.GetLocalIndex(),
			ReviewerID:   int64(user.User.ID),
			ReviewerName: user.User.Username,
			CreatedAt:    createdAt,
			// All we get are approvals
			State: base.ReviewStateApproved,
		})
	}

	return reviews, nil
}

func (g *GitlabDownloader) awardToReaction(award *gitlab.AwardEmoji) *base.Reaction {
	return &base.Reaction{
		UserID:   int64(award.User.ID),
		UserName: award.User.Username,
		Content:  award.Name,
	}
}
