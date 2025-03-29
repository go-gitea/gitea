// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"

	"xorm.io/builder"
)

// IssueList defines a list of issues
type IssueList []*Issue

// get the repo IDs to be loaded later, these IDs are for issue.Repo and issue.PullRequest.HeadRepo
func (issues IssueList) getRepoIDs() []int64 {
	return container.FilterSlice(issues, func(issue *Issue) (int64, bool) {
		if issue.Repo == nil {
			return issue.RepoID, true
		}
		if issue.PullRequest != nil && issue.PullRequest.HeadRepo == nil {
			return issue.PullRequest.HeadRepoID, true
		}
		return 0, false
	})
}

// LoadRepositories loads issues' all repositories
func (issues IssueList) LoadRepositories(ctx context.Context) (repo_model.RepositoryList, error) {
	if len(issues) == 0 {
		return nil, nil
	}

	repoIDs := issues.getRepoIDs()
	repoMaps := make(map[int64]*repo_model.Repository, len(repoIDs))
	left := len(repoIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		err := db.GetEngine(ctx).
			In("id", repoIDs[:limit]).
			Find(&repoMaps)
		if err != nil {
			return nil, fmt.Errorf("find repository: %w", err)
		}
		left -= limit
		repoIDs = repoIDs[limit:]
	}

	for _, issue := range issues {
		if issue.Repo == nil {
			issue.Repo = repoMaps[issue.RepoID]
		} else {
			repoMaps[issue.RepoID] = issue.Repo
		}
		if issue.PullRequest != nil {
			issue.PullRequest.BaseRepo = issue.Repo
			if issue.PullRequest.HeadRepo == nil {
				issue.PullRequest.HeadRepo = repoMaps[issue.PullRequest.HeadRepoID]
			}
		}
	}
	return repo_model.ValuesRepository(repoMaps), nil
}

func (issues IssueList) LoadPosters(ctx context.Context) error {
	if len(issues) == 0 {
		return nil
	}

	posterIDs := container.FilterSlice(issues, func(issue *Issue) (int64, bool) {
		return issue.PosterID, issue.Poster == nil && issue.PosterID > 0
	})

	posterMaps, err := user_model.GetUsersMapByIDs(ctx, posterIDs)
	if err != nil {
		return err
	}

	for _, issue := range issues {
		if issue.Poster == nil {
			issue.Poster = user_model.GetPossibleUserFromMap(issue.PosterID, posterMaps)
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

func (issues IssueList) LoadLabels(ctx context.Context) error {
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
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).Table("label").
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
					return fmt.Errorf("IssueList.LoadLabels: Close: %w", err1)
				}
				return err
			}
			issueLabels[labelIssue.IssueLabel.IssueID] = append(issueLabels[labelIssue.IssueLabel.IssueID], labelIssue.Label)
		}
		// When there are no rows left and we try to close it.
		// Since that is not relevant for us, we can safely ignore it.
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("IssueList.LoadLabels: Close: %w", err1)
		}
		left -= limit
		issueIDs = issueIDs[limit:]
	}

	for _, issue := range issues {
		issue.Labels = issueLabels[issue.ID]
		issue.isLabelsLoaded = true
	}
	return nil
}

func (issues IssueList) getMilestoneIDs() []int64 {
	return container.FilterSlice(issues, func(issue *Issue) (int64, bool) {
		return issue.MilestoneID, true
	})
}

func (issues IssueList) LoadMilestones(ctx context.Context) error {
	milestoneIDs := issues.getMilestoneIDs()
	if len(milestoneIDs) == 0 {
		return nil
	}

	milestoneMaps := make(map[int64]*Milestone, len(milestoneIDs))
	left := len(milestoneIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		err := db.GetEngine(ctx).
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
		issue.isMilestoneLoaded = true
	}
	return nil
}

func (issues IssueList) LoadProjects(ctx context.Context) error {
	issueIDs := issues.getIssueIDs()
	projectMaps := make(map[int64]*project_model.Project, len(issues))
	left := len(issueIDs)

	type projectWithIssueID struct {
		*project_model.Project `xorm:"extends"`
		IssueID                int64
	}

	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}

		projects := make([]*projectWithIssueID, 0, limit)
		err := db.GetEngine(ctx).
			Table("project").
			Select("project.*, project_issue.issue_id").
			Join("INNER", "project_issue", "project.id = project_issue.project_id").
			In("project_issue.issue_id", issueIDs[:limit]).
			Find(&projects)
		if err != nil {
			return err
		}
		for _, project := range projects {
			projectMaps[project.IssueID] = project.Project
		}
		left -= limit
		issueIDs = issueIDs[limit:]
	}

	for _, issue := range issues {
		issue.Project = projectMaps[issue.ID]
	}
	return nil
}

func (issues IssueList) LoadAssignees(ctx context.Context) error {
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
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).Table("issue_assignees").
			Join("INNER", "`user`", "`user`.id = `issue_assignees`.assignee_id").
			In("`issue_assignees`.issue_id", issueIDs[:limit]).OrderBy(user_model.GetOrderByName()).
			Rows(new(AssigneeIssue))
		if err != nil {
			return err
		}

		for rows.Next() {
			var assigneeIssue AssigneeIssue
			err = rows.Scan(&assigneeIssue)
			if err != nil {
				if err1 := rows.Close(); err1 != nil {
					return fmt.Errorf("IssueList.loadAssignees: Close: %w", err1)
				}
				return err
			}

			assignees[assigneeIssue.IssueAssignee.IssueID] = append(assignees[assigneeIssue.IssueAssignee.IssueID], assigneeIssue.Assignee)
		}
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("IssueList.loadAssignees: Close: %w", err1)
		}
		left -= limit
		issueIDs = issueIDs[limit:]
	}

	for _, issue := range issues {
		issue.Assignees = assignees[issue.ID]
		if len(issue.Assignees) > 0 {
			issue.Assignee = issue.Assignees[0]
		}
		issue.isAssigneeLoaded = true
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

// LoadPullRequests loads pull requests
func (issues IssueList) LoadPullRequests(ctx context.Context) error {
	issuesIDs := issues.getPullIssueIDs()
	if len(issuesIDs) == 0 {
		return nil
	}

	pullRequestMaps := make(map[int64]*PullRequest, len(issuesIDs))
	left := len(issuesIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
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
					return fmt.Errorf("IssueList.loadPullRequests: Close: %w", err1)
				}
				return err
			}
			pullRequestMaps[pr.IssueID] = &pr
		}
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("IssueList.loadPullRequests: Close: %w", err1)
		}
		left -= limit
		issuesIDs = issuesIDs[limit:]
	}

	for _, issue := range issues {
		issue.PullRequest = pullRequestMaps[issue.ID]
		if issue.PullRequest != nil {
			issue.PullRequest.Issue = issue
		}
	}
	return nil
}

// LoadAttachments loads attachments
func (issues IssueList) LoadAttachments(ctx context.Context) (err error) {
	if len(issues) == 0 {
		return nil
	}

	attachments := make(map[int64][]*repo_model.Attachment, len(issues))
	issuesIDs := issues.getIssueIDs()
	left := len(issuesIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).
			In("issue_id", issuesIDs[:limit]).
			Rows(new(repo_model.Attachment))
		if err != nil {
			return err
		}

		for rows.Next() {
			var attachment repo_model.Attachment
			err = rows.Scan(&attachment)
			if err != nil {
				if err1 := rows.Close(); err1 != nil {
					return fmt.Errorf("IssueList.loadAttachments: Close: %w", err1)
				}
				return err
			}
			attachments[attachment.IssueID] = append(attachments[attachment.IssueID], &attachment)
		}
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("IssueList.loadAttachments: Close: %w", err1)
		}
		left -= limit
		issuesIDs = issuesIDs[limit:]
	}

	for _, issue := range issues {
		issue.Attachments = attachments[issue.ID]
		issue.isAttachmentsLoaded = true
	}
	return nil
}

func (issues IssueList) loadComments(ctx context.Context, cond builder.Cond) (err error) {
	if len(issues) == 0 {
		return nil
	}

	comments := make(map[int64][]*Comment, len(issues))
	issuesIDs := issues.getIssueIDs()
	left := len(issuesIDs)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}
		rows, err := db.GetEngine(ctx).Table("comment").
			Join("INNER", "issue", "issue.id = comment.issue_id").
			In("issue.id", issuesIDs[:limit]).
			Where(cond).
			NoAutoCondition().
			Rows(new(Comment))
		if err != nil {
			return err
		}

		for rows.Next() {
			var comment Comment
			err = rows.Scan(&comment)
			if err != nil {
				if err1 := rows.Close(); err1 != nil {
					return fmt.Errorf("IssueList.loadComments: Close: %w", err1)
				}
				return err
			}
			comments[comment.IssueID] = append(comments[comment.IssueID], &comment)
		}
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("IssueList.loadComments: Close: %w", err1)
		}
		left -= limit
		issuesIDs = issuesIDs[limit:]
	}

	for _, issue := range issues {
		issue.Comments = comments[issue.ID]
	}
	return nil
}

func (issues IssueList) loadTotalTrackedTimes(ctx context.Context) (err error) {
	type totalTimesByIssue struct {
		IssueID int64
		Time    int64
	}
	if len(issues) == 0 {
		return nil
	}
	trackedTimes := make(map[int64]int64, len(issues))

	reposMap := make(map[int64]*repo_model.Repository, len(issues))
	for _, issue := range issues {
		reposMap[issue.RepoID] = issue.Repo
	}
	repos := repo_model.RepositoryListOfMap(reposMap)

	if err := repos.LoadUnits(ctx); err != nil {
		return err
	}

	ids := make([]int64, 0, len(issues))
	for _, issue := range issues {
		if issue.Repo.IsTimetrackerEnabled(ctx) {
			ids = append(ids, issue.ID)
		}
	}

	left := len(ids)
	for left > 0 {
		limit := db.DefaultMaxInSize
		if left < limit {
			limit = left
		}

		// select issue_id, sum(time) from tracked_time where issue_id in (<issue ids in current page>) group by issue_id
		rows, err := db.GetEngine(ctx).Table("tracked_time").
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
					return fmt.Errorf("IssueList.loadTotalTrackedTimes: Close: %w", err1)
				}
				return err
			}
			trackedTimes[totalTime.IssueID] = totalTime.Time
		}
		if err1 := rows.Close(); err1 != nil {
			return fmt.Errorf("IssueList.loadTotalTrackedTimes: Close: %w", err1)
		}
		left -= limit
		ids = ids[limit:]
	}

	for _, issue := range issues {
		issue.TotalTrackedTime = trackedTimes[issue.ID]
	}
	return nil
}

func (issues IssueList) LoadPinOrder(ctx context.Context) error {
	if len(issues) == 0 {
		return nil
	}

	issueIDs := container.FilterSlice(issues, func(issue *Issue) (int64, bool) {
		return issue.ID, issue.PinOrder == 0
	})
	if len(issueIDs) == 0 {
		return nil
	}
	issuePins, err := GetIssuePinsByIssueIDs(ctx, issueIDs)
	if err != nil {
		return err
	}

	for _, issue := range issues {
		if issue.PinOrder != 0 {
			continue
		}
		for _, pin := range issuePins {
			if pin.IssueID == issue.ID {
				issue.PinOrder = pin.PinOrder
				break
			}
		}
		if issue.PinOrder == 0 {
			issue.PinOrder = -1
		}
	}
	return nil
}

// loadAttributes loads all attributes, expect for attachments and comments
func (issues IssueList) LoadAttributes(ctx context.Context) error {
	if _, err := issues.LoadRepositories(ctx); err != nil {
		return fmt.Errorf("issue.loadAttributes: LoadRepositories: %w", err)
	}

	if err := issues.LoadPosters(ctx); err != nil {
		return fmt.Errorf("issue.loadAttributes: LoadPosters: %w", err)
	}

	if err := issues.LoadLabels(ctx); err != nil {
		return fmt.Errorf("issue.loadAttributes: LoadLabels: %w", err)
	}

	if err := issues.LoadMilestones(ctx); err != nil {
		return fmt.Errorf("issue.loadAttributes: LoadMilestones: %w", err)
	}

	if err := issues.LoadProjects(ctx); err != nil {
		return fmt.Errorf("issue.loadAttributes: loadProjects: %w", err)
	}

	if err := issues.LoadAssignees(ctx); err != nil {
		return fmt.Errorf("issue.loadAttributes: loadAssignees: %w", err)
	}

	if err := issues.LoadPullRequests(ctx); err != nil {
		return fmt.Errorf("issue.loadAttributes: loadPullRequests: %w", err)
	}

	if err := issues.loadTotalTrackedTimes(ctx); err != nil {
		return fmt.Errorf("issue.loadAttributes: loadTotalTrackedTimes: %w", err)
	}

	return nil
}

// LoadComments loads comments
func (issues IssueList) LoadComments(ctx context.Context) error {
	return issues.loadComments(ctx, builder.NewCond())
}

// LoadDiscussComments loads discuss comments
func (issues IssueList) LoadDiscussComments(ctx context.Context) error {
	return issues.loadComments(ctx, builder.Eq{"comment.type": CommentTypeComment})
}

// GetApprovalCounts returns a map of issue ID to slice of approval counts
// FIXME: only returns official counts due to double counting of non-official approvals
func (issues IssueList) GetApprovalCounts(ctx context.Context) (map[int64][]*ReviewCount, error) {
	rCounts := make([]*ReviewCount, 0, 2*len(issues))
	ids := make([]int64, len(issues))
	for i, issue := range issues {
		ids[i] = issue.ID
	}
	sess := db.GetEngine(ctx).In("issue_id", ids)
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

func (issues IssueList) LoadIsRead(ctx context.Context, userID int64) error {
	issueIDs := issues.getIssueIDs()
	issueUsers := make([]*IssueUser, 0, len(issueIDs))
	if err := db.GetEngine(ctx).Where("uid =?", userID).
		In("issue_id").
		Find(&issueUsers); err != nil {
		return err
	}

	for _, issueUser := range issueUsers {
		for _, issue := range issues {
			if issue.ID == issueUser.IssueID {
				issue.IsRead = issueUser.IsRead
			}
		}
	}

	return nil
}
