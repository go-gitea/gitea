// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/process"

	"github.com/go-xorm/xorm"
)

// ActivityAuthorData represents statistical git commit count data
type ActivityAuthorData struct {
	Name       string `json:"name"`
	Login      string `json:"login"`
	AvatarLink string `json:"avatar_link"`
	Commits    int64  `json:"commits"`
}

// CodeActivityStats represents git statistics data
type CodeActivityStats struct {
	AuthorCount              int64
	CommitCount              int64
	ChangedFiles             int64
	Additions                int64
	Deletions                int64
	CommitCountInAllBranches int64
	Authors                  map[string]int64
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
	Code                        *CodeActivityStats
}

// GetActivityStats return stats for repository at given time range
func GetActivityStats(repo *Repository, timeFrom time.Time, releases, issues, prs, code bool) (*ActivityStats, error) {
	stats := &ActivityStats{Code: &CodeActivityStats{}}
	if releases {
		if err := stats.FillReleases(repo.ID, timeFrom); err != nil {
			return nil, fmt.Errorf("FillReleases: %v", err)
		}
	}
	if prs {
		if err := stats.FillPullRequests(repo.ID, timeFrom); err != nil {
			return nil, fmt.Errorf("FillPullRequests: %v", err)
		}
	}
	if issues {
		if err := stats.FillIssues(repo.ID, timeFrom); err != nil {
			return nil, fmt.Errorf("FillIssues: %v", err)
		}
	}
	if err := stats.FillUnresolvedIssues(repo.ID, timeFrom, issues, prs); err != nil {
		return nil, fmt.Errorf("FillUnresolvedIssues: %v", err)
	}
	if code {
		if err := stats.Code.FillFromGit(repo, timeFrom, false); err != nil {
			return nil, fmt.Errorf("FillFromGit: %v", err)
		}
	}
	return stats, nil
}

// GetActivityStatsAuthors returns stats for git commits for all branches
func GetActivityStatsAuthors(repo *Repository, timeFrom time.Time) (*CodeActivityStats, error) {
	code := &CodeActivityStats{}
	if err := code.FillFromGit(repo, timeFrom, true); err != nil {
		return nil, fmt.Errorf("FillFromGit: %v", err)
	}
	return code, nil
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
	sess := x.Where("pull_request.base_repo_id=?", repoID).
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
func (stats *ActivityStats) FillIssues(repoID int64, fromTime time.Time) error {
	var err error
	var count int64

	// Closed issues
	sess := issuesForActivityStatement(repoID, fromTime, true, false)
	sess.OrderBy("issue.closed_unix DESC")
	stats.ClosedIssues = make(IssueList, 0)
	if err = sess.Find(&stats.ClosedIssues); err != nil {
		return err
	}

	// Closed issue authors
	sess = issuesForActivityStatement(repoID, fromTime, true, false)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("issue").Get(&count); err != nil {
		return err
	}
	stats.ClosedIssueAuthorCount = count

	// New issues
	sess = issuesForActivityStatement(repoID, fromTime, false, false)
	sess.OrderBy("issue.created_unix ASC")
	stats.OpenedIssues = make(IssueList, 0)
	if err = sess.Find(&stats.OpenedIssues); err != nil {
		return err
	}

	// Opened issue authors
	sess = issuesForActivityStatement(repoID, fromTime, false, false)
	if _, err = sess.Select("count(distinct issue.poster_id) as `count`").Table("issue").Get(&count); err != nil {
		return err
	}
	stats.OpenedIssueAuthorCount = count

	return nil
}

// FillUnresolvedIssues returns unresolved issue and pull request information for activity page
func (stats *ActivityStats) FillUnresolvedIssues(repoID int64, fromTime time.Time, issues, prs bool) error {
	// Check if we need to select anything
	if !issues && !prs {
		return nil
	}
	sess := issuesForActivityStatement(repoID, fromTime, false, true)
	if !issues || !prs {
		sess.And("issue.is_pull = ?", prs)
	}
	sess.OrderBy("issue.updated_unix DESC")
	stats.UnresolvedIssues = make(IssueList, 0)
	return sess.Find(&stats.UnresolvedIssues)
}

func issuesForActivityStatement(repoID int64, fromTime time.Time, closed, unresolved bool) *xorm.Session {
	sess := x.Where("issue.repo_id = ?", repoID).
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
	return x.Where("release.repo_id = ?", repoID).
		And("release.is_draft = ?", false).
		And("release.created_unix >= ?", fromTime.Unix())
}

// FillFromGit returns code statistics for acitivity page
func (stats *CodeActivityStats) FillFromGit(repo *Repository, fromTime time.Time, allBranches bool) error {
	gitPath := repo.RepoPath()
	since := fromTime.Format(time.RFC3339)

	stdout, stderr, err := process.GetManager().ExecDir(-1, gitPath,
		fmt.Sprintf("FillFromGit.RevList (git rev-list): %s", gitPath),
		"git", "rev-list", "--count", "--no-merges", "--branches=*", "--date=iso", fmt.Sprintf("--since='%s'", since))
	if err != nil {
		return fmt.Errorf("git rev-list --count --branch [%s]: %s", gitPath, stderr)
	}

	c, err := strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
	if err != nil {
		return err
	}
	stats.CommitCountInAllBranches = c

	args := []string{"log", "--numstat", "--no-merges", "--pretty=format:---%n%h%n%an%n%ae%n", "--date=iso", fmt.Sprintf("--since='%s'", since)}
	if allBranches {
		args = append(args, "--branches=*")
	} else {
		args = append(args, "--first-parent", repo.DefaultBranch)
	}

	stdout, stderr, err = process.GetManager().ExecDir(-1, gitPath,
		fmt.Sprintf("FillFromGit.RevList (git rev-list): %s", gitPath),
		"git", args...)
	if err != nil {
		return fmt.Errorf("git log --numstat [%s]: %s", gitPath, stderr)
	}

	scanner := bufio.NewScanner(strings.NewReader(stdout))
	scanner.Split(bufio.ScanLines)
	stats.CommitCount = 0
	stats.Additions = 0
	stats.Deletions = 0
	authors := make(map[string]int64)
	files := make(map[string]bool)
	p := 0
	for scanner.Scan() {
		l := strings.TrimSpace(scanner.Text())
		if l == "---" {
			p = 1
		} else if p == 0 {
			continue
		} else {
			p++
		}
		if p > 4 && len(l) == 0 {
			continue
		}
		switch p {
		case 1: // Separator
		case 2: // Commit sha-1
			stats.CommitCount++
		case 3: // Author
			//fmt.Println("Author: " + l)
		case 4: // E-mail
			email := strings.ToLower(l)
			i := authors[email]
			authors[email] = i + 1
		default: // Changed file
			if parts := strings.Fields(l); len(parts) >= 3 {
				if parts[0] != "-" {
					if c, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64); err == nil {
						stats.Additions += c
					}
				}
				if parts[1] != "-" {
					if c, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err == nil {
						stats.Deletions += c
					}
				}
				if _, ok := files[parts[2]]; !ok {
					files[parts[2]] = true
				}
			}
		}
	}
	stats.AuthorCount = int64(len(authors))
	stats.ChangedFiles = int64(len(files))
	stats.Authors = authors

	return nil
}

// GetTopAuthors get top users with most commit count based on already loaded data from git
func (stats *CodeActivityStats) GetTopAuthors(count int) ([]*ActivityAuthorData, error) {
	if stats.Authors == nil {
		return nil, nil
	}
	users := make(map[int64]*ActivityAuthorData)
	for k, v := range stats.Authors {
		if len(k) == 0 {
			continue
		}
		u, err := GetUserByEmail(k)
		if u == nil || IsErrUserNotExist(err) {
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
				Commits:    v,
			}
		} else {
			user.Commits += v
		}
	}
	v := make([]*ActivityAuthorData, 0)
	for _, u := range users {
		v = append(v, u)
	}

	sort.Slice(v[:], func(i, j int) bool {
		return v[i].Commits < v[j].Commits
	})

	cnt := count
	if cnt > len(v) {
		cnt = len(v)
	}

	return v[:cnt], nil
}
