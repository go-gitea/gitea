// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/proxy"
	"code.gitea.io/gitea/modules/structs"
)

var (
	_ base.Downloader        = &CodebaseDownloader{}
	_ base.DownloaderFactory = &CodebaseDownloaderFactory{}
)

func init() {
	RegisterDownloaderFactory(&CodebaseDownloaderFactory{})
}

// CodebaseDownloaderFactory defines a downloader factory
type CodebaseDownloaderFactory struct{}

// New returns a downloader related to this factory according MigrateOptions
func (f *CodebaseDownloaderFactory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}
	u.User = nil

	fields := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(fields) != 2 {
		return nil, fmt.Errorf("invalid path: %s", u.Path)
	}
	project := fields[0]
	repoName := strings.TrimSuffix(fields[1], ".git")

	log.Trace("Create Codebase downloader. BaseURL: %v RepoName: %s", u, repoName)

	return NewCodebaseDownloader(ctx, u, project, repoName, opts.AuthUsername, opts.AuthPassword), nil
}

// GitServiceType returns the type of git service
func (f *CodebaseDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.CodebaseService
}

type codebaseUser struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CodebaseDownloader implements a Downloader interface to get repository information
// from Codebase
type CodebaseDownloader struct {
	base.NullDownloader
	client        *http.Client
	baseURL       *url.URL
	projectURL    *url.URL
	project       string
	repoName      string
	maxIssueIndex int64
	userMap       map[int64]*codebaseUser
	commitMap     map[string]string
}

// NewCodebaseDownloader creates a new downloader
func NewCodebaseDownloader(_ context.Context, projectURL *url.URL, project, repoName, username, password string) *CodebaseDownloader {
	baseURL, _ := url.Parse("https://api3.codebasehq.com")

	downloader := &CodebaseDownloader{
		baseURL:    baseURL,
		projectURL: projectURL,
		project:    project,
		repoName:   repoName,
		client: &http.Client{
			Transport: &http.Transport{
				Proxy: func(req *http.Request) (*url.URL, error) {
					if len(username) > 0 && len(password) > 0 {
						req.SetBasicAuth(username, password)
					}
					return proxy.Proxy()(req)
				},
			},
		},
		userMap:   make(map[int64]*codebaseUser),
		commitMap: make(map[string]string),
	}

	log.Trace("Create Codebase downloader. BaseURL: %s Project: %s RepoName: %s", baseURL, project, repoName)
	return downloader
}

// String implements Stringer
func (d *CodebaseDownloader) String() string {
	return fmt.Sprintf("migration from codebase server %s %s/%s", d.baseURL, d.project, d.repoName)
}

func (d *CodebaseDownloader) LogString() string {
	if d == nil {
		return "<CodebaseDownloader nil>"
	}
	return fmt.Sprintf("<CodebaseDownloader %s %s/%s>", d.baseURL, d.project, d.repoName)
}

// FormatCloneURL add authentication into remote URLs
func (d *CodebaseDownloader) FormatCloneURL(opts base.MigrateOptions, remoteAddr string) (string, error) {
	return opts.CloneAddr, nil
}

func (d *CodebaseDownloader) callAPI(ctx context.Context, endpoint string, parameter map[string]string, result any) error {
	u, err := d.baseURL.Parse(endpoint)
	if err != nil {
		return err
	}

	if parameter != nil {
		query := u.Query()
		for k, v := range parameter {
			query.Set(k, v)
		}
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Accept", "application/xml")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return xml.NewDecoder(resp.Body).Decode(&result)
}

// GetRepoInfo returns repository information
// https://support.codebasehq.com/kb/projects
func (d *CodebaseDownloader) GetRepoInfo(ctx context.Context) (*base.Repository, error) {
	var rawRepository struct {
		XMLName     xml.Name `xml:"repository"`
		Name        string   `xml:"name"`
		Description string   `xml:"description"`
		Permalink   string   `xml:"permalink"`
		CloneURL    string   `xml:"clone-url"`
		Source      string   `xml:"source"`
	}

	err := d.callAPI(
		ctx,
		fmt.Sprintf("/%s/%s", d.project, d.repoName),
		nil,
		&rawRepository,
	)
	if err != nil {
		return nil, err
	}

	return &base.Repository{
		Name:        rawRepository.Name,
		Description: rawRepository.Description,
		CloneURL:    rawRepository.CloneURL,
		OriginalURL: d.projectURL.String(),
	}, nil
}

// GetMilestones returns milestones
// https://support.codebasehq.com/kb/tickets-and-milestones/milestones
func (d *CodebaseDownloader) GetMilestones(ctx context.Context) ([]*base.Milestone, error) {
	var rawMilestones struct {
		XMLName            xml.Name `xml:"ticketing-milestone"`
		Type               string   `xml:"type,attr"`
		TicketingMilestone []struct {
			Text string `xml:",chardata"`
			ID   struct {
				Value int64  `xml:",chardata"`
				Type  string `xml:"type,attr"`
			} `xml:"id"`
			Identifier string `xml:"identifier"`
			Name       string `xml:"name"`
			Deadline   struct {
				Value string `xml:",chardata"`
				Type  string `xml:"type,attr"`
			} `xml:"deadline"`
			Description string `xml:"description"`
			Status      string `xml:"status"`
		} `xml:"ticketing-milestone"`
	}

	err := d.callAPI(
		ctx,
		fmt.Sprintf("/%s/milestones", d.project),
		nil,
		&rawMilestones,
	)
	if err != nil {
		return nil, err
	}

	milestones := make([]*base.Milestone, 0, len(rawMilestones.TicketingMilestone))
	for _, milestone := range rawMilestones.TicketingMilestone {
		var deadline *time.Time
		if len(milestone.Deadline.Value) > 0 {
			if val, err := time.Parse("2006-01-02", milestone.Deadline.Value); err == nil {
				deadline = &val
			}
		}

		closed := deadline
		state := "closed"
		if milestone.Status == "active" {
			closed = nil
			state = ""
		}

		milestones = append(milestones, &base.Milestone{
			Title:    milestone.Name,
			Deadline: deadline,
			Closed:   closed,
			State:    state,
		})
	}
	return milestones, nil
}

// GetLabels returns labels
// https://support.codebasehq.com/kb/tickets-and-milestones/statuses-priorities-and-categories
func (d *CodebaseDownloader) GetLabels(ctx context.Context) ([]*base.Label, error) {
	var rawTypes struct {
		XMLName       xml.Name `xml:"ticketing-types"`
		Type          string   `xml:"type,attr"`
		TicketingType []struct {
			ID struct {
				Value int64  `xml:",chardata"`
				Type  string `xml:"type,attr"`
			} `xml:"id"`
			Name string `xml:"name"`
		} `xml:"ticketing-type"`
	}

	err := d.callAPI(
		ctx,
		fmt.Sprintf("/%s/tickets/types", d.project),
		nil,
		&rawTypes,
	)
	if err != nil {
		return nil, err
	}

	labels := make([]*base.Label, 0, len(rawTypes.TicketingType))
	for _, label := range rawTypes.TicketingType {
		labels = append(labels, &base.Label{
			Name:  label.Name,
			Color: "ffffff",
		})
	}
	return labels, nil
}

type codebaseIssueContext struct {
	Comments []*base.Comment
}

// GetIssues returns issues, limits are not supported
// https://support.codebasehq.com/kb/tickets-and-milestones
// https://support.codebasehq.com/kb/tickets-and-milestones/updating-tickets
func (d *CodebaseDownloader) GetIssues(ctx context.Context, _, _ int) ([]*base.Issue, bool, error) {
	var rawIssues struct {
		XMLName xml.Name `xml:"tickets"`
		Type    string   `xml:"type,attr"`
		Ticket  []struct {
			TicketID struct {
				Value int64  `xml:",chardata"`
				Type  string `xml:"type,attr"`
			} `xml:"ticket-id"`
			Summary    string `xml:"summary"`
			TicketType string `xml:"ticket-type"`
			ReporterID struct {
				Value int64  `xml:",chardata"`
				Type  string `xml:"type,attr"`
			} `xml:"reporter-id"`
			Reporter string `xml:"reporter"`
			Type     struct {
				Name string `xml:"name"`
			} `xml:"type"`
			Status struct {
				TreatAsClosed struct {
					Value bool   `xml:",chardata"`
					Type  string `xml:"type,attr"`
				} `xml:"treat-as-closed"`
			} `xml:"status"`
			Milestone struct {
				Name string `xml:"name"`
			} `xml:"milestone"`
			UpdatedAt struct {
				Value time.Time `xml:",chardata"`
				Type  string    `xml:"type,attr"`
			} `xml:"updated-at"`
			CreatedAt struct {
				Value time.Time `xml:",chardata"`
				Type  string    `xml:"type,attr"`
			} `xml:"created-at"`
		} `xml:"ticket"`
	}

	err := d.callAPI(
		ctx,
		fmt.Sprintf("/%s/tickets", d.project),
		nil,
		&rawIssues,
	)
	if err != nil {
		return nil, false, err
	}

	issues := make([]*base.Issue, 0, len(rawIssues.Ticket))
	for _, issue := range rawIssues.Ticket {
		var notes struct {
			XMLName    xml.Name `xml:"ticket-notes"`
			Type       string   `xml:"type,attr"`
			TicketNote []struct {
				Content   string `xml:"content"`
				CreatedAt struct {
					Value time.Time `xml:",chardata"`
					Type  string    `xml:"type,attr"`
				} `xml:"created-at"`
				UpdatedAt struct {
					Value time.Time `xml:",chardata"`
					Type  string    `xml:"type,attr"`
				} `xml:"updated-at"`
				ID struct {
					Value int64  `xml:",chardata"`
					Type  string `xml:"type,attr"`
				} `xml:"id"`
				UserID struct {
					Value int64  `xml:",chardata"`
					Type  string `xml:"type,attr"`
				} `xml:"user-id"`
			} `xml:"ticket-note"`
		}
		err := d.callAPI(
			ctx,
			fmt.Sprintf("/%s/tickets/%d/notes", d.project, issue.TicketID.Value),
			nil,
			&notes,
		)
		if err != nil {
			return nil, false, err
		}
		comments := make([]*base.Comment, 0, len(notes.TicketNote))
		for _, note := range notes.TicketNote {
			if len(note.Content) == 0 {
				continue
			}
			poster := d.tryGetUser(ctx, note.UserID.Value)
			comments = append(comments, &base.Comment{
				IssueIndex:  issue.TicketID.Value,
				Index:       note.ID.Value,
				PosterID:    poster.ID,
				PosterName:  poster.Name,
				PosterEmail: poster.Email,
				Content:     note.Content,
				Created:     note.CreatedAt.Value,
				Updated:     note.UpdatedAt.Value,
			})
		}
		if len(comments) == 0 {
			comments = append(comments, &base.Comment{})
		}

		state := "open"
		if issue.Status.TreatAsClosed.Value {
			state = "closed"
		}
		poster := d.tryGetUser(ctx, issue.ReporterID.Value)
		issues = append(issues, &base.Issue{
			Title:       issue.Summary,
			Number:      issue.TicketID.Value,
			PosterName:  poster.Name,
			PosterEmail: poster.Email,
			Content:     comments[0].Content,
			Milestone:   issue.Milestone.Name,
			State:       state,
			Created:     issue.CreatedAt.Value,
			Updated:     issue.UpdatedAt.Value,
			Labels: []*base.Label{
				{Name: issue.Type.Name},
			},
			ForeignIndex: issue.TicketID.Value,
			Context: codebaseIssueContext{
				Comments: comments[1:],
			},
		})

		if d.maxIssueIndex < issue.TicketID.Value {
			d.maxIssueIndex = issue.TicketID.Value
		}
	}

	return issues, true, nil
}

// GetComments returns comments
func (d *CodebaseDownloader) GetComments(_ context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	context, ok := commentable.GetContext().(codebaseIssueContext)
	if !ok {
		return nil, false, fmt.Errorf("unexpected context: %+v", commentable.GetContext())
	}

	return context.Comments, true, nil
}

// GetPullRequests returns pull requests
// https://support.codebasehq.com/kb/repositories/merge-requests
func (d *CodebaseDownloader) GetPullRequests(ctx context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	var rawMergeRequests struct {
		XMLName      xml.Name `xml:"merge-requests"`
		Type         string   `xml:"type,attr"`
		MergeRequest []struct {
			ID struct {
				Value int64  `xml:",chardata"`
				Type  string `xml:"type,attr"`
			} `xml:"id"`
		} `xml:"merge-request"`
	}

	err := d.callAPI(
		ctx,
		fmt.Sprintf("/%s/%s/merge_requests", d.project, d.repoName),
		map[string]string{
			"query":  `"Target Project" is "` + d.repoName + `"`,
			"offset": strconv.Itoa((page - 1) * perPage),
			"count":  strconv.Itoa(perPage),
		},
		&rawMergeRequests,
	)
	if err != nil {
		return nil, false, err
	}

	pullRequests := make([]*base.PullRequest, 0, len(rawMergeRequests.MergeRequest))
	for i, mr := range rawMergeRequests.MergeRequest {
		var rawMergeRequest struct {
			XMLName xml.Name `xml:"merge-request"`
			ID      struct {
				Value int64  `xml:",chardata"`
				Type  string `xml:"type,attr"`
			} `xml:"id"`
			SourceRef string `xml:"source-ref"` // NOTE: from the documentation these are actually just branches NOT full refs
			TargetRef string `xml:"target-ref"` // NOTE: from the documentation these are actually just branches NOT full refs
			Subject   string `xml:"subject"`
			Status    string `xml:"status"`
			UserID    struct {
				Value int64  `xml:",chardata"`
				Type  string `xml:"type,attr"`
			} `xml:"user-id"`
			CreatedAt struct {
				Value time.Time `xml:",chardata"`
				Type  string    `xml:"type,attr"`
			} `xml:"created-at"`
			UpdatedAt struct {
				Value time.Time `xml:",chardata"`
				Type  string    `xml:"type,attr"`
			} `xml:"updated-at"`
			Comments struct {
				Type    string `xml:"type,attr"`
				Comment []struct {
					Content string `xml:"content"`
					ID      struct {
						Value int64  `xml:",chardata"`
						Type  string `xml:"type,attr"`
					} `xml:"id"`
					UserID struct {
						Value int64  `xml:",chardata"`
						Type  string `xml:"type,attr"`
					} `xml:"user-id"`
					Action struct {
						Value string `xml:",chardata"`
						Nil   string `xml:"nil,attr"`
					} `xml:"action"`
					CreatedAt struct {
						Value time.Time `xml:",chardata"`
						Type  string    `xml:"type,attr"`
					} `xml:"created-at"`
				} `xml:"comment"`
			} `xml:"comments"`
		}
		err := d.callAPI(
			ctx,
			fmt.Sprintf("/%s/%s/merge_requests/%d", d.project, d.repoName, mr.ID.Value),
			nil,
			&rawMergeRequest,
		)
		if err != nil {
			return nil, false, err
		}

		number := d.maxIssueIndex + int64(i) + 1

		state := "open"
		merged := false
		var closeTime *time.Time
		var mergedTime *time.Time
		if rawMergeRequest.Status != "new" {
			state = "closed"
			closeTime = &rawMergeRequest.UpdatedAt.Value
		}

		comments := make([]*base.Comment, 0, len(rawMergeRequest.Comments.Comment))
		for _, comment := range rawMergeRequest.Comments.Comment {
			if len(comment.Content) == 0 {
				if comment.Action.Value == "merging" {
					merged = true
					mergedTime = &comment.CreatedAt.Value
				}
				continue
			}
			poster := d.tryGetUser(ctx, comment.UserID.Value)
			comments = append(comments, &base.Comment{
				IssueIndex:  number,
				Index:       comment.ID.Value,
				PosterID:    poster.ID,
				PosterName:  poster.Name,
				PosterEmail: poster.Email,
				Content:     comment.Content,
				Created:     comment.CreatedAt.Value,
				Updated:     comment.CreatedAt.Value,
			})
		}
		if len(comments) == 0 {
			comments = append(comments, &base.Comment{})
		}

		poster := d.tryGetUser(ctx, rawMergeRequest.UserID.Value)

		pullRequests = append(pullRequests, &base.PullRequest{
			Title:       rawMergeRequest.Subject,
			Number:      number,
			PosterName:  poster.Name,
			PosterEmail: poster.Email,
			Content:     comments[0].Content,
			State:       state,
			Created:     rawMergeRequest.CreatedAt.Value,
			Updated:     rawMergeRequest.UpdatedAt.Value,
			Closed:      closeTime,
			Merged:      merged,
			MergedTime:  mergedTime,
			Head: base.PullRequestBranch{
				Ref:      rawMergeRequest.SourceRef,
				SHA:      d.getHeadCommit(ctx, rawMergeRequest.SourceRef),
				RepoName: d.repoName,
			},
			Base: base.PullRequestBranch{
				Ref:      rawMergeRequest.TargetRef,
				SHA:      d.getHeadCommit(ctx, rawMergeRequest.TargetRef),
				RepoName: d.repoName,
			},
			ForeignIndex: rawMergeRequest.ID.Value,
			Context: codebaseIssueContext{
				Comments: comments[1:],
			},
		})

		// SECURITY: Ensure that the PR is safe
		_ = CheckAndEnsureSafePR(pullRequests[len(pullRequests)-1], d.baseURL.String(), d)
	}

	return pullRequests, true, nil
}

func (d *CodebaseDownloader) tryGetUser(ctx context.Context, userID int64) *codebaseUser {
	if len(d.userMap) == 0 {
		var rawUsers struct {
			XMLName xml.Name `xml:"users"`
			Type    string   `xml:"type,attr"`
			User    []struct {
				EmailAddress string `xml:"email-address"`
				ID           struct {
					Value int64  `xml:",chardata"`
					Type  string `xml:"type,attr"`
				} `xml:"id"`
				LastName  string `xml:"last-name"`
				FirstName string `xml:"first-name"`
				Username  string `xml:"username"`
			} `xml:"user"`
		}

		err := d.callAPI(
			ctx,
			"/users",
			nil,
			&rawUsers,
		)
		if err == nil {
			for _, user := range rawUsers.User {
				d.userMap[user.ID.Value] = &codebaseUser{
					Name:  user.Username,
					Email: user.EmailAddress,
				}
			}
		}
	}

	user, ok := d.userMap[userID]
	if !ok {
		user = &codebaseUser{
			Name: fmt.Sprintf("User %d", userID),
		}
		d.userMap[userID] = user
	}

	return user
}

func (d *CodebaseDownloader) getHeadCommit(ctx context.Context, ref string) string {
	commitRef, ok := d.commitMap[ref]
	if !ok {
		var rawCommits struct {
			XMLName xml.Name `xml:"commits"`
			Type    string   `xml:"type,attr"`
			Commit  []struct {
				Ref string `xml:"ref"`
			} `xml:"commit"`
		}
		err := d.callAPI(
			ctx,
			fmt.Sprintf("/%s/%s/commits/%s", d.project, d.repoName, ref),
			nil,
			&rawCommits,
		)
		if err == nil && len(rawCommits.Commit) > 0 {
			commitRef = rawCommits.Commit[0].Ref
			d.commitMap[ref] = commitRef
		}
	}
	return commitRef
}
