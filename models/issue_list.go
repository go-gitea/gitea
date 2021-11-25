// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"

	"xorm.io/builder"
)

// IssueList defines a list of issues
type IssueList []*Issue

const (
	// default variables number on IN () in SQL
	defaultMaxInSize = 50
)

func (issues IssueList) getRepoIDs() []int64 {
	repoIDs := make(map[int64]struct{}, len(issues))
	for _, issue := range issues {
		if _, ok := repoIDs[issue.RepoID]; !ok {
			repoIDs[issue.RepoID] = struct{}{}
		}
	}
	return keysInt64(repoIDs)
}

func (issues IssueList) loadRepositories(e db.Engine) ([]*Repository, error) {
	if len(issues) == 0 {
		return nil, nil
	}

	repoIDs := issues.getRepoIDs()
	repoMaps := make(map[int64]*Repository, len(repoIDs))
	left := len(repoIDs)
	for left > 0 {
		limit := defaultMaxInSize
		if left < limit {
			limit = left
		}
		err := e.
			In("id", repoIDs[:limit]).
			Find(&repoMaps)
		if err != nil {
			return nil, fmt.Errorf("find repository: %v", err)
		}
		left -= limit
		repoIDs = repoIDs[limit:]
	}

	for _, issue := range issues {
		issue.Repo = repoMaps[issue.RepoID]
		if issue.PullRequest != nil {
			issue.PullRequest.BaseRepo = issue.Repo
		}
	}
	return valuesRepository(repoMaps), nil
}

// LoadRepositories loads issues' all repositories
func (issues IssueList) LoadRepositories() ([]*Repository, error) {
	return issues.loadRepositories(db.GetEngine(db.DefaultContext))
}

func (issues IssueList) getPosterIDs() []int64 {
	posterIDs := make(map[int64]struct{}, len(issues))
	for _, issue := range issues {
		if _, ok := posterIDs[issue.PosterID]; !ok {
			posterIDs[issue.PosterID] = struct{}{}
		}
	}
	return keysInt64(posterIDs)
}

func (issues IssueList) loadPosters(e db.Engine) error {
	if len(issues) == 0 {
		return nil
	}

	posterIDs := issues.getPosterIDs()
	posterMaps := make(map[int64]*user_model.User, len(posterIDs))
	left := len(posterIDs)
	for left > 0 {
		limit := defaultMaxInSize
		if left < limit {
			limit = left
		}
		err := e.
			In("id", posterIDs[:limit]).
			Find(&posterMaps)
		if err != nil {
			return err
		}
		left -= limit
		posterIDs = posterIDs[limit:]
	}

	for _, issue := range issues {
		if issue.PosterID <= 0 {
			continue
		}
		var ok bool
		if issue.Poster, ok = posterMaps[issue.PosterID]; !ok {
			issue.Poster = user_model.NewGhostUser()
		}
	}
	return nil
}

func (issues IssueList) getIssueIDs() []int64 {
	ids := make([]int64, 0, len(issues))
	for _, issue := range issues {
		ids = append(ids, issue.ID)
	}
	return ids
}

func (issues IssueList) loadLabels(e db.Engine) error {
	if len(issues) == 0 {
		return nil
	}

	type LabelIssue struct {
		Label      *Label      `xorm:"extends"`
		IssueLabel *IssueLabel `xorm:"extends"`
	}

	issueLabels := make(map[int64][]*Label, len(issues)*3)
	issueIDs := issues.getIssueIDs()
	left := len(issueIDs)
	for left > 0 {
		limit := defaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := e.Table("label").
			Join("LEFT", "issue_label", "issue_label.label_id = label.id").
			In("issue_label.issue_id", issueIDs[:limit]).
			Asc("label.name").
			Rows(new(LabelIssue))
		if err != nil {
			return err
		}

		for rows.Next() {
			var labelIssue LabelIssue
			err = rows.Scan(&labelIssue)
			if err != nil {
				if err1 := rows.Close(); err1 != nil {
					return fmt.Errorf("IssueList.loadLabels: Close: %v", err1)
				}
				return err
			}
			issueLabels[labelIssue.IssueLabel.IssueID] = append(issueLabels[labelIssue.IssueLabel.IssueID], labelIssue.Label)
		}
		// When there are no rows left and we try to close it.
		// Since that is not relevant for us, we can safely ignore it.
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("IssueList.loadLabels: Close: %v", err1)
		}
		left -= limit
		issueIDs = issueIDs[limit:]
	}

	for _, issue := range issues {
		issue.Labels = issueLabels[issue.ID]
	}
	return nil
}

func (issues IssueList) getMilestoneIDs() []int64 {
	ids := make(map[int64]struct{}, len(issues))
	for _, issue := range issues {
		if _, ok := ids[issue.MilestoneID]; !ok {
			ids[issue.MilestoneID] = struct{}{}
		}
	}
	return keysInt64(ids)
}

func (issues IssueList) loadMilestones(e db.Engine) error {
	milestoneIDs := issues.getMilestoneIDs()
	if len(milestoneIDs) == 0 {
		return nil
	}

	milestoneMaps := make(map[int64]*Milestone, len(milestoneIDs))
	left := len(milestoneIDs)
	for left > 0 {
		limit := defaultMaxInSize
		if left < limit {
			limit = left
		}
		err := e.
			In("id", milestoneIDs[:limit]).
			Find(&milestoneMaps)
		if err != nil {
			return err
		}
		left -= limit
		milestoneIDs = milestoneIDs[limit:]
	}

	for _, issue := range issues {
		issue.Milestone = milestoneMaps[issue.MilestoneID]
	}
	return nil
}

func (issues IssueList) loadAssignees(e db.Engine) error {
	if len(issues) == 0 {
		return nil
	}

	type AssigneeIssue struct {
		IssueAssignee *IssueAssignees  `xorm:"extends"`
		Assignee      *user_model.User `xorm:"extends"`
	}

	assignees := make(map[int64][]*user_model.User, len(issues))
	issueIDs := issues.getIssueIDs()
	left := len(issueIDs)
	for left > 0 {
		limit := defaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := e.Table("issue_assignees").
			Join("INNER", "`user`", "`user`.id = `issue_assignees`.assignee_id").
			In("`issue_assignees`.issue_id", issueIDs[:limit]).
			Rows(new(AssigneeIssue))
		if err != nil {
			return err
		}

		for rows.Next() {
			var assigneeIssue AssigneeIssue
			err = rows.Scan(&assigneeIssue)
			if err != nil {
				if err1 := rows.Close(); err1 != nil {
					return fmt.Errorf("IssueList.loadAssignees: Close: %v", err1)
				}
				return err
			}

			assignees[assigneeIssue.IssueAssignee.IssueID] = append(assignees[assigneeIssue.IssueAssignee.IssueID], assigneeIssue.Assignee)
		}
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("IssueList.loadAssignees: Close: %v", err1)
		}
		left -= limit
		issueIDs = issueIDs[limit:]
	}

	for _, issue := range issues {
		issue.Assignees = assignees[issue.ID]
	}
	return nil
}

func (issues IssueList) getPullIssueIDs() []int64 {
	ids := make([]int64, 0, len(issues))
	for _, issue := range issues {
		if issue.IsPull && issue.PullRequest == nil {
			ids = append(ids, issue.ID)
		}
	}
	return ids
}

func (issues IssueList) loadPullRequests(e db.Engine) error {
	issuesIDs := issues.getPullIssueIDs()
	if len(issuesIDs) == 0 {
		return nil
	}

	pullRequestMaps := make(map[int64]*PullRequest, len(issuesIDs))
	left := len(issuesIDs)
	for left > 0 {
		limit := defaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := e.
			In("issue_id", issuesIDs[:limit]).
			Rows(new(PullRequest))
		if err != nil {
			return err
		}

		for rows.Next() {
			var pr PullRequest
			err = rows.Scan(&pr)
			if err != nil {
				if err1 := rows.Close(); err1 != nil {
					return fmt.Errorf("IssueList.loadPullRequests: Close: %v", err1)
				}
				return err
			}
			pullRequestMaps[pr.IssueID] = &pr
		}
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("IssueList.loadPullRequests: Close: %v", err1)
		}
		left -= limit
		issuesIDs = issuesIDs[limit:]
	}

	for _, issue := range issues {
		issue.PullRequest = pullRequestMaps[issue.ID]
	}
	return nil
}

func (issues IssueList) loadAttachments(e db.Engine) (err error) {
	if len(issues) == 0 {
		return nil
	}

	attachments := make(map[int64][]*repo_model.Attachment, len(issues))
	issuesIDs := issues.getIssueIDs()
	left := len(issuesIDs)
	for left > 0 {
		limit := defaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := e.Table("attachment").
			Join("INNER", "issue", "issue.id = attachment.issue_id").
			In("issue.id", issuesIDs[:limit]).
			Rows(new(repo_model.Attachment))
		if err != nil {
			return err
		}

		for rows.Next() {
			var attachment repo_model.Attachment
			err = rows.Scan(&attachment)
			if err != nil {
				if err1 := rows.Close(); err1 != nil {
					return fmt.Errorf("IssueList.loadAttachments: Close: %v", err1)
				}
				return err
			}
			attachments[attachment.IssueID] = append(attachments[attachment.IssueID], &attachment)
		}
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("IssueList.loadAttachments: Close: %v", err1)
		}
		left -= limit
		issuesIDs = issuesIDs[limit:]
	}

	for _, issue := range issues {
		issue.Attachments = attachments[issue.ID]
	}
	return nil
}

func (issues IssueList) loadComments(e db.Engine, cond builder.Cond) (err error) {
	if len(issues) == 0 {
		return nil
	}

	comments := make(map[int64][]*Comment, len(issues))
	issuesIDs := issues.getIssueIDs()
	left := len(issuesIDs)
	for left > 0 {
		limit := defaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := e.Table("comment").
			Join("INNER", "issue", "issue.id = comment.issue_id").
			In("issue.id", issuesIDs[:limit]).
			Where(cond).
			Rows(new(Comment))
		if err != nil {
			return err
		}

		for rows.Next() {
			var comment Comment
			err = rows.Scan(&comment)
			if err != nil {
				if err1 := rows.Close(); err1 != nil {
					return fmt.Errorf("IssueList.loadComments: Close: %v", err1)
				}
				return err
			}
			comments[comment.IssueID] = append(comments[comment.IssueID], &comment)
		}
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("IssueList.loadComments: Close: %v", err1)
		}
		left -= limit
		issuesIDs = issuesIDs[limit:]
	}

	for _, issue := range issues {
		issue.Comments = comments[issue.ID]
	}
	return nil
}

func (issues IssueList) loadTotalTrackedTimes(e db.Engine) (err error) {
	type totalTimesByIssue struct {
		IssueID int64
		Time    int64
	}
	if len(issues) == 0 {
		return nil
	}
	trackedTimes := make(map[int64]int64, len(issues))

	ids := make([]int64, 0, len(issues))
	for _, issue := range issues {
		if issue.Repo.IsTimetrackerEnabled() {
			ids = append(ids, issue.ID)
		}
	}

	left := len(ids)
	for left > 0 {
		limit := defaultMaxInSize
		if left < limit {
			limit = left
		}

		// select issue_id, sum(time) from tracked_time where issue_id in (<issue ids in current page>) group by issue_id
		rows, err := e.Table("tracked_time").
			Where("deleted = ?", false).
			Select("issue_id, sum(time) as time").
			In("issue_id", ids[:limit]).
			GroupBy("issue_id").
			Rows(new(totalTimesByIssue))
		if err != nil {
			return err
		}

		for rows.Next() {
			var totalTime totalTimesByIssue
			err = rows.Scan(&totalTime)
			if err != nil {
				if err1 := rows.Close(); err1 != nil {
					return fmt.Errorf("IssueList.loadTotalTrackedTimes: Close: %v", err1)
				}
				return err
			}
			trackedTimes[totalTime.IssueID] = totalTime.Time
		}
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("IssueList.loadTotalTrackedTimes: Close: %v", err1)
		}
		left -= limit
		ids = ids[limit:]
	}

	for _, issue := range issues {
		issue.TotalTrackedTime = trackedTimes[issue.ID]
	}
	return nil
}

// loadAttributes loads all attributes, expect for attachments and comments
func (issues IssueList) loadAttributes(e db.Engine) error {
	if _, err := issues.loadRepositories(e); err != nil {
		return fmt.Errorf("issue.loadAttributes: loadRepositories: %v", err)
	}

	if err := issues.loadPosters(e); err != nil {
		return fmt.Errorf("issue.loadAttributes: loadPosters: %v", err)
	}

	if err := issues.loadLabels(e); err != nil {
		return fmt.Errorf("issue.loadAttributes: loadLabels: %v", err)
	}

	if err := issues.loadMilestones(e); err != nil {
		return fmt.Errorf("issue.loadAttributes: loadMilestones: %v", err)
	}

	if err := issues.loadAssignees(e); err != nil {
		return fmt.Errorf("issue.loadAttributes: loadAssignees: %v", err)
	}

	if err := issues.loadPullRequests(e); err != nil {
		return fmt.Errorf("issue.loadAttributes: loadPullRequests: %v", err)
	}

	if err := issues.loadTotalTrackedTimes(e); err != nil {
		return fmt.Errorf("issue.loadAttributes: loadTotalTrackedTimes: %v", err)
	}

	return nil
}

// LoadAttributes loads attributes of the issues, except for attachments and
// comments
func (issues IssueList) LoadAttributes() error {
	return issues.loadAttributes(db.GetEngine(db.DefaultContext))
}

// LoadAttachments loads attachments
func (issues IssueList) LoadAttachments() error {
	return issues.loadAttachments(db.GetEngine(db.DefaultContext))
}

// LoadComments loads comments
func (issues IssueList) LoadComments() error {
	return issues.loadComments(db.GetEngine(db.DefaultContext), builder.NewCond())
}

// LoadDiscussComments loads discuss comments
func (issues IssueList) LoadDiscussComments() error {
	return issues.loadComments(db.GetEngine(db.DefaultContext), builder.Eq{"comment.type": CommentTypeComment})
}

// LoadPullRequests loads pull requests
func (issues IssueList) LoadPullRequests() error {
	return issues.loadPullRequests(db.GetEngine(db.DefaultContext))
}

// GetApprovalCounts returns a map of issue ID to slice of approval counts
// FIXME: only returns official counts due to double counting of non-official approvals
func (issues IssueList) GetApprovalCounts() (map[int64][]*ReviewCount, error) {
	return issues.getApprovalCounts(db.GetEngine(db.DefaultContext))
}

func (issues IssueList) getApprovalCounts(e db.Engine) (map[int64][]*ReviewCount, error) {
	rCounts := make([]*ReviewCount, 0, 2*len(issues))
	ids := make([]int64, len(issues))
	for i, issue := range issues {
		ids[i] = issue.ID
	}
	sess := e.In("issue_id", ids)
	err := sess.Select("issue_id, type, count(id) as `count`").
		Where("official = ? AND dismissed = ?", true, false).
		GroupBy("issue_id, type").
		OrderBy("issue_id").
		Table("review").
		Find(&rCounts)
	if err != nil {
		return nil, err
	}

	approvalCountMap := make(map[int64][]*ReviewCount, len(issues))

	for _, c := range rCounts {
		approvalCountMap[c.IssueID] = append(approvalCountMap[c.IssueID], c)
	}

	return approvalCountMap, nil
}
