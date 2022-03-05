// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"sort"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// ActivityAuthorData represents statistical git commit count data
type ActivityAuthorData struct {
	Name       string `json:"name"`
	Login      string `json:"login"`
	AvatarLink string `json:"avatar_link"`
	HomeLink   string `json:"home_link"`
	Commits    int64  `json:"commits"`
}

// ActivityStats represets issue and pull request information.
type ActivityStats struct {
	OpenedPRs                   PullRequestList
	OpenedPRAuthorCount         int64
	MergedPRs                   PullRequestList
	MergedPRAuthorCount         int64
	OpenedIssues                IssueList
	OpenedIssueAuthorCount      int64
	ClosedIssues                IssueList
	ClosedIssueAuthorCount      int64
	UnresolvedIssues            IssueList
	PublishedReleases           []*Release
	PublishedReleaseAuthorCount int64
	Code                        *git.CodeActivityStats
}

// GetActivityStatsOpts represents the possible options to GetActivityStats
type GetActivityStatsOpts struct {
	TimeFrom             time.Time
	UserID               int64
	ShowReleases         bool
	ShowIssues           bool
	ShowPRs              bool
	ShowCode             bool
	CanReadPrivateIssues bool
}

// GetActivityStats return stats for repository at given time range, as user
func GetActivityStats(repo *repo_model.Repository, opts *GetActivityStatsOpts) (*ActivityStats, error) {
	stats := &ActivityStats{Code: &git.CodeActivityStats{}}
	if opts.ShowReleases {
		if err := stats.FillReleases(repo.ID, opts.TimeFrom); err != nil {
			return nil, fmt.Errorf("FillReleases: %v", err)
		}
	}
	if opts.ShowPRs {
		if err := stats.FillPullRequests(repo.ID, opts.TimeFrom); err != nil {
			return nil, fmt.Errorf("FillPullRequests: %v", err)
		}
	}
	if opts.ShowIssues {
		if err := stats.FillIssues(repo.ID, opts.TimeFrom, opts.CanReadPrivateIssues, opts.UserID); err != nil {
			return nil, fmt.Errorf("FillIssues: %v", err)
		}
	}
	if err := stats.FillUnresolvedIssues(repo.ID, opts.TimeFrom, opts.ShowIssues, opts.ShowPRs, opts.CanReadPrivateIssues, opts.UserID); err != nil {
		return nil, fmt.Errorf("FillUnresolvedIssues: %v", err)
	}

	if opts.ShowCode {
		gitRepo, closer, err := git.RepositoryFromContextOrOpen(git.DefaultContext, repo.RepoPath())
		if err != nil {
			return nil, fmt.Errorf("OpenRepository: %v", err)
		}
		defer closer.Close()

		code, err := gitRepo.GetCodeActivityStats(opts.TimeFrom, repo.DefaultBranch)
		if err != nil {
			return nil, fmt.Errorf("FillFromGit: %v", err)
		}
		stats.Code = code
	}
	return stats, nil
}

// GetActivityStatsTopAuthors returns top author stats for git commits for all branches
func GetActivityStatsTopAuthors(ctx context.Context, repo *repo_model.Repository, timeFrom time.Time, count int) ([]*ActivityAuthorData, error) {
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo.RepoPath())
	if err != nil {
		return nil, fmt.Errorf("OpenRepository: %v", err)
	}
	defer closer.Close()

	code, err := gitRepo.GetCodeActivityStats(timeFrom, "")
	if err != nil {
		return nil, fmt.Errorf("FillFromGit: %v", err)
	}
	if code.Authors == nil {
		return nil, nil
	}
	users := make(map[int64]*ActivityAuthorData)
	var unknownUserID int64
	unknownUserAvatarLink := user_model.NewGhostUser().AvatarLink()
	for _, v := range code.Authors {
		if len(v.Email) == 0 {
			continue
		}
		u, err := user_model.GetUserByEmail(v.Email)
		if u == nil || user_model.IsErrUserNotExist(err) {
			unknownUserID--
			users[unknownUserID] = &ActivityAuthorData{
				Name:       v.Name,
				AvatarLink: unknownUserAvatarLink,
				Commits:    v.Commits,
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		if user, ok := users[u.ID]; !ok {
			users[u.ID] = &ActivityAuthorData{
				Name:       u.DisplayName(),
				Login:      u.LowerName,
				AvatarLink: u.AvatarLink(),
				HomeLink:   u.HomeLink(),
				Commits:    v.Commits,
			}
		} else {
			user.Commits += v.Commits
		}
	}
	v := make([]*ActivityAuthorData, 0)
	for _, u := range users {
		v = append(v, u)
	}

	sort.Slice(v, func(i, j int) bool {
		return v[i].Commits > v[j].Commits
	})

	cnt := count
	if cnt > len(v) {
		cnt = len(v)
	}

	return v[:cnt], nil
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
	return stats.OpenedIssueCount() + stats.ClosedIssueCount()
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
func (stats *ActivityStats) FillPullRequests(repoID int64, fromTime time.Time) error {
	var err error
	var count int64

	// Merged pull requests
	sess := pullRequestsForActivityStatement(repoID, fromTime, true)
	sess.OrderBy("pull_request.merged_unix DESC")
	stats.MergedPRs = make(PullRequestList, 0)
	if err = sess.Find(&stats.MergedPRs); err != nil {
		return err
	}
	if err = stats.MergedPRs.LoadAttributes(); err != nil {
		return err
	}

	// Merged pull request authors
	sess = pullRequestsForActivityStatement(repoID, fromTime, true)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("pull_request").Get(&count); err != nil {
		return err
	}
	stats.MergedPRAuthorCount = count

	// Opened pull requests
	sess = pullRequestsForActivityStatement(repoID, fromTime, false)
	sess.OrderBy("issue.created_unix ASC")
	stats.OpenedPRs = make(PullRequestList, 0)
	if err = sess.Find(&stats.OpenedPRs); err != nil {
		return err
	}
	if err = stats.OpenedPRs.LoadAttributes(); err != nil {
		return err
	}

	// Opened pull request authors
	sess = pullRequestsForActivityStatement(repoID, fromTime, false)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("pull_request").Get(&count); err != nil {
		return err
	}
	stats.OpenedPRAuthorCount = count

	return nil
}

func pullRequestsForActivityStatement(repoID int64, fromTime time.Time, merged bool) *xorm.Session {
	sess := db.GetEngine(db.DefaultContext).Where("pull_request.base_repo_id=?", repoID).
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
func (stats *ActivityStats) FillIssues(repoID int64, fromTime time.Time, canReadPrivateIssues bool, userID int64) error {
	var err error
	var count int64

	activityStreamOptions := &activityStreamOpts{
		fromTime:             fromTime,
		userID:               userID,
		repoID:               repoID,
		closed:               true,
		unresolved:           false,
		canReadPrivateIssues: canReadPrivateIssues,
	}

	// Closed issues
	sess := issuesForActivityStatement(activityStreamOptions)
	sess.OrderBy("issue.closed_unix DESC")
	stats.ClosedIssues = make(IssueList, 0)
	if err = sess.Find(&stats.ClosedIssues); err != nil {
		return err
	}

	// Closed issue authors
	sess = issuesForActivityStatement(activityStreamOptions)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("issue").Get(&count); err != nil {
		return err
	}
	stats.ClosedIssueAuthorCount = count

	activityStreamOptions.closed = false

	// New issues
	sess = issuesForActivityStatement(activityStreamOptions)
	sess.OrderBy("issue.created_unix ASC")
	stats.OpenedIssues = make(IssueList, 0)
	if err = sess.Find(&stats.OpenedIssues); err != nil {
		return err
	}

	// Opened issue authors
	sess = issuesForActivityStatement(activityStreamOptions)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("issue").Get(&count); err != nil {
		return err
	}
	stats.OpenedIssueAuthorCount = count

	return nil
}

// FillUnresolvedIssues returns unresolved issue and pull request information for activity page
func (stats *ActivityStats) FillUnresolvedIssues(repoID int64, fromTime time.Time, issues, prs bool, canReadPrivateIssues bool, userID int64) error {
	// Check if we need to select anything
	if !issues && !prs {
		return nil
	}
	sess := issuesForActivityStatement(&activityStreamOpts{
		fromTime:             fromTime,
		userID:               userID,
		repoID:               repoID,
		closed:               false,
		unresolved:           true,
		canReadPrivateIssues: canReadPrivateIssues,
	})
	if !issues || !prs {
		sess.And("issue.is_pull = ?", prs)
	}
	sess.OrderBy("issue.updated_unix DESC")
	stats.UnresolvedIssues = make(IssueList, 0)
	return sess.Find(&stats.UnresolvedIssues)
}

type activityStreamOpts struct {
	fromTime             time.Time
	userID               int64
	repoID               int64
	closed               bool
	unresolved           bool
	canReadPrivateIssues bool
}

func issuesForActivityStatement(opts *activityStreamOpts) *xorm.Session {
	sess := db.GetEngine(db.DefaultContext).Where("issue.repo_id = ?", opts.repoID).
		And("issue.is_closed = ?", opts.closed)

	if !opts.unresolved {
		sess.And("issue.is_pull = ?", false)
		if opts.closed {
			sess.And("issue.closed_unix >= ?", opts.fromTime.Unix())
		} else {
			sess.And("issue.created_unix >= ?", opts.fromTime.Unix())
		}
	} else {
		sess.And("issue.created_unix < ?", opts.fromTime.Unix())
		sess.And("issue.updated_unix >= ?", opts.fromTime.Unix())
	}

	if !opts.canReadPrivateIssues {
		if opts.userID == 0 {
			sess.And("issue.is_private = ?", false)
		} else {
			// Allow to see private issues if the user is the poster of it.
			sess.And(
				builder.Or(
					builder.Eq{"`issue`.is_private": false},
					builder.And(
						builder.Eq{"`issue`.is_private": true},
						builder.In("`issue`.poster_id", opts.userID),
					),
				),
			)
		}
	}

	return sess
}

// FillReleases returns release information for activity page
func (stats *ActivityStats) FillReleases(repoID int64, fromTime time.Time) error {
	var err error
	var count int64

	// Published releases list
	sess := releasesForActivityStatement(repoID, fromTime)
	sess.OrderBy("release.created_unix DESC")
	stats.PublishedReleases = make([]*Release, 0)
	if err = sess.Find(&stats.PublishedReleases); err != nil {
		return err
	}

	// Published releases authors
	sess = releasesForActivityStatement(repoID, fromTime)
	if _, err = sess.Select("count(distinct release.publisher_id) as `count`").Table("release").Get(&count); err != nil {
		return err
	}
	stats.PublishedReleaseAuthorCount = count

	return nil
}

func releasesForActivityStatement(repoID int64, fromTime time.Time) *xorm.Session {
	return db.GetEngine(db.DefaultContext).Where("release.repo_id = ?", repoID).
		And("release.is_draft = ?", false).
		And("release.created_unix >= ?", fromTime.Unix())
}
