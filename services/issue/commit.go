// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/repository"
)

const (
	secondsByMinute = float64(time.Minute / time.Second) // seconds in a minute
	secondsByHour   = 60 * secondsByMinute               // seconds in an hour
	secondsByDay    = 8 * secondsByHour                  // seconds in a day
	secondsByWeek   = 5 * secondsByDay                   // seconds in a week
	secondsByMonth  = 4 * secondsByWeek                  // seconds in a month
)

var reDuration = regexp.MustCompile(`(?i)^(?:(\d+([\.,]\d+)?)(?:mo))?(?:(\d+([\.,]\d+)?)(?:w))?(?:(\d+([\.,]\d+)?)(?:d))?(?:(\d+([\.,]\d+)?)(?:h))?(?:(\d+([\.,]\d+)?)(?:m))?$`)

// timeLogToAmount parses time log string and returns amount in seconds
func timeLogToAmount(str string) int64 {
	matches := reDuration.FindAllStringSubmatch(str, -1)
	if len(matches) == 0 {
		return 0
	}

	match := matches[0]

	var a int64

	// months
	if len(match[1]) > 0 {
		mo, _ := strconv.ParseFloat(strings.Replace(match[1], ",", ".", 1), 64)
		a += int64(mo * secondsByMonth)
	}

	// weeks
	if len(match[3]) > 0 {
		w, _ := strconv.ParseFloat(strings.Replace(match[3], ",", ".", 1), 64)
		a += int64(w * secondsByWeek)
	}

	// days
	if len(match[5]) > 0 {
		d, _ := strconv.ParseFloat(strings.Replace(match[5], ",", ".", 1), 64)
		a += int64(d * secondsByDay)
	}

	// hours
	if len(match[7]) > 0 {
		h, _ := strconv.ParseFloat(strings.Replace(match[7], ",", ".", 1), 64)
		a += int64(h * secondsByHour)
	}

	// minutes
	if len(match[9]) > 0 {
		d, _ := strconv.ParseFloat(strings.Replace(match[9], ",", ".", 1), 64)
		a += int64(d * secondsByMinute)
	}

	return a
}

func issueAddTime(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, time time.Time, timeLog string) error {
	amount := timeLogToAmount(timeLog)
	if amount == 0 {
		return nil
	}

	_, err := issues_model.AddTime(ctx, doer, issue, amount, time)
	return err
}

// getIssueFromRef returns the issue referenced by a ref. Returns a nil *Issue
// if the provided ref references a non-existent issue.
func getIssueFromRef(ctx context.Context, repo *repo_model.Repository, index int64) (*issues_model.Issue, error) {
	issue, err := issues_model.GetIssueByIndex(ctx, repo.ID, index)
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return issue, nil
}

// UpdateIssuesCommit checks if issues are manipulated by commit message.
func UpdateIssuesCommit(ctx context.Context, doer *user_model.User, repo *repo_model.Repository, commits []*repository.PushCommit, branchName string) error {
	// Commits are appended in the reverse order.
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]

		type markKey struct {
			ID     int64
			Action references.XRefAction
		}

		refMarked := make(container.Set[markKey])
		var refRepo *repo_model.Repository
		var refIssue *issues_model.Issue
		var err error
		for _, ref := range references.FindAllIssueReferences(c.Message) {
			// issue is from another repo
			if len(ref.Owner) > 0 && len(ref.Name) > 0 {
				refRepo, err = repo_model.GetRepositoryByOwnerAndName(ctx, ref.Owner, ref.Name)
				if err != nil {
					if repo_model.IsErrRepoNotExist(err) {
						log.Warn("Repository referenced in commit but does not exist: %v", err)
					} else {
						log.Error("repo_model.GetRepositoryByOwnerAndName: %v", err)
					}
					continue
				}
			} else {
				refRepo = repo
			}
			if refIssue, err = getIssueFromRef(ctx, refRepo, ref.Index); err != nil {
				return err
			}
			if refIssue == nil {
				continue
			}

			perm, err := access_model.GetUserRepoPermission(ctx, refRepo, doer)
			if err != nil {
				return err
			}

			key := markKey{ID: refIssue.ID, Action: ref.Action}
			if !refMarked.Add(key) {
				continue
			}

			// FIXME: this kind of condition is all over the code, it should be consolidated in a single place
			canclose := perm.IsAdmin() || perm.IsOwner() || perm.CanWriteIssuesOrPulls(refIssue.IsPull) || refIssue.PosterID == doer.ID
			cancomment := canclose || perm.CanReadIssuesOrPulls(refIssue.IsPull)

			// Don't proceed if the user can't comment
			if !cancomment {
				continue
			}

			message := fmt.Sprintf(`<a href="%s/commit/%s">%s</a>`, html.EscapeString(repo.Link()), html.EscapeString(url.PathEscape(c.Sha1)), html.EscapeString(strings.SplitN(c.Message, "\n", 2)[0]))
			if err = CreateRefComment(ctx, doer, refRepo, refIssue, message, c.Sha1); err != nil {
				if errors.Is(err, user_model.ErrBlockedUser) {
					continue
				}
				return err
			}

			// Only issues can be closed/reopened this way, and user needs the correct permissions
			if refIssue.IsPull || !canclose {
				continue
			}

			// Only process closing/reopening keywords
			if ref.Action != references.XRefActionCloses && ref.Action != references.XRefActionReopens {
				continue
			}

			if !repo.CloseIssuesViaCommitInAnyBranch {
				// If the issue was specified to be in a particular branch, don't allow commits in other branches to close it
				if refIssue.Ref != "" {
					issueBranchName := strings.TrimPrefix(refIssue.Ref, git.BranchPrefix)
					if branchName != issueBranchName {
						continue
					}
					// Otherwise, only process commits to the default branch
				} else if branchName != repo.DefaultBranch {
					continue
				}
			}

			refIssue.Repo = refRepo
			if ref.Action == references.XRefActionCloses && !refIssue.IsClosed {
				if len(ref.TimeLog) > 0 {
					if err := issueAddTime(ctx, refIssue, doer, c.Timestamp, ref.TimeLog); err != nil {
						return err
					}
				}
				if err := CloseIssue(ctx, refIssue, doer, c.Sha1); err != nil {
					return err
				}
			} else if ref.Action == references.XRefActionReopens && refIssue.IsClosed {
				if err := ReopenIssue(ctx, refIssue, doer, c.Sha1); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
