// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

// Match returns ture if the migration remote URL matched this downloader factory
//   To allow self-hosting Gitlab instances, this matches based on the Host or a '#gitlab' fragment
func (f *GitlabDownloaderFactory) Match(opts base.MigrateOptions) (bool, error) {
	var matched bool

	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return false, err
	}
	if strings.EqualFold(u.Host, "gitlab.com") && opts.AuthUsername != "" {
		matched = true
	}
	if strings.EqualFold(u.Fragment, "gitlab") && opts.AuthUsername != "" {
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

	//fields := strings.Split(u.Path, "/")
	//oldOwner := fields[1]
	//oldName := strings.TrimSuffix(fields[2], ".git")

	baseURL := u.Scheme + "://" + u.Host
	repoNameSpace := strings.TrimPrefix(u.Path, "/")

	log.Trace("Create gitlab downloader. baseURL: %s Token: %s RepoName: %s", baseURL, opts.AuthUsername, repoNameSpace)
	log.Trace("opts.CloneAddr %v", opts.CloneAddr)

	return NewGitlabDownloader(baseURL, repoNameSpace, opts.AuthUsername, opts.AuthPassword), nil
}

// GitServiceType returns the type of git service
func (f *GitlabDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.GithubService
}

// GitlabDownloader implements a Downloader interface to get repository informations
// from gitlab via go-gitlab
type GitlabDownloader struct {
	ctx      context.Context
	client   *gitlab.Client
	repoPath string
}

// NewGitlabDownloader creates a github Downloader via gitlab API
func NewGitlabDownloader(baseURL, repoPath, username, password string) *GitlabDownloader {
	var downloader = GitlabDownloader{
		ctx:      context.Background(),
		repoPath: repoPath,
	}

	var client *http.Client
	/*
		gitlabClient := gitlab.NewClient(client, username)
		gitlabClient.SetBaseURL(baseURL)
	*/

	gitlabClient, err := gitlab.NewBasicAuthClient(client, baseURL, username, password)
	if err != nil {
		log.Trace("Error logging into gitlab: %v", err)
		return nil
	}

	downloader.client = gitlabClient

	return &downloader
}

// GetRepoInfo returns a repository information
func (g *GitlabDownloader) GetRepoInfo() (*base.Repository, error) {
	gr, _, err := g.client.Projects.GetProject(g.repoPath, nil, nil)
	if err != nil {
		return nil, err
	}
	// convert github repo to stand Repo
	return &base.Repository{
		//Owner:       gr.Owner.Username,
		Name:        gr.Name,
		IsPrivate:   (!gr.Public),
		Description: gr.Description,
		OriginalURL: gr.WebURL,
		CloneURL:    gr.HTTPURLToRepo,
	}, nil
}

// GetTopics return github topics
func (g *GitlabDownloader) GetTopics() ([]string, error) {
	//r, _, err := g.client.Repositories.Get(g.ctx, g.repoOwner, g.repoName)
	gr, _, err := g.client.Projects.GetProject(g.repoPath, nil, nil)
	if err != nil {
		return nil, err
	}
	return gr.TagList, err
}

// GetMilestones returns milestones
func (g *GitlabDownloader) GetMilestones() ([]*base.Milestone, error) {
	var perPage = 100
	var state = "all"
	var milestones = make([]*base.Milestone, 0, perPage)
	for i := 1; ; i++ {
		ms, _, err := g.client.Milestones.ListMilestones(g.repoPath, &gitlab.ListMilestonesOptions{
			State: &state,
			ListOptions: gitlab.ListOptions{
				Page:    i,
				PerPage: perPage,
			}}, nil)
		if err != nil {
			return nil, err
		}
		var milestones = make([]*base.Milestone, 0)

		for _, m := range ms {
			var desc string
			if m.Description != "" {
				desc = m.Description
			}
			var state = "open"
			if m.State != "" {
				state = m.State
			}
			milestones = append(milestones, &base.Milestone{
				Title:       m.Title,
				Description: desc,
				//Deadline:    m.DueDate,
				State:   state,
				Created: *m.CreatedAt,
				Updated: m.UpdatedAt,
				Closed:  m.UpdatedAt,
			})
		}
		if len(ms) < perPage {
			break
		}
	}
	return milestones, nil
}

// GetLabels returns labels
func (g *GitlabDownloader) GetLabels() ([]*base.Label, error) {
	var perPage = 100
	var labels = make([]*base.Label, 0, perPage)
	for i := 1; ; i++ {
		ls, _, err := g.client.Labels.ListLabels(g.repoPath, &gitlab.ListLabelsOptions{
			Page:    i,
			PerPage: perPage,
		}, nil)
		if err != nil {
			return nil, err
		}
		for _, label := range ls {
			baseLabel := &base.Label{
				Name:        label.Name,
				Color:       strings.TrimLeft(label.Color, "#)"),
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
		//Draft:           *rel.Draft,
		//Prerelease:      *rel.Prerelease,
		Created:       *rel.CreatedAt,
		PublisherID:   int64(rel.Author.ID),
		PublisherName: rel.Author.Name,
		//PublisherEmail:  rel.Author.Email,
		//Published: rel.PublishedAt.Time,
	}

	for k, asset := range rel.Assets.Links {
		u, _ := url.Parse(asset.URL)
		r.Assets = append(r.Assets, base.ReleaseAsset{
			URL:         u.String(),
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
		ls, _, err := g.client.Releases.ListReleases(g.repoPath, &gitlab.ListReleasesOptions{
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

	issues, _, err := g.client.Issues.ListProjectIssues(g.repoPath, opt, nil)
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
			PosterName: issue.Author.Name,
			Content:    issue.Description,
			Milestone:  milestone,
			State:      issue.State,
			Created:    *issue.CreatedAt,
			Labels:     labels,
			Closed:     issue.ClosedAt,
			IsLocked:   issue.DiscussionLocked,
		})
	}

	return allIssues, len(issues) < perPage, nil
}

// GetComments returns comments according issueNumber
func (g *GitlabDownloader) GetComments(issueNumber int64) ([]*base.Comment, error) {
	var allComments = make([]*base.Comment, 0, 100)
	opt := &gitlab.ListIssueDiscussionsOptions{
		Page:    1,
		PerPage: 100,
	}
	for {
		comments, resp, err := g.client.Discussions.ListIssueDiscussions(url.PathEscape(g.repoPath), int(issueNumber), opt, nil)
		if err != nil {
			return nil, fmt.Errorf("error while listing comments: %v %v", g.repoPath, err)
		}
		for _, comment := range comments {
			// Flatten comment threads
			if !comment.IndividualNote {
				for _, note := range comment.Notes {
					allComments = append(allComments, &base.Comment{
						IssueIndex:  issueNumber,
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
					IssueIndex:  issueNumber,
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
		opt.Page = resp.NextPage
	}
	return allComments, nil
}

// GetPullRequests returns pull requests according page and perPage
func (g *GitlabDownloader) GetPullRequests(page, perPage int) ([]*base.PullRequest, error) {
	//state := "all"
	//sort := "created"

	opt := &gitlab.ListProjectMergeRequestsOptions{
		//State: &state,
		//Sort:  &sort,
		ListOptions: gitlab.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
	}

	var allPRs = make([]*base.PullRequest, 0, perPage)

	prs, _, err := g.client.MergeRequests.ListProjectMergeRequests(g.repoPath, opt, nil)
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
		// pr.Merged is not valid, so use MergedAt to test if it's merged
		if pr.MergedAt != nil {
			merged = true
		}

		/*
			var (
				headRepoName string
				cloneURL     string
				headRef      string
				headSHA      string
			)
				if pr.Head.Repo != nil {
					if pr.Head.Repo.Name != nil {
						headRepoName = *pr.Head.Repo.Name
					}
					if pr.Head.Repo.CloneURL != nil {
						cloneURL = *pr.Head.Repo.CloneURL
					}
				}
				if pr.Head.Ref != nil {
					headRef = *pr.Head.Ref
				}
				if pr.Head.SHA != nil {
					headSHA = *pr.Head.SHA
				}
				var mergeCommitSHA string
				if pr.MergeCommitSHA != nil {
					mergeCommitSHA = *pr.MergeCommitSHA
				}

				var headUserName string
				if pr.Head.User != nil && pr.Head.User.Login != nil {
					headUserName = *pr.Head.User.Login
				}
		*/

		var milestone string
		if pr.Milestone != nil {
			milestone = pr.Milestone.Title
		}

		allPRs = append(allPRs, &base.PullRequest{
			Title:          pr.Title,
			Number:         int64(pr.IID),
			PosterName:     pr.Author.Name,
			PosterID:       int64(pr.Author.ID),
			Content:        pr.Description,
			Milestone:      milestone,
			State:          pr.State,
			Created:        *pr.CreatedAt,
			Closed:         pr.ClosedAt,
			Labels:         labels,
			Merged:         merged,
			MergeCommitSHA: pr.MergeCommitSHA,
			MergedTime:     pr.MergedAt,
			IsLocked:       pr.DiscussionLocked,
			Head: base.PullRequestBranch{
				Ref:       pr.Reference,
				SHA:       pr.DiffRefs.HeadSha,
				RepoName:  pr.Reference,
				OwnerName: pr.Author.Username,
				CloneURL:  pr.WebURL,
			},
			Base: base.PullRequestBranch{
				Ref:       pr.Reference,
				SHA:       pr.DiffRefs.BaseSha,
				RepoName:  pr.Reference,
				OwnerName: pr.Author.Username,
			},
			PatchURL: pr.WebURL,
		})
	}

	return allPRs, nil
}
