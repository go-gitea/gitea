// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
)

// DeleteNotPassedAssignee deletes all assignees who aren't passed via the "assignees" array
func DeleteNotPassedAssignee(issue *models.Issue, doer *models.User, assignees []*models.User) (err error) {
	var found bool

	for _, assignee := range issue.Assignees {

		found = false
		for _, alreadyAssignee := range assignees {
			if assignee.ID == alreadyAssignee.ID {
				found = true
				break
			}
		}

		if !found {
			// This function also does comments and hooks, which is why we call it seperatly instead of directly removing the assignees here
			if _, _, err := ToggleAssignee(issue, doer, assignee.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// ToggleAssignee changes a user between assigned and not assigned for this issue, and make issue comment for it.
func ToggleAssignee(issue *models.Issue, doer *models.User, assigneeID int64) (removed bool, comment *models.Comment, err error) {
	removed, comment, err = issue.ToggleAssignee(doer, assigneeID)
	if err != nil {
		return
	}

	assignee, err1 := models.GetUserByID(assigneeID)
	if err1 != nil {
		err = err1
		return
	}

	notification.NotifyIssueChangeAssignee(doer, issue, assignee, removed, comment)

	return
}

// ReviewRequest add or remove a review for this PR, and make comment for it.
func ReviewRequest(issue *models.Issue, doer *models.User, reviewer *models.User, isAdd bool) (comment *models.Comment, err error) {
	if isAdd {
		comment, err = models.AddReviewRequest(issue, reviewer, doer)
	} else {
		comment, err = models.RemoveReviewRequest(issue, reviewer, doer)
	}

	if err != nil {
		return
	}

	if comment != nil {
		notification.NotifyPullReviewRequest(doer, issue, reviewer, isAdd, comment)
	}

	return
}

// IsLegalReviewRequest check review request permission
func IsLegalReviewRequest(reviewer, doer *models.User, isAdd bool, issue *models.Issue) error {
	if !issue.IsPull {
		return fmt.Errorf("this issue is not Pull Request [issue_id: %d]", issue.ID)
	}

	if err := issue.LoadRepo(); err != nil {
		return err
	}

	if reviewer.IsOrganization() {
		return fmt.Errorf("Organization can't be added as reviewer [user_id: %d, repo_id: %d]", reviewer.ID, issue.Repo.ID)
	}
	if doer.IsOrganization() {
		return fmt.Errorf("Organization can't be doer to add reviewer [user_id: %d, repo_id: %d]", doer.ID, issue.Repo.ID)
	}

	permReviewer, err := models.GetUserRepoPermission(issue.Repo, reviewer)
	if err != nil {
		return err
	}

	permDoer, err := models.GetUserRepoPermission(issue.Repo, doer)
	if err != nil {
		return err
	}

	lastreview, err := models.GetReviewerByIssueIDAndUserID(issue.ID, reviewer.ID)
	if err != nil {
		return err
	}

	var pemResult bool
	if isAdd {
		pemResult = permReviewer.CanAccessAny(models.AccessModeRead, models.UnitTypePullRequests)
		if !pemResult {
			return fmt.Errorf("Reviewer can't read [user_id: %d, repo_name: %s]", reviewer.ID, issue.Repo.Name)
		}

		pemResult = permDoer.CanAccessAny(models.AccessModeWrite, models.UnitTypePullRequests)
		if !pemResult {
			pemResult, err = models.IsOfficialReviewer(issue, doer)
			if err != nil {
				return err
			}
			if !pemResult {
				return fmt.Errorf("Doer can't choose reviewer [user_id: %d, repo_name: %s, issue_id: %d]", doer.ID, issue.Repo.Name, issue.ID)
			}
		}

		if doer.ID == reviewer.ID {
			return fmt.Errorf("doer can't be reviewer [user_id: %d, repo_name: %s]", doer.ID, issue.Repo.Name)
		}

		if reviewer.ID == issue.PosterID {
			return fmt.Errorf("poster of pr can't be reviewer [user_id: %d, repo_name: %s]", reviewer.ID, issue.Repo.Name)
		}
	} else {
		if lastreview.Type == models.ReviewTypeRequest && lastreview.ReviewerID == doer.ID {
			return nil
		}

		pemResult = permDoer.IsAdmin()
		if !pemResult {
			return fmt.Errorf("Doer is not admin [user_id: %d, repo_name: %s]", doer.ID, issue.Repo.Name)
		}
	}

	return nil
}
