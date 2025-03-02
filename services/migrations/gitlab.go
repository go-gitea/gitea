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
	"regexp"
	"strings"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/structs"

	gitlab "gitlab.com/gitlab-org/api/client-go"
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

type gitlabIIDResolver struct {
	maxIssueIID int64
	frozen      bool
}

func (r *gitlabIIDResolver) recordIssueIID(issueIID int) {
	if r.frozen {
		panic("cannot record issue IID after pull request IID generation has started")
	}
	r.maxIssueIID = max(r.maxIssueIID, int64(issueIID))
}

func (r *gitlabIIDResolver) generatePullRequestNumber(mrIID int) int64 {
	r.frozen = true
	return r.maxIssueIID + int64(mrIID)
}

// GitlabDownloader implements a Downloader interface to get repository information
// from gitlab via go-gitlab
// - issueCount is incremented in GetIssues() to ensure PR and Issue numbers do not overlap,
// because Gitlab has individual Issue and Pull Request numbers.
type GitlabDownloader struct {
	base.NullDownloader
	client      *gitlab.Client
	baseURL     string
	repoID      int
	repoName    string
	iidResolver gitlabIIDResolver
	maxPerPage  int
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

func (g *GitlabDownloader) LogString() string {
	if g == nil {
		return "<GitlabDownloader nil>"
	}
	return fmt.Sprintf("<GitlabDownloader %s [%d]/%s>", g.baseURL, g.repoID, g.repoName)
}

// GetRepoInfo returns a repository information
func (g *GitlabDownloader) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	gr, _, err := g.client.Projects.GetProject(g.repoID, nil, nil, gitlab.WithContext(ctx))
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
func (g *GitlabDownloader) GetTopics(ctx context.Context) ([]string, error) {
	gr, _, err := g.client.Projects.GetProject(g.repoID, nil, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return gr.TagList, err
}

// GetMilestones returns milestones
func (g *GitlabDownloader) GetMilestones(ctx context.Context) ([]*base.Milestone, error) {
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
		}, nil, gitlab.WithContext(ctx))
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

// GetLabels returns labels
func (g *GitlabDownloader) GetLabels(ctx context.Context) ([]*base.Label, error) {
	perPage := g.maxPerPage
	labels := make([]*base.Label, 0, perPage)
	for i := 1; ; i++ {
		ls, _, err := g.client.Labels.ListLabels(g.repoID, &gitlab.ListLabelsOptions{ListOptions: gitlab.ListOptions{
			Page:    i,
			PerPage: perPage,
		}}, nil, gitlab.WithContext(ctx))
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

func (g *GitlabDownloader) convertGitlabRelease(ctx context.Context, rel *gitlab.Release) *base.Release {
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
		assetID := asset.ID // Don't optimize this, for closure we need a local variable
		r.Assets = append(r.Assets, &base.ReleaseAsset{
			ID:            int64(asset.ID),
			Name:          asset.Name,
			ContentType:   &rel.Assets.Sources[k].Format,
			Size:          &zero,
			DownloadCount: &zero,
			DownloadFunc: func() (io.ReadCloser, error) {
				link, _, err := g.client.ReleaseLinks.GetReleaseLink(g.repoID, rel.TagName, assetID, gitlab.WithContext(ctx))
				if err != nil {
					return nil, err
				}

				if !hasBaseURL(link.URL, g.baseURL) {
					WarnAndNotice("Unexpected AssetURL for assetID[%d] in %s: %s", assetID, g, link.URL)
					return io.NopCloser(strings.NewReader(link.URL)), nil
				}

				req, err := http.NewRequest("GET", link.URL, nil)
				if err != nil {
					return nil, err
				}
				req = req.WithContext(ctx)
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
func (g *GitlabDownloader) GetReleases(ctx context.Context) ([]*base.Release, error) {
	perPage := g.maxPerPage
	releases := make([]*base.Release, 0, perPage)
	for i := 1; ; i++ {
		ls, _, err := g.client.Releases.ListReleases(g.repoID, &gitlab.ListReleasesOptions{
			ListOptions: gitlab.ListOptions{
				Page:    i,
				PerPage: perPage,
			},
		}, nil, gitlab.WithContext(ctx))
		if err != nil {
			return nil, err
		}

		for _, release := range ls {
			releases = append(releases, g.convertGitlabRelease(ctx, release))
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
func (g *GitlabDownloader) GetIssues(ctx context.Context, page, perPage int) ([]*base.Issue, bool, error) {
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

	issues, _, err := g.client.Issues.ListProjectIssues(g.repoID, opt, nil, gitlab.WithContext(ctx))
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

		var reactions []*gitlab.AwardEmoji
		awardPage := 1
		for {
			awards, _, err := g.client.AwardEmoji.ListIssueAwardEmoji(g.repoID, issue.IID, &gitlab.ListAwardEmojiOptions{Page: awardPage, PerPage: perPage}, gitlab.WithContext(ctx))
			if err != nil {
				return nil, false, fmt.Errorf("error while listing issue awards: %w", err)
			}

			reactions = append(reactions, awards...)

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
			Reactions:    g.awardsToReactions(reactions),
			Closed:       issue.ClosedAt,
			IsLocked:     issue.DiscussionLocked,
			Updated:      *issue.UpdatedAt,
			ForeignIndex: int64(issue.IID),
			Context:      gitlabIssueContext{IsMergeRequest: false},
		})

		// record the issue IID, to be used in GetPullRequests()
		g.iidResolver.recordIssueIID(issue.IID)
	}

	return allIssues, len(issues) < perPage, nil
}

// GetComments returns comments according issueNumber
// TODO: figure out how to transfer comment reactions
func (g *GitlabDownloader) GetComments(ctx context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
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
			}, nil, gitlab.WithContext(ctx))
		} else {
			comments, resp, err = g.client.Discussions.ListMergeRequestDiscussions(g.repoID, int(commentable.GetForeignIndex()), &gitlab.ListMergeRequestDiscussionsOptions{
				Page:    page,
				PerPage: g.maxPerPage,
			}, nil, gitlab.WithContext(ctx))
		}

		if err != nil {
			return nil, false, fmt.Errorf("error while listing comments: %v %w", g.repoID, err)
		}
		for _, comment := range comments {
			for _, note := range comment.Notes {
				allComments = append(allComments, g.convertNoteToComment(commentable.GetLocalIndex(), note))
			}
		}
		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	page = 1
	for {
		var stateEvents []*gitlab.StateEvent
		var resp *gitlab.Response
		var err error
		if context.IsMergeRequest {
			stateEvents, resp, err = g.client.ResourceStateEvents.ListMergeStateEvents(g.repoID, int(commentable.GetForeignIndex()), &gitlab.ListStateEventsOptions{
				ListOptions: gitlab.ListOptions{
					Page:    page,
					PerPage: g.maxPerPage,
				},
			}, nil, gitlab.WithContext(ctx))
		} else {
			stateEvents, resp, err = g.client.ResourceStateEvents.ListIssueStateEvents(g.repoID, int(commentable.GetForeignIndex()), &gitlab.ListStateEventsOptions{
				ListOptions: gitlab.ListOptions{
					Page:    page,
					PerPage: g.maxPerPage,
				},
			}, nil, gitlab.WithContext(ctx))
		}
		if err != nil {
			return nil, false, fmt.Errorf("error while listing state events: %v %w", g.repoID, err)
		}

		for _, stateEvent := range stateEvents {
			comment := &base.Comment{
				IssueIndex: commentable.GetLocalIndex(),
				Index:      int64(stateEvent.ID),
				PosterID:   int64(stateEvent.User.ID),
				PosterName: stateEvent.User.Username,
				Content:    "",
				Created:    *stateEvent.CreatedAt,
			}
			switch stateEvent.State {
			case gitlab.ClosedEventType:
				comment.CommentType = issues_model.CommentTypeClose.String()
			case gitlab.MergedEventType:
				comment.CommentType = issues_model.CommentTypeMergePull.String()
			case gitlab.ReopenedEventType:
				comment.CommentType = issues_model.CommentTypeReopen.String()
			default:
				// Ignore other event types
				continue
			}
			allComments = append(allComments, comment)
		}

		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return allComments, true, nil
}

var targetBranchChangeRegexp = regexp.MustCompile("^changed target branch from `(.*?)` to `(.*?)`$")

func (g *GitlabDownloader) convertNoteToComment(localIndex int64, note *gitlab.Note) *base.Comment {
	comment := &base.Comment{
		IssueIndex:  localIndex,
		Index:       int64(note.ID),
		PosterID:    int64(note.Author.ID),
		PosterName:  note.Author.Username,
		PosterEmail: note.Author.Email,
		Content:     note.Body,
		Created:     *note.CreatedAt,
		Meta:        map[string]any{},
	}

	// Try to find the underlying event of system notes.
	if note.System {
		if match := targetBranchChangeRegexp.FindStringSubmatch(note.Body); match != nil {
			comment.CommentType = issues_model.CommentTypeChangeTargetBranch.String()
			comment.Meta["OldRef"] = match[1]
			comment.Meta["NewRef"] = match[2]
		} else if strings.HasPrefix(note.Body, "enabled an automatic merge") {
			comment.CommentType = issues_model.CommentTypePRScheduledToAutoMerge.String()
		} else if note.Body == "canceled the automatic merge" {
			comment.CommentType = issues_model.CommentTypePRUnScheduledToAutoMerge.String()
		}
	}

	return comment
}

// GetPullRequests returns pull requests according page and perPage
func (g *GitlabDownloader) GetPullRequests(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}

	view := "simple"
	opt := &gitlab.ListProjectMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
		View: &view,
	}

	allPRs := make([]*base.PullRequest, 0, perPage)

	prs, _, err := g.client.MergeRequests.ListProjectMergeRequests(g.repoID, opt, nil, gitlab.WithContext(ctx))
	if err != nil {
		return nil, false, fmt.Errorf("error while listing merge requests: %w", err)
	}
	for _, simplePR := range prs {
		// Load merge request again by itself, as not all fields are populated in the ListProjectMergeRequests endpoint.
		// See https://gitlab.com/gitlab-org/gitlab/-/issues/29620
		pr, _, err := g.client.MergeRequests.GetMergeRequest(g.repoID, simplePR.IID, nil)
		if err != nil {
			return nil, false, fmt.Errorf("error while loading merge request: %w", err)
		}

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

		mergeCommitSHA := pr.MergeCommitSHA
		if mergeCommitSHA == "" {
			mergeCommitSHA = pr.SquashCommitSHA
		}

		var locked bool
		if pr.State == "locked" {
			locked = true
		}

		var milestone string
		if pr.Milestone != nil {
			milestone = pr.Milestone.Title
		}

		var reactions []*gitlab.AwardEmoji
		awardPage := 1
		for {
			awards, _, err := g.client.AwardEmoji.ListMergeRequestAwardEmoji(g.repoID, pr.IID, &gitlab.ListAwardEmojiOptions{Page: awardPage, PerPage: perPage}, gitlab.WithContext(ctx))
			if err != nil {
				return nil, false, fmt.Errorf("error while listing merge requests awards: %w", err)
			}

			reactions = append(reactions, awards...)

			if len(awards) < perPage {
				break
			}

			awardPage++
		}

		// Generate new PR Numbers by the known Issue Numbers, because they share the same number space in Gitea, but they are independent in Gitlab
		newPRNumber := g.iidResolver.generatePullRequestNumber(pr.IID)

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
			MergeCommitSHA: mergeCommitSHA,
			MergedTime:     mergeTime,
			IsLocked:       locked,
			Reactions:      g.awardsToReactions(reactions),
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
			IsDraft:      pr.Draft,
		})

		// SECURITY: Ensure that the PR is safe
		_ = CheckAndEnsureSafePR(allPRs[len(allPRs)-1], g.baseURL, g)
	}

	return allPRs, len(prs) < perPage, nil
}

// GetReviews returns pull requests review
func (g *GitlabDownloader) GetReviews(ctx context.Context, reviewable base.Reviewable) ([]*base.Review, error) {
	approvals, resp, err := g.client.MergeRequestApprovals.GetConfiguration(g.repoID, int(reviewable.GetForeignIndex()), gitlab.WithContext(ctx))
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

func (g *GitlabDownloader) awardsToReactions(awards []*gitlab.AwardEmoji) []*base.Reaction {
	result := make([]*base.Reaction, 0, len(awards))
	uniqCheck := make(container.Set[string])
	for _, award := range awards {
		uid := fmt.Sprintf("%s%d", award.Name, award.User.ID)
		if uniqCheck.Add(uid) {
			result = append(result, &base.Reaction{
				UserID:   int64(award.User.ID),
				UserName: award.User.Username,
				Content:  award.Name,
			})
		}
	}
	return result
}
