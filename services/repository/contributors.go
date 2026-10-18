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
	"gitea.dev/modules/cache"
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
		if err := UpdateRepoContributors(ctx, repoID); err != nil && ctx.Err() == nil {
			log.Error("UpdateRepoContributors [%d]: %v", repoID, err)
			// EnsureContributorStats returns it for a while instead of queueing the failing count again
			_ = cache.GetCache().Put(contributorsErrorCacheKey(repoID), err.Error(), 10*60)
		}
	}
	return nil
}

func contributorsErrorCacheKey(repoID int64) string {
	return fmt.Sprintf("repo_contributors_error_%d", repoID)
}

func AddRepoToContributorsQueue(repoID int64) error {
	if err := contributorsQueue.Push(repoID); err != nil && !errors.Is(err, queue.ErrAlreadyInQueue) {
		return err
	}
	return nil
}

func UpdateRepoContributors(ctx context.Context, repoID int64) error {
	// updates finding the lock taken skip their push, so the lock holder checks for it again after releasing
	for {
		repo, err := repo_model.GetRepositoryByID(ctx, repoID)
		if repo_model.IsErrRepoNotExist(err) {
			return nil
		} else if err != nil {
			return err
		}
		headID, countedID, err := getContributorStatsCommits(ctx, repo)
		if err != nil || headID == countedID {
			return err
		}
		locked, err := globallock.TryLockAndDo(ctx, fmt.Sprintf("repo_contributors_%d", repoID), func(ctx context.Context) error {
			return updateRepoContributors(ctx, repo, headID, countedID)
		})
		if err != nil || !locked {
			return err
		}
	}
}

func getContributorStatsCommits(ctx context.Context, repo *repo_model.Repository) (headID, countedID string, err error) {
	if repo.IsEmpty {
		return "", "", nil
	}
	gitRepo, closer, err := git.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return "", "", err
	}
	defer closer.Close()
	if headID, err = gitRepo.GetBranchCommitID(ctx, repo.DefaultBranch); err != nil {
		return "", "", err
	}
	status, err := repo_model.GetIndexerStatus(ctx, repo, repo_model.RepoIndexerTypeContributors)
	if err != nil {
		return "", "", err
	}
	return headID, status.CommitSha, nil
}

func updateRepoContributors(ctx context.Context, repo *repo_model.Repository, headID, countedID string) error {
	status, err := repo_model.GetIndexerStatus(ctx, repo, repo_model.RepoIndexerTypeContributors)
	if err != nil || status.CommitSha != countedID {
		return err
	}

	ctx, _, finished := process.GetManager().AddContext(ctx, "UpdateRepoContributors: "+repo.FullName())
	defer finished()

	gitRepo, err := git.OpenRepository(ctx, repo)
	if err != nil {
		return err
	}
	defer gitRepo.Close()

	revision := headID
	// add to the counted stats only on top of an ancestor with an unchanged .mailmap, otherwise count everything again
	if countedID != "" && gitcmd.NewCommand("merge-base", "--is-ancestor").AddDynamicArguments(countedID, headID).WithRepo(gitRepo).Run(ctx) == nil &&
		gitcmd.NewCommand("diff", "--quiet").AddDynamicArguments(countedID, headID).AddDashesAndList(".mailmap").WithRepo(gitRepo).Run(ctx) == nil {
		revision = countedID + ".." + headID
	}

	contributors, weeks, err := collectContributorStats(ctx, gitRepo, repo.ID, revision)
	if err != nil {
		return err
	}
	return db.WithTx(ctx, func(ctx context.Context) error {
		if revision != headID {
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
	name   string
	merges int64
	weeks  map[int64]*contributionStats
}

type contributorKey struct {
	userID int64
	email  string
}

type contributorWeekKey struct {
	contributorKey
	week int64
}

func collectContributorStats(ctx context.Context, gitRepo *git.Repository, repoID int64, revision string) ([]*repo_model.RepoContributor, []*repo_model.RepoContributorWeek, error) {
	authors, err := getAuthorStats(ctx, gitRepo, revision)
	if err != nil {
		return nil, nil, err
	}

	contributors := make(map[contributorKey]*repo_model.RepoContributor, len(authors))
	weeks := make(map[contributorWeekKey]*repo_model.RepoContributorWeek)
	// GetUsersByEmails queries all emails at once, which databases limit
	for emails := range slices.Chunk(slices.Sorted(maps.Keys(authors)), 500) {
		emailUserMap, err := user_model.GetUsersByEmails(ctx, emails)
		if err != nil {
			return nil, nil, err
		}
		for _, email := range emails {
			author := authors[email]
			key, name := contributorKey{email: util.TruncateRunes(email, 255)}, util.TruncateRunes(author.name, 255)
			if user := emailUserMap.GetByEmail(email); user != nil {
				key, name = contributorKey{userID: user.ID}, ""
			}
			contributor := contributors[key]
			if contributor == nil {
				contributor = &repo_model.RepoContributor{RepoID: repoID, UserID: key.userID, Email: key.email, Name: name}
				contributors[key] = contributor
			}
			contributor.Commits += author.merges
			for weekStart, stats := range author.weeks {
				week := weeks[contributorWeekKey{key, weekStart}]
				if week == nil {
					week = &repo_model.RepoContributorWeek{RepoID: repoID, UserID: key.userID, Email: key.email, Week: weekStart}
					weeks[contributorWeekKey{key, weekStart}] = week
				}
				week.Commits += stats.commits
				week.Additions += stats.additions
				week.Deletions += stats.deletions
				contributor.Commits += stats.commits
				contributor.Additions += stats.additions
				contributor.Deletions += stats.deletions
			}
		}
	}

	contributorList := slices.SortedFunc(maps.Values(contributors), func(a, b *repo_model.RepoContributor) int {
		return cmp.Or(cmp.Compare(b.Commits, a.Commits), cmp.Compare(a.UserID, b.UserID), strings.Compare(a.Email, b.Email))
	})
	return contributorList, slices.Collect(maps.Values(weeks)), nil
}

func getAuthorStats(ctx context.Context, gitRepo *git.Repository, revision string) (map[string]*authorStats, error) {
	cmd := gitcmd.NewCommand("log", "--shortstat", "--format=%x00%aN%x00%aE%x00%as%x00%P").AddDynamicArguments(revision)
	stdoutReader, stdoutReaderClose := cmd.MakeStdoutPipe()
	defer stdoutReaderClose()

	authors := make(map[string]*authorStats)
	err := cmd.WithRepo(gitRepo).WithPipelineFunc(func(gitcmd.Context) error {
		var week *contributionStats
		scanner := bufio.NewScanner(stdoutReader)
		for scanner.Scan() {
			line := scanner.Text()
			if header, ok := strings.CutPrefix(line, "\x00"); ok {
				name, rest, _ := strings.Cut(header, "\x00")
				email, rest, _ := strings.Cut(rest, "\x00")
				date, parents, _ := strings.Cut(rest, "\x00")
				email = strings.ToLower(strings.TrimSpace(email))
				week = nil
				if email == "" {
					continue
				}
				author := authors[email]
				if author == nil {
					author = &authorStats{name: name, weeks: make(map[int64]*contributionStats)}
					authors[email] = author
				}
				if strings.Contains(parents, " ") {
					author.merges++
					continue
				}
				authorDate, err := time.Parse(time.DateOnly, date)
				if err != nil {
					return err
				}
				weekStart := authorDate.AddDate(0, 0, -int(authorDate.Weekday())).UnixMilli()
				week = author.weeks[weekStart]
				if week == nil {
					week = &contributionStats{}
					author.weeks[weekStart] = week
				}
				week.commits++
			} else if week != nil && line != "" {
				additions, deletions := parseShortStat(line)
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
