// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package activities

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/avatars"
	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	contribution_model "code.gitea.io/gitea/models/repo/contribution"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"

	"xorm.io/builder"
)

// ActivityAuthorData represents statistical git commit count data
type ActivityAuthorData struct {
	Name       string `json:"name"`
	Login      string `json:"login"`
	AvatarLink string `json:"avatar_link"`
	HomeLink   string `json:"home_link"`
	Commits    int64  `json:"commits"`
}

// CodeActivityStats represents git statistics data
type CodeActivityStats struct {
	AuthorCount  int64
	CommitCount  int64
	ChangedFiles int64
	Additions    int64
	Deletions    int64
}

// ActivityStats represents issue and pull request information.
type ActivityStats struct {
	OpenedPRs                   issues_model.PullRequestList
	OpenedPRAuthorCount         int64
	MergedPRs                   issues_model.PullRequestList
	MergedPRAuthorCount         int64
	ActiveIssues                issues_model.IssueList
	OpenedIssues                issues_model.IssueList
	OpenedIssueAuthorCount      int64
	ClosedIssues                issues_model.IssueList
	ClosedIssueAuthorCount      int64
	UnresolvedIssues            issues_model.IssueList
	PublishedReleases           []*repo_model.Release
	PublishedReleaseAuthorCount int64
	Code                        *CodeActivityStats
}

// GetActivityStats return stats for repository at given time range
func GetActivityStats(ctx context.Context, repo *repo_model.Repository, timeFrom time.Time, releases, issues, prs, code bool) (*ActivityStats, error) {
	stats := &ActivityStats{Code: &CodeActivityStats{}}
	if releases {
		if err := stats.FillReleases(ctx, repo.ID, timeFrom); err != nil {
			return nil, fmt.Errorf("FillReleases: %w", err)
		}
	}
	if prs {
		if err := stats.FillPullRequests(ctx, repo.ID, timeFrom); err != nil {
			return nil, fmt.Errorf("FillPullRequests: %w", err)
		}
	}
	if issues {
		if err := stats.FillIssues(ctx, repo.ID, timeFrom); err != nil {
			return nil, fmt.Errorf("FillIssues: %w", err)
		}
	}
	if err := stats.FillUnresolvedIssues(ctx, repo.ID, timeFrom, issues, prs); err != nil {
		return nil, fmt.Errorf("FillUnresolvedIssues: %w", err)
	}
	if code {
		codeStats, err := getCodeActivityStats(ctx, repo, timeFrom)
		if err != nil {
			return nil, err
		}
		stats.Code = codeStats
	}
	return stats, nil
}

// GetActivityStatsTopAuthors returns top author stats for git commits for all branches
func GetActivityStatsTopAuthors(ctx context.Context, repo *repo_model.Repository, timeFrom time.Time, count int) ([]*ActivityAuthorData, error) {
	stats, err := contribution_model.GetContributorActivity(ctx, repo, timeFrom, count)
	if err != nil {
		return nil, err
	}
	if len(stats) == 0 {
		return []*ActivityAuthorData{}, nil
	}

	return ActivityStats2AuthorData(ctx, stats, 24)
}

func ActivityStats2AuthorData(ctx context.Context, stats []*contribution_model.ContributorSummary, avatarSize int) ([]*ActivityAuthorData, error) {
	userIDs := container.Set[int64]{}
	for _, stat := range stats {
		if stat.UserID > 0 {
			userIDs.Add(stat.UserID)
		}
	}

	userMap, err := user_model.GetUsersMapByIDs(ctx, userIDs.Values())
	if err != nil {
		return nil, err
	}

	contributors := make([]*ActivityAuthorData, 0, len(stats))
	gitUserAvatarLink := user_model.NewGhostUser().AvatarLinkWithSize(ctx, avatarSize)
	for _, stat := range stats {
		user := userMap[stat.UserID]
		if user == nil {
			avatarLink := avatars.GenerateEmailAvatarFastLink(ctx, stat.Email, avatarSize)
			if avatarLink == "" {
				avatarLink = gitUserAvatarLink
			}
			contributors = append(contributors, &ActivityAuthorData{
				Name:       stat.AuthorName,
				AvatarLink: avatarLink,
				Commits:    stat.Commits,
			})
			continue
		}
		contributors = append(contributors, &ActivityAuthorData{
			Name:       user.DisplayName(),
			Login:      user.LowerName,
			AvatarLink: user.AvatarLinkWithSize(ctx, avatarSize),
			HomeLink:   user.HomeLink(),
			Commits:    stat.Commits,
		})
	}

	return contributors, nil
}

func getCodeActivityStats(ctx context.Context, repo *repo_model.Repository, timeFrom time.Time) (*CodeActivityStats, error) {
	start := contribution_model.NewContributorDayStart(timeFrom.UTC())

	var res CodeActivityStats
	_, err := db.GetEngine(ctx).
		SQL("SELECT COALESCE(SUM(additions), 0) AS additions, COALESCE(SUM(deletions), 0) AS deletions, COALESCE(SUM(commits), 0) AS commit_count, COALESCE(SUM(changed_files), 0) AS changed_files FROM repo_contributor_daily WHERE repo_id=? AND day_start >= ?", repo.ID, start).
		Get(&res)
	if err != nil {
		return nil, err
	}

	_, err = db.GetEngine(ctx).
		SQL("SELECT COUNT(*) AS author_count FROM (SELECT user_id, email, author_name FROM repo_contributor_daily WHERE repo_id=? AND day_start >= ? GROUP BY user_id, email, author_name) temp", repo.ID, start).
		Get(&res)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

// ActivePRCount returns total active pull request count
func (stats *ActivityStats) ActivePRCount() int {
	return stats.OpenedPRCount() + stats.MergedPRCount()
}

// OpenedPRCount returns opened pull request count
func (stats *ActivityStats) OpenedPRCount() int {
	return len(stats.OpenedPRs)
}

// OpenedPRPerc returns opened pull request percents from total active
func (stats *ActivityStats) OpenedPRPerc() int {
	return int(float32(stats.OpenedPRCount()) / float32(stats.ActivePRCount()) * 100.0)
}

// MergedPRCount returns merged pull request count
func (stats *ActivityStats) MergedPRCount() int {
	return len(stats.MergedPRs)
}

// MergedPRPerc returns merged pull request percent from total active
func (stats *ActivityStats) MergedPRPerc() int {
	return int(float32(stats.MergedPRCount()) / float32(stats.ActivePRCount()) * 100.0)
}

// ActiveIssueCount returns total active issue count
func (stats *ActivityStats) ActiveIssueCount() int {
	return len(stats.ActiveIssues)
}

// OpenedIssueCount returns open issue count
func (stats *ActivityStats) OpenedIssueCount() int {
	return len(stats.OpenedIssues)
}

// OpenedIssuePerc returns open issue count percent from total active
func (stats *ActivityStats) OpenedIssuePerc() int {
	return int(float32(stats.OpenedIssueCount()) / float32(stats.ActiveIssueCount()) * 100.0)
}

// ClosedIssueCount returns closed issue count
func (stats *ActivityStats) ClosedIssueCount() int {
	return len(stats.ClosedIssues)
}

// ClosedIssuePerc returns closed issue count percent from total active
func (stats *ActivityStats) ClosedIssuePerc() int {
	return int(float32(stats.ClosedIssueCount()) / float32(stats.ActiveIssueCount()) * 100.0)
}

// UnresolvedIssueCount returns unresolved issue and pull request count
func (stats *ActivityStats) UnresolvedIssueCount() int {
	return len(stats.UnresolvedIssues)
}

// PublishedReleaseCount returns published release count
func (stats *ActivityStats) PublishedReleaseCount() int {
	return len(stats.PublishedReleases)
}

// FillPullRequests returns pull request information for activity page
func (stats *ActivityStats) FillPullRequests(ctx context.Context, repoID int64, fromTime time.Time) error {
	var err error
	var count int64

	// Merged pull requests
	sess := pullRequestsForActivityStatement(ctx, repoID, fromTime, true)
	sess.OrderBy("pull_request.merged_unix DESC")
	stats.MergedPRs = make(issues_model.PullRequestList, 0)
	if err = sess.Find(&stats.MergedPRs); err != nil {
		return err
	}
	if err = stats.MergedPRs.LoadAttributes(ctx); err != nil {
		return err
	}

	// Merged pull request authors
	sess = pullRequestsForActivityStatement(ctx, repoID, fromTime, true)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("pull_request").Get(&count); err != nil {
		return err
	}
	stats.MergedPRAuthorCount = count

	// Opened pull requests
	sess = pullRequestsForActivityStatement(ctx, repoID, fromTime, false)
	sess.OrderBy("issue.created_unix ASC")
	stats.OpenedPRs = make(issues_model.PullRequestList, 0)
	if err = sess.Find(&stats.OpenedPRs); err != nil {
		return err
	}
	if err = stats.OpenedPRs.LoadAttributes(ctx); err != nil {
		return err
	}

	// Opened pull request authors
	sess = pullRequestsForActivityStatement(ctx, repoID, fromTime, false)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("pull_request").Get(&count); err != nil {
		return err
	}
	stats.OpenedPRAuthorCount = count

	return nil
}

func pullRequestsForActivityStatement(ctx context.Context, repoID int64, fromTime time.Time, merged bool) db.Session {
	sess := db.GetEngine(ctx).Where("pull_request.base_repo_id=?", repoID).
		Join("INNER", "issue", "pull_request.issue_id = issue.id")

	if merged {
		sess.And("pull_request.has_merged = ?", true)
		sess.And("pull_request.merged_unix >= ?", fromTime.Unix())
	} else {
		sess.And("issue.is_closed = ?", false)
		sess.And("issue.created_unix >= ?", fromTime.Unix())
	}

	return sess
}

// FillIssues returns issue information for activity page
func (stats *ActivityStats) FillIssues(ctx context.Context, repoID int64, fromTime time.Time) error {
	var err error
	var count int64

	// Closed issues
	sess := issuesForActivityStatement(ctx, repoID, fromTime, true, false)
	sess.OrderBy("issue.closed_unix DESC")
	stats.ClosedIssues = make(issues_model.IssueList, 0)
	if err = sess.Find(&stats.ClosedIssues); err != nil {
		return err
	}

	// Closed issue authors
	sess = issuesForActivityStatement(ctx, repoID, fromTime, true, false)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("issue").Get(&count); err != nil {
		return err
	}
	stats.ClosedIssueAuthorCount = count

	// New issues
	sess = newlyCreatedIssues(ctx, repoID, fromTime)
	sess.OrderBy("issue.created_unix ASC")
	stats.OpenedIssues = make(issues_model.IssueList, 0)
	if err = sess.Find(&stats.OpenedIssues); err != nil {
		return err
	}

	// Active issues
	sess = activeIssues(ctx, repoID, fromTime)
	sess.OrderBy("issue.created_unix ASC")
	stats.ActiveIssues = make(issues_model.IssueList, 0)
	if err = sess.Find(&stats.ActiveIssues); err != nil {
		return err
	}

	// Opened issue authors
	sess = issuesForActivityStatement(ctx, repoID, fromTime, false, false)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("issue").Get(&count); err != nil {
		return err
	}
	stats.OpenedIssueAuthorCount = count

	return nil
}

// FillUnresolvedIssues returns unresolved issue and pull request information for activity page
func (stats *ActivityStats) FillUnresolvedIssues(ctx context.Context, repoID int64, fromTime time.Time, issues, prs bool) error {
	// Check if we need to select anything
	if !issues && !prs {
		return nil
	}
	sess := issuesForActivityStatement(ctx, repoID, fromTime, false, true)
	if !issues || !prs {
		sess.And("issue.is_pull = ?", prs)
	}
	sess.OrderBy("issue.updated_unix DESC")
	stats.UnresolvedIssues = make(issues_model.IssueList, 0)
	return sess.Find(&stats.UnresolvedIssues)
}

func newlyCreatedIssues(ctx context.Context, repoID int64, fromTime time.Time) db.Session {
	sess := db.GetEngine(ctx).Where("issue.repo_id = ?", repoID).
		And("issue.is_pull = ?", false).                // Retain the is_pull check to exclude pull requests
		And("issue.created_unix >= ?", fromTime.Unix()) // Include all issues created after fromTime

	return sess
}

func activeIssues(ctx context.Context, repoID int64, fromTime time.Time) db.Session {
	sess := db.GetEngine(ctx).Where("issue.repo_id = ?", repoID).
		And("issue.is_pull = ?", false).
		And(builder.Or(
			builder.Gte{"issue.created_unix": fromTime.Unix()},
			builder.Gte{"issue.closed_unix": fromTime.Unix()},
		))

	return sess
}

func issuesForActivityStatement(ctx context.Context, repoID int64, fromTime time.Time, closed, unresolved bool) db.Session {
	sess := db.GetEngine(ctx).Where("issue.repo_id = ?", repoID).
		And("issue.is_closed = ?", closed)

	if !unresolved {
		sess.And("issue.is_pull = ?", false)
		if closed {
			sess.And("issue.closed_unix >= ?", fromTime.Unix())
		} else {
			sess.And("issue.created_unix >= ?", fromTime.Unix())
		}
	} else {
		sess.And("issue.created_unix < ?", fromTime.Unix())
		sess.And("issue.updated_unix >= ?", fromTime.Unix())
	}

	return sess
}

// FillReleases returns release information for activity page
func (stats *ActivityStats) FillReleases(ctx context.Context, repoID int64, fromTime time.Time) error {
	var err error
	var count int64

	// Published releases list
	sess := releasesForActivityStatement(ctx, repoID, fromTime)
	sess.OrderBy("`release`.created_unix DESC")
	stats.PublishedReleases = make([]*repo_model.Release, 0)
	if err = sess.Find(&stats.PublishedReleases); err != nil {
		return err
	}

	// Published releases authors
	sess = releasesForActivityStatement(ctx, repoID, fromTime)
	if _, err = sess.Select("count(distinct `release`.publisher_id) as `count`").Table("release").Get(&count); err != nil {
		return err
	}
	stats.PublishedReleaseAuthorCount = count

	return nil
}

func releasesForActivityStatement(ctx context.Context, repoID int64, fromTime time.Time) db.Session {
	return db.GetEngine(ctx).Where("`release`.repo_id = ?", repoID).
		And("`release`.is_draft = ?", false).
		And("`release`.created_unix >= ?", fromTime.Unix())
}
