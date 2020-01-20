// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/setting"
)

// getIssueFromRef returns the issue referenced by a ref. Returns a nil *Issue
// if the provided ref references a non-existent issue.
func getIssueFromRef(repo *models.Repository, index int64) (*models.Issue, error) {
	issue, err := models.GetIssueByIndex(repo.ID, index)
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return issue, nil
}

func changeIssueStatus(repo *models.Repository, issue *models.Issue, doer *models.User, closed bool) error {
	stopTimerIfAvailable := func(doer *models.User, issue *models.Issue) error {

		if models.StopwatchExists(doer.ID, issue.ID) {
			if err := models.CreateOrStopIssueStopwatch(doer, issue); err != nil {
				return err
			}
		}

		return nil
	}

	issue.Repo = repo
	comment, err := issue.ChangeStatus(doer, closed)
	if err != nil {
		// Don't return an error when dependencies are open as this would let the push fail
		if models.IsErrDependenciesLeft(err) {
			return stopTimerIfAvailable(doer, issue)
		}
		return err
	}

	notification.NotifyIssueChangeStatus(doer, issue, comment, closed)

	return stopTimerIfAvailable(doer, issue)
}

// UpdateIssuesCommit checks if issues are manipulated by commit message.
func UpdateIssuesCommit(doer *models.User, repo *models.Repository, commits []*models.PushCommit, branchName string) error {
	// Commits are appended in the reverse order.
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]

		type markKey struct {
			ID     int64
			Action references.XRefAction
		}

		refMarked := make(map[markKey]bool)
		var refRepo *models.Repository
		var refIssue *models.Issue
		var err error
		for _, ref := range references.FindAllIssueReferences(c.Message) {

			// issue is from another repo
			if len(ref.Owner) > 0 && len(ref.Name) > 0 {
				refRepo, err = models.GetRepositoryFromMatch(ref.Owner, ref.Name)
				if err != nil {
					continue
				}
			} else {
				refRepo = repo
			}
			if refIssue, err = getIssueFromRef(refRepo, ref.Index); err != nil {
				return err
			}
			if refIssue == nil {
				continue
			}

			perm, err := models.GetUserRepoPermission(refRepo, doer)
			if err != nil {
				return err
			}

			key := markKey{ID: refIssue.ID, Action: ref.Action}
			if refMarked[key] {
				continue
			}
			refMarked[key] = true

			// FIXME: this kind of condition is all over the code, it should be consolidated in a single place
			canclose := perm.IsAdmin() || perm.IsOwner() || perm.CanWriteIssuesOrPulls(refIssue.IsPull) || refIssue.PosterID == doer.ID
			cancomment := canclose || perm.CanReadIssuesOrPulls(refIssue.IsPull)

			// Don't proceed if the user can't comment
			if !cancomment {
				continue
			}

			message := fmt.Sprintf(`<a href="%s/commit/%s">%s</a>`, repo.Link(), c.Sha1, html.EscapeString(c.Message))
			if err = models.CreateRefComment(doer, refRepo, refIssue, message, c.Sha1); err != nil {
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
					if branchName != refIssue.Ref {
						continue
					}
					// Otherwise, only process commits to the default branch
				} else if branchName != repo.DefaultBranch {
					continue
				}
			}
			close := (ref.Action == references.XRefActionCloses)
			if close != refIssue.IsClosed {
				if err := changeIssueStatus(refRepo, refIssue, doer, close); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// CommitRepoActionOptions represent options of a new commit action.
type CommitRepoActionOptions struct {
	PusherName  string
	RepoOwnerID int64
	RepoName    string
	RefFullName string
	OldCommitID string
	NewCommitID string
	Commits     *models.PushCommits
}

// CommitRepoAction adds new commit action to the repository, and prepare
// corresponding webhooks.
func CommitRepoAction(optsList ...*CommitRepoActionOptions) error {
	var pusher *models.User
	var repo *models.Repository
	actions := make([]*models.Action, len(optsList))

	for i, opts := range optsList {
		if pusher == nil || pusher.Name != opts.PusherName {
			var err error
			pusher, err = models.GetUserByName(opts.PusherName)
			if err != nil {
				return fmt.Errorf("GetUserByName [%s]: %v", opts.PusherName, err)
			}
		}

		if repo == nil || repo.OwnerID != opts.RepoOwnerID || repo.Name != opts.RepoName {
			var err error
			if repo != nil {
				// Change repository empty status and update last updated time.
				if err := models.UpdateRepository(repo, false); err != nil {
					return fmt.Errorf("UpdateRepository: %v", err)
				}
			}
			repo, err = models.GetRepositoryByName(opts.RepoOwnerID, opts.RepoName)
			if err != nil {
				return fmt.Errorf("GetRepositoryByName [owner_id: %d, name: %s]: %v", opts.RepoOwnerID, opts.RepoName, err)
			}
		}
		refName := git.RefEndName(opts.RefFullName)

		// Change default branch and empty status only if pushed ref is non-empty branch.
		if repo.IsEmpty && opts.NewCommitID != git.EmptySHA && strings.HasPrefix(opts.RefFullName, git.BranchPrefix) {
			repo.DefaultBranch = refName
			repo.IsEmpty = false
			if refName != "master" {
				gitRepo, err := git.OpenRepository(repo.RepoPath())
				if err != nil {
					return err
				}
				if err := gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
					if !git.IsErrUnsupportedVersion(err) {
						gitRepo.Close()
						return err
					}
				}
				gitRepo.Close()
			}
		}

		isNewBranch := false
		opType := models.ActionCommitRepo

		// Check it's tag push or branch.
		if strings.HasPrefix(opts.RefFullName, git.TagPrefix) {
			opType = models.ActionPushTag
			if opts.NewCommitID == git.EmptySHA {
				opType = models.ActionDeleteTag
			}
			opts.Commits = &models.PushCommits{}
		} else if opts.NewCommitID == git.EmptySHA {
			opType = models.ActionDeleteBranch
			opts.Commits = &models.PushCommits{}
		} else {
			// if not the first commit, set the compare URL.
			if opts.OldCommitID == git.EmptySHA {
				isNewBranch = true
			} else {
				opts.Commits.CompareURL = repo.ComposeCompareURL(opts.OldCommitID, opts.NewCommitID)
			}

			if err := UpdateIssuesCommit(pusher, repo, opts.Commits.Commits, refName); err != nil {
				log.Error("updateIssuesCommit: %v", err)
			}
		}

		if len(opts.Commits.Commits) > setting.UI.FeedMaxCommitNum {
			opts.Commits.Commits = opts.Commits.Commits[:setting.UI.FeedMaxCommitNum]
		}

		data, err := json.Marshal(opts.Commits)
		if err != nil {
			return fmt.Errorf("Marshal: %v", err)
		}

		actions[i] = &models.Action{
			ActUserID: pusher.ID,
			ActUser:   pusher,
			OpType:    opType,
			Content:   string(data),
			RepoID:    repo.ID,
			Repo:      repo,
			RefName:   refName,
			IsPrivate: repo.IsPrivate,
		}

		var isHookEventPush = true
		switch opType {
		case models.ActionCommitRepo: // Push
			if isNewBranch {
				notification.NotifyCreateRef(pusher, repo, "branch", opts.RefFullName)
			}
		case models.ActionDeleteBranch: // Delete Branch
			notification.NotifyDeleteRef(pusher, repo, "branch", opts.RefFullName)

		case models.ActionPushTag: // Create
			notification.NotifyCreateRef(pusher, repo, "tag", opts.RefFullName)

		case models.ActionDeleteTag: // Delete Tag
			notification.NotifyDeleteRef(pusher, repo, "tag", opts.RefFullName)
		default:
			isHookEventPush = false
		}

		if isHookEventPush {
			notification.NotifyPushCommits(pusher, repo, opts.RefFullName, opts.OldCommitID, opts.NewCommitID, opts.Commits)
		}
	}

	if repo != nil {
		// Change repository empty status and update last updated time.
		if err := models.UpdateRepository(repo, false); err != nil {
			return fmt.Errorf("UpdateRepository: %v", err)
		}
	}

	if err := models.NotifyWatchers(actions...); err != nil {
		return fmt.Errorf("NotifyWatchers: %v", err)
	}
	return nil
}
