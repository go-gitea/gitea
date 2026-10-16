// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/process"
	"gitea.dev/modules/queue"
	"gitea.dev/modules/util"
)

var contributorsQueue *queue.WorkerPoolQueue[int64]

func initContributorsQueue() error {
	contributorsQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "repo_contributors", handleContributorsQueue)
	if contributorsQueue == nil {
		return errors.New("unable to create repo_contributors queue")
	}
	go graceful.GetManager().RunWithCancel(contributorsQueue)
	return nil
}

func handleContributorsQueue(repoIDs ...int64) []int64 {
	ctx := graceful.GetManager().ShutdownContext()
	for _, repoID := range repoIDs {
		if err := UpdateRepoContributors(ctx, repoID); err != nil {
			log.Error("UpdateRepoContributors [%d]: %v", repoID, err)
		}
	}
	return nil
}

// AddRepoToContributorsQueue queues an update of the contributor stats of a repository
func AddRepoToContributorsQueue(repoID int64) error {
	if err := contributorsQueue.Push(repoID); err != nil && !errors.Is(err, queue.ErrAlreadyInQueue) {
		return err
	}
	return nil
}

// UpdateRepoContributors brings the contributor stats of a repository up to date with its default branch.
// Commits added on top of the last counted commit are added to the stats, otherwise all commits are counted again.
func UpdateRepoContributors(ctx context.Context, repoID int64) error {
	for {
		var updated bool
		// a running update isn't waited for, so it checks again for pushes made meanwhile once it is done
		locked, err := globallock.TryLockAndDo(ctx, fmt.Sprintf("repo_contributors_%d", repoID), func(ctx context.Context) (err error) {
			updated, err = updateRepoContributors(ctx, repoID)
			return err
		})
		if err != nil || !locked || !updated {
			return err
		}
	}
}

func updateRepoContributors(ctx context.Context, repoID int64) (bool, error) {
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if repo_model.IsErrRepoNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if repo.IsEmpty {
		return false, nil
	}

	ctx, _, finished := process.GetManager().AddContext(ctx, "UpdateRepoContributors: "+repo.FullName())
	defer finished()

	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return false, err
	}
	defer gitRepo.Close()

	head, err := gitRepo.GetBranchCommit(ctx, repo.DefaultBranch)
	if err != nil {
		return false, err
	}
	headID := head.ID.String()
	status, err := repo_model.GetIndexerStatus(ctx, repo, repo_model.RepoIndexerTypeContributors)
	if err != nil {
		return false, err
	}
	if status.CommitSha == headID {
		return false, nil
	}

	revision, incremental := headID, false
	if status.CommitSha != "" {
		// an error means the counted commit is gone, so the history was rewritten too
		if isForcePush, err := head.IsForcePush(ctx, gitRepo, status.CommitSha); err == nil && !isForcePush {
			revision, incremental = status.CommitSha+".."+headID, true
		}
	}

	contributors, weeks, err := collectContributorStats(ctx, gitRepo, repo.ID, revision)
	if err != nil {
		return false, err
	}
	return true, db.WithTx(ctx, func(ctx context.Context) error {
		if incremental {
			err = repo_model.AddRepoContributors(ctx, contributors, weeks)
		} else {
			err = repo_model.ReplaceRepoContributors(ctx, repo.ID, contributors, weeks)
		}
		if err != nil {
			return err
		}
		return repo_model.UpdateIndexerStatus(ctx, repo, repo_model.RepoIndexerTypeContributors, headID)
	})
}

type contributionStats struct {
	commits, additions, deletions int64
}

type authorStats struct {
	name  string
	total contributionStats
	weeks map[int64]*contributionStats
}

type contributorKey struct {
	userID int64
	email  string
}

type contributorWeekKey struct {
	contributorKey
	week int64
}

// collectContributorStats counts the non-merge commits of the revision per author, matching their emails to users
func collectContributorStats(ctx context.Context, gitRepo *git.Repository, repoID int64, revision string) ([]*repo_model.RepoContributor, []*repo_model.RepoContributorWeek, error) {
	authors, err := getAuthorStats(ctx, gitRepo, revision)
	if err != nil {
		return nil, nil, err
	}

	emails := slices.Sorted(maps.Keys(authors))
	users := make(map[string]*user_model.User, len(emails))
	// GetUsersByEmails queries all emails at once, which databases limit
	for batch := range slices.Chunk(emails, 500) {
		emailUserMap, err := user_model.GetUsersByEmails(ctx, batch)
		if err != nil {
			return nil, nil, err
		}
		for _, email := range batch {
			if u := emailUserMap.GetByEmail(email); u != nil {
				users[email] = u
			}
		}
	}

	contributors := make(map[contributorKey]*repo_model.RepoContributor, len(authors))
	weeks := make(map[contributorWeekKey]*repo_model.RepoContributorWeek)
	for _, email := range emails {
		author := authors[email]
		key, name := contributorKey{email: util.TruncateRunes(email, 255)}, util.TruncateRunes(author.name, 255)
		if u := users[email]; u != nil {
			key, name = contributorKey{userID: u.ID}, ""
		}
		c := contributors[key]
		if c == nil {
			c = &repo_model.RepoContributor{RepoID: repoID, UserID: key.userID, Email: key.email, Name: name}
			contributors[key] = c
		}
		c.Commits += author.total.commits
		c.Additions += author.total.additions
		c.Deletions += author.total.deletions
		for week, stats := range author.weeks {
			w := weeks[contributorWeekKey{key, week}]
			if w == nil {
				w = &repo_model.RepoContributorWeek{RepoID: repoID, UserID: key.userID, Email: key.email, Week: week}
				weeks[contributorWeekKey{key, week}] = w
			}
			w.Commits += stats.commits
			w.Additions += stats.additions
			w.Deletions += stats.deletions
		}
	}

	contributorList := slices.SortedFunc(maps.Values(contributors), func(a, b *repo_model.RepoContributor) int {
		return cmp.Or(cmp.Compare(b.Commits, a.Commits), cmp.Compare(a.UserID, b.UserID), strings.Compare(a.Email, b.Email))
	})
	weekList := slices.SortedFunc(maps.Values(weeks), func(a, b *repo_model.RepoContributorWeek) int {
		return cmp.Or(cmp.Compare(a.Week, b.Week), cmp.Compare(a.UserID, b.UserID), strings.Compare(a.Email, b.Email))
	})
	return contributorList, weekList, nil
}

// getAuthorStats returns the stats of the non-merge commits of the revision per lower-cased author email
func getAuthorStats(ctx context.Context, gitRepo *git.Repository, revision string) (map[string]*authorStats, error) {
	// a commit is printed as "\x1e<name>\x1f<email>\x1f<date>", followed by its shortstat line if it changed any file
	cmd := gitcmd.NewCommand("log", "--shortstat", "--no-merges", "--format=%x1e%aN%x1f%aE%x1f%as").AddDynamicArguments(revision)
	stdoutReader, stdoutReaderClose := cmd.MakeStdoutPipe()
	defer stdoutReaderClose()

	authors := make(map[string]*authorStats)
	err := cmd.WithRepo(gitRepo).WithPipelineFunc(func(gitcmd.Context) error {
		var author *authorStats
		var week *contributionStats
		scanner := bufio.NewScanner(stdoutReader)
		for scanner.Scan() {
			line := scanner.Text()
			if header, ok := strings.CutPrefix(line, "\x1e"); ok {
				fields := strings.Split(header, "\x1f")
				if len(fields) != 3 {
					return fmt.Errorf("unexpected commit header %q", header)
				}
				author, week = nil, nil
				email := strings.ToLower(strings.TrimSpace(fields[1]))
				if email == "" {
					continue // a commit without an author email names no contributor
				}
				weekStart, err := weekStartOfDate(fields[2])
				if err != nil {
					return err
				}
				author = authors[email]
				if author == nil {
					author = &authorStats{name: fields[0], weeks: make(map[int64]*contributionStats)}
					authors[email] = author
				}
				week = author.weeks[weekStart]
				if week == nil {
					week = &contributionStats{}
					author.weeks[weekStart] = week
				}
				author.total.commits++
				week.commits++
			} else if author != nil && line != "" {
				additions, deletions := parseShortStat(line)
				author.total.additions += additions
				author.total.deletions += deletions
				week.additions += additions
				week.deletions += deletions
			}
		}
		return scanner.Err()
	}).RunWithStderr(ctx)
	if err != nil {
		return nil, fmt.Errorf("getAuthorStats: %w", err)
	}
	return authors, nil
}

// weekStartOfDate returns the Unix milliseconds of the Sunday starting the week of the date
func weekStartOfDate(date string) (int64, error) {
	t, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return 0, err
	}
	return t.AddDate(0, 0, -int(t.Weekday())).UnixMilli(), nil
}

// parseShortStat parses a line like " 3 files changed, 10 insertions(+), 2 deletions(-)"
func parseShortStat(line string) (additions, deletions int64) {
	for field := range strings.SplitSeq(line, ",") {
		num, kind, _ := strings.Cut(strings.TrimSpace(field), " ")
		switch {
		case strings.HasPrefix(kind, "insertion"):
			additions, _ = strconv.ParseInt(num, 10, 64)
		case strings.HasPrefix(kind, "deletion"):
			deletions, _ = strconv.ParseInt(num, 10, 64)
		}
	}
	return additions, deletions
}
