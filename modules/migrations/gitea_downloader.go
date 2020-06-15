package migrations

import (
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/structs"
	sdk "code.gitea.io/sdk/gitea"
	"errors"
	"net/http"
	"time"

	"context"
	"fmt"
	"net/url"
	"strings"
)

var (
	_ base.Downloader        = &GiteaDownloader{}
	_ base.DownloaderFactory = &GiteaDownloaderFactory{}
)

func init() {
	RegisterDownloaderFactory(&GiteaDownloaderFactory{})
}

// GiteaDownloaderFactory defines a gitea downloader factory
type GiteaDownloaderFactory struct {
}

// Match returns true if the migration remote URL matched this downloader factory
func (f *GiteaDownloaderFactory) Match(opts base.MigrateOptions) (bool, error) {
	var matched bool

	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return false, err
	}
	if strings.EqualFold(u.Host, "gitea.com") && opts.AuthUsername != "" {
		matched = true
	}

	return matched, nil
}

// New returns a Downloader related to this factory according MigrateOptions
func (f *GiteaDownloaderFactory) New(opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	baseURL := u.Scheme + "://" + u.Host
	repoNameSpace := strings.TrimPrefix(u.Path, "/")

	log.Trace("Create gitea downloader. BaseURL: %s RepoName: %s", baseURL, repoNameSpace)

	return NewGiteaDownloader(baseURL, repoNameSpace, opts.AuthUsername, opts.AuthPassword), nil
}

// GitServiceType returns the type of git service
func (f *GiteaDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.GiteaService
}

// GiteaDownloader implements a Downloader interface to get repository informations
// from gitlab via go-gitlab
// - issueCount is incremented in GetIssues() to ensure PR and Issue numbers do not overlap,
// because Gitea has individual Issue and Pull Request numbers.
// - issueSeen, working alongside issueCount, is checked in GetComments() to see whether we
// need to fetch the Issue or PR comments, as Gitea stores them separately.
type GiteaDownloader struct {
	ctx             context.Context
	client          *sdk.Client
	owner           string
	repoName        string
	fetchPRcomments bool
}

// NewGiteaDownloader creates a gitlab Downloader via gitlab API
//   Use either a username/password, personal token entered into the username field, or anonymous/public access
//   Note: Public access only allows very basic access
func NewGiteaDownloader(baseURL, repoPath, username, password string) *GiteaDownloader {
	var giteaClient *sdk.Client
	var err error

	if username != "" {
		if password == "" {
			giteaClient = sdk.NewClient(baseURL, username)
		} else {
			// NewClient initializes and returns a API client.
			giteaClient = sdk.NewClientWithHTTP(baseURL, &http.Client{})
			giteaClient.SetBasicAuth(username, password)
		}
	}

	_, err = giteaClient.ListMyRepos(sdk.ListReposOptions{})
	if err != nil {
		log.Trace("Error logging into gitlab: %v", err)
		return nil
	}

	// Grab and store project/repo ID here, due to issues using the URL escaped path
	var splitRepoPath = strings.Split(repoPath, "/")
	gr, err := giteaClient.GetRepo(splitRepoPath[0], splitRepoPath[1])
	if err != nil {
		log.Trace("Error retrieving project: %v", err)
		return nil
	}

	if gr == nil {
		log.Trace("Error getting project, project is nil")
		return nil
	}

	return &GiteaDownloader{
		ctx:      context.Background(),
		client:   giteaClient,
		repoName: gr.Name,
		owner:    gr.Owner.UserName,
	}
}

// SetContext set context
func (g *GiteaDownloader) SetContext(ctx context.Context) {
	g.ctx = ctx
}

// GetRepoInfo returns a repository information
func (g *GiteaDownloader) GetRepoInfo() (*base.Repository, error) {
	if g == nil {
		return nil, errors.New("error: GiteaDownloader is nil")
	}

	gr, err := g.client.GetRepo(g.owner, g.repoName)
	if err != nil {
		return nil, err
	}

	var owner string
	if gr.Owner == nil {
		log.Trace("gr.Owner is nil, using owner from Client")
		owner = g.owner
	} else {
		owner = gr.Owner.UserName
	}

	// convert gitlab repo to stand Repo
	return &base.Repository{
		Owner:       owner,
		Name:        gr.Name,
		IsPrivate:   gr.Private,
		Description: gr.Description,
		// TODO: Is this right?
		OriginalURL: gr.HTMLURL,
		CloneURL:    gr.CloneURL,
	}, nil
}

// GetTopics return gitlab topics
func (g *GiteaDownloader) GetTopics() ([]string, error) {
	if g == nil {
		return nil, errors.New("error: GiteaDownloader is nil")
	}
	ts, err := g.client.ListRepoTopics(g.owner, g.repoName, sdk.ListRepoTopicsOptions{})
	if err != nil {
		return nil, err
	}
	return ts, nil
}

// GetMilestones returns milestones
func (g *GiteaDownloader) GetMilestones() ([]*base.Milestone, error) {
	if g == nil {
		return nil, errors.New("error: GiteaDownloader is nil")
	}
	ms, err := g.client.ListRepoMilestones(g.owner, g.repoName, sdk.ListMilestoneOption{State: sdk.StateAll})
	if err != nil {
		return nil, err
	}
	var milestones []*base.Milestone

	now := time.Now()
	for _, m := range ms {
		milestones = append(milestones, &base.Milestone{
			Title:       m.Title,
			Description: m.Description,
			Deadline:    m.Deadline,
			State:       string(m.State),
			// TODO
			Created: now,
			// TODO
			Updated: &now,
			Closed:  m.Closed,
		})
	}
	return milestones, nil
}

func (g *GiteaDownloader) convertGiteaLabel(label *sdk.Label) *base.Label {
	return &base.Label{
		Name:        label.Name,
		Color:       label.Color,
		Description: label.Description,
	}
}

// GetLabels returns labels
func (g *GiteaDownloader) GetLabels() ([]*base.Label, error) {
	if g == nil {
		return nil, errors.New("error: GiteaDownloader is nil")
	}
	ls, err := g.client.ListRepoLabels(g.owner, g.repoName, sdk.ListLabelsOptions{})
	if err != nil {
		return nil, err
	}
	var labels []*base.Label
	for _, label := range ls {
		labels = append(labels, g.convertGiteaLabel(label))
	}
	return labels, nil
}

func (g *GiteaDownloader) convertGiteaRelease(rel *sdk.Release) *base.Release {

	r := &base.Release{
		TagName:         rel.TagName,
		TargetCommitish: rel.Target,
		Name:            rel.Title,
		Body:            rel.Note,
		Draft:           rel.IsDraft,
		Prerelease:      rel.IsPrerelease,
		PublisherID:     rel.Publisher.ID,
		PublisherName:   rel.Publisher.UserName,
		PublisherEmail:  rel.Publisher.Email,
		Created:         rel.CreatedAt,
		Published:       rel.PublishedAt,
	}

	for _, asset := range rel.Attachments {
		r.Assets = append(r.Assets, base.ReleaseAsset{
			URL:  asset.DownloadURL,
			Name: asset.Name,
			// TODO
			ContentType: nil,
		})
	}
	return r
}

// GetReleases returns releases
func (g *GiteaDownloader) GetReleases() ([]*base.Release, error) {
	ls, err := g.client.ListReleases(g.owner, g.repoName, sdk.ListReleasesOptions{})
	if err != nil {
		return nil, err
	}
	var releases []*base.Release
	for _, release := range ls {
		releases = append(releases, g.convertGiteaRelease(release))
	}
	return releases, nil
}

func (g *GiteaDownloader) GetReactionsToIssue(issueNumber int64) ([]*base.Reaction, error) {
	var allReactions []*base.Reaction

	rs, err := g.client.GetIssueReactions(g.owner, g.repoName, issueNumber)

	if err != nil {
		return nil, err
	}

	for _, r := range rs {
		allReactions = append(allReactions, &base.Reaction{
			UserID:   r.User.ID,
			UserName: r.User.UserName,
			Content:  r.Reaction,
		})
	}

	return allReactions, nil
}

// GetIssues returns issues according start and limit
//   Note: issue label description and colors are not supported by the go-gitlab library at this time
func (g *GiteaDownloader) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	opt := sdk.ListIssueOption{
		State: sdk.StateAll,
		Type:  sdk.IssueTypeIssue,
		ListOptions: sdk.ListOptions{
			Page:     page,
			PageSize: perPage,
		},
	}

	issues, err := g.client.ListRepoIssues(g.owner, g.repoName, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing issues: %v", err)
	}
	var allIssues []*base.Issue
	for _, issue := range issues {

		issueNumber := issue.Index
		var labels []*base.Label

		for _, l := range issue.Labels {
			labels = append(labels, &base.Label{
				Name:        l.Name,
				Color:       l.Color,
				Description: l.Description,
			})
		}

		var milestone string
		if issue.Milestone != nil {
			milestone = issue.Milestone.Title
		}

		reactions, _ := g.GetReactionsToIssue(issueNumber)

		allIssues = append(allIssues, &base.Issue{
			Number:      issueNumber,
			PosterID:    issue.Poster.ID,
			PosterName:  issue.Poster.UserName,
			PosterEmail: issue.Poster.Email,
			Title:       issue.Title,
			Content:     issue.Body,
			Milestone:   milestone,
			State:       string(issue.State),
			// TODO
			IsLocked:    false,
			Created:     issue.Created,
			Updated:     issue.Updated,
			Closed:      issue.Closed,
			Labels:      labels,
			Reactions:   reactions,
		})
	}

	return allIssues, len(issues) < perPage, nil
}

func (g *GiteaDownloader) GetReactionsToComment(commentID int64) ([]*base.Reaction, error) {
	var allReactions []*base.Reaction

	rs, err := g.client.GetIssueCommentReactions(g.owner, g.repoName, commentID)

	if err != nil {
		return nil, err
	}

	for _, r := range rs {
		allReactions = append(allReactions, &base.Reaction{
			UserID:   r.User.ID,
			UserName: r.User.UserName,
			Content:  r.Reaction,
		})
	}

	return allReactions, nil
}

// GetComments returns comments according issueNumber
func (g *GiteaDownloader) GetComments(issueNumber int64) ([]*base.Comment, error) {
	var allComments []*base.Comment

	comments, err := g.client.ListIssueComments(g.owner, g.repoName, issueNumber, sdk.ListIssueCommentOptions{})

	if err != nil {
		return nil, fmt.Errorf("error while listing comments: %v %v %v", g.owner, g.repoName, err)
	}
	for _, comment := range comments {
		reactions, _ := g.GetReactionsToComment(comment.ID)

		allComments = append(allComments, &base.Comment{
			IssueIndex:  issueNumber,
			PosterID:    comment.Poster.ID,
			PosterName:  comment.Poster.UserName,
			PosterEmail: comment.Poster.Email,
			Content:     comment.Body,
			Created:     comment.Created,
			Updated:     comment.Updated,
			Reactions:   reactions,
		})
	}
	return allComments, nil
}

func (g *GiteaDownloader) convertGiteaPullRequestBranch(branchInfo *sdk.PRBranchInfo) base.PullRequestBranch {
	return base.PullRequestBranch{
		Ref:       branchInfo.Ref,
		SHA:       branchInfo.Sha,
		RepoName:  branchInfo.Repository.Name,
		OwnerName: branchInfo.Repository.Owner.UserName,
		CloneURL:  branchInfo.Repository.CloneURL,
	}
}

// GetPullRequests returns pull requests according page and perPage
func (g *GiteaDownloader) GetPullRequests(page, perPage int) ([]*base.PullRequest, error) {
	opt := sdk.ListPullRequestsOptions{
		State: sdk.StateAll,
		ListOptions: sdk.ListOptions{
			Page:     page,
			PageSize: perPage,
		},
	}

	// Set fetchPRcomments to true here, so PR comments are fetched instead of Issue comments
	g.fetchPRcomments = true

	var allPRs []*base.PullRequest

	prs, err := g.client.ListRepoPullRequests(g.owner, g.repoName, opt)
	if err != nil {
		return nil, fmt.Errorf("error while listing merge requests: %v", err)
	}
	for _, pr := range prs {

		issueNumber := pr.Index
		var labels []*base.Label

		for _, l := range pr.Labels {
			labels = append(labels, g.convertGiteaLabel(l))
		}

		var mergeCommitSHA string
		if pr.MergedCommitID != nil {
			mergeCommitSHA = *pr.MergedCommitID
		}
		var updated time.Time
		if pr.Updated != nil {
			updated = *pr.Updated
		}

		reactions, _ := g.GetReactionsToIssue(issueNumber)
		var assignee string
		if pr.Assignee != nil {
			assignee = pr.Assignee.UserName
		}

		allPRs = append(allPRs, &base.PullRequest{
			Number:         pr.Index,
			OriginalNumber: 0,
			Title:          pr.Title,
			PosterName:     pr.Poster.UserName,
			PosterID:       pr.Poster.ID,
			PosterEmail:    pr.Poster.Email,
			Content:        pr.Body,
			Milestone:      pr.Milestone.Title,
			State:          string(pr.State),
			Created:        *pr.Created,
			Updated:        updated,
			Closed:         pr.Closed,
			Labels:         labels,
			PatchURL:       pr.PatchURL,
			Merged:         pr.HasMerged,
			MergedTime:     pr.Merged,
			MergeCommitSHA: mergeCommitSHA,
			Head:           g.convertGiteaPullRequestBranch(pr.Head),
			Base:           g.convertGiteaPullRequestBranch(pr.Base),
			Assignee:       assignee,
			// TODO
			Assignees:      nil,
			// TODO
			IsLocked:       false,
			Reactions:      reactions,
		})
	}

	return allPRs, nil
}

func mapReviewState(stateType sdk.ReviewStateType) string {
	switch stateType {
	case sdk.ReviewStatePending:
		return base.ReviewStatePending
	case sdk.ReviewStateApproved:
		return base.ReviewStateApproved
	case sdk.ReviewStateRequestChanges:
		return base.ReviewStateChangesRequested
	case sdk.ReviewStateComment:
		return base.ReviewStateCommented
	default:
		return ""
	}
}

// GetReviews returns pull requests review
func (g *GiteaDownloader) GetReviews(pullRequestNumber int64) ([]*base.Review, error) {
	prrs, err := g.client.ListPullReviews(g.owner, g.repoName, pullRequestNumber, sdk.ListPullReviewsOptions{})
	if err != nil {
		return nil, err
	}

	var allPRReviews []*base.Review

	for _, prr := range prrs {
		id := prr.ID
		var currentReviewComments []*base.ReviewComment

		prrcs, err := g.client.ListPullReviewComments(g.owner, g.repoName, pullRequestNumber, id, sdk.ListPullReviewsCommentsOptions{})
		if err != nil {
			currentReviewComments = make([]*base.ReviewComment, 0)
		} else {
			for _, prrc := range prrcs {
				id := prrc.ID
				reactions, _ := g.GetReactionsToComment(id)

				currentReviewComments = append(currentReviewComments, &base.ReviewComment{
					ID:        id,
					// TODO: Find out
					InReplyTo: 0,
					Content:   prrc.Body,
					TreePath:  prrc.Path,
					DiffHunk:  prrc.DiffHunk,
					Position:  int(prrc.LineNum),
					CommitID:  prrc.CommitID,
					PosterID:  prrc.Reviewer.ID,
					Reactions: reactions,
					CreatedAt: prrc.Created,
					UpdatedAt: prrc.Updated,
				})
			}
		}

		allPRReviews = append(allPRReviews, &base.Review{
			ID:           prr.ID,
			IssueIndex:   pullRequestNumber,
			ReviewerID:   prr.Reviewer.ID,
			ReviewerName: prr.Reviewer.UserName,
			Official:     prr.Official,
			CommitID:     prr.CommitID,
			Content:      prr.Body,
			CreatedAt:    prr.Submitted,
			State:        mapReviewState(prr.State),
			Comments:     currentReviewComments,
		})
	}

	return allPRReviews, nil
}
