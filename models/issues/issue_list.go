// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"fmt"

	"gitea.dev/models/db"
	project_model "gitea.dev/models/project"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"

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
	err := db.QueryAndProcessByChunk(repoIDs, func(chunk []int64) (out []*repo_model.Repository, err error) {
		err = db.GetEngine(ctx).In("id", chunk).Find(&out)
		return out, err
	}, func(repo *repo_model.Repository) error {
		repoMaps[repo.ID] = repo
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find repository: %w", err)
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

	err := db.QueryAndProcessByChunk(issueIDs, func(chunk []int64) (out []*LabelIssue, err error) {
		err = db.GetEngine(ctx).Table("label").
			Join("LEFT", "issue_label", "issue_label.label_id = label.id").
			In("issue_label.issue_id", chunk).
			Asc("label.name").
			Find(&out)
		return out, err
	}, func(out *LabelIssue) error {
		issueLabels[out.IssueLabel.IssueID] = append(issueLabels[out.IssueLabel.IssueID], out.Label)
		return nil
	})
	if err != nil {
		return fmt.Errorf("IssueList.LoadLabels: %w", err)
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
	err := db.QueryAndProcessByChunk(milestoneIDs, func(chunk []int64) (out []*Milestone, err error) {
		err = db.GetEngine(ctx).In("id", chunk).Find(&out)
		return out, err
	}, func(milestone *Milestone) error {
		milestoneMaps[milestone.ID] = milestone
		return nil
	})
	if err != nil {
		return err
	}

	for _, issue := range issues {
		issue.Milestone = milestoneMaps[issue.MilestoneID]
		issue.isMilestoneLoaded = true
	}
	return nil
}

func (issues IssueList) LoadProjects(ctx context.Context) error {
	issueIDs := issues.getIssueIDs()
	issueProjectMaps := make(map[int64][]*project_model.Project, len(issues))

	type projectWithIssueID struct {
		*project_model.Project `xorm:"extends"`
		IssueID                int64
	}

	err := db.QueryAndProcessByChunk(issueIDs, func(chunk []int64) (out []*projectWithIssueID, err error) {
		err = db.GetEngine(ctx).
			Table("project").
			Select("project.*, project_issue.issue_id").
			Join("INNER", "project_issue", "project.id = project_issue.project_id").
			In("project_issue.issue_id", chunk).
			OrderBy("project_issue.issue_id ASC, project.id ASC").
			Find(&out)
		return out, err
	}, func(project *projectWithIssueID) error {
		issueProjectMaps[project.IssueID] = append(issueProjectMaps[project.IssueID], project.Project)
		return nil
	})
	if err != nil {
		return fmt.Errorf("IssueList.LoadProjects: %w", err)
	}

	for _, issue := range issues {
		issue.Projects = issueProjectMaps[issue.ID]
		issue.isProjectsLoaded = true
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
	err := db.QueryAndProcessByChunk(issueIDs, func(chunk []int64) (out []*AssigneeIssue, err error) {
		err = db.GetEngine(ctx).Table("issue_assignees").
			Join("INNER", "`user`", "`user`.id = `issue_assignees`.assignee_id").
			In("`issue_assignees`.issue_id", chunk).OrderBy(user_model.GetOrderByName()).
			Find(&out)
		return out, err
	}, func(assigneeIssue *AssigneeIssue) error {
		assignees[assigneeIssue.IssueAssignee.IssueID] = append(assignees[assigneeIssue.IssueAssignee.IssueID], assigneeIssue.Assignee)
		return nil
	})
	if err != nil {
		return fmt.Errorf("IssueList.LoadAssignees: %w", err)
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
	err := db.QueryAndProcessByChunk(issuesIDs, func(chunk []int64) (out []*PullRequest, err error) {
		err = db.GetEngine(ctx).
			In("issue_id", chunk).
			Find(&out)
		return out, err
	}, func(pr *PullRequest) error {
		pullRequestMaps[pr.IssueID] = pr
		return nil
	})
	if err != nil {
		return fmt.Errorf("IssueList.LoadPullRequests: %w", err)
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
	err = db.QueryAndProcessByChunk(issuesIDs, func(chunk []int64) (out []*repo_model.Attachment, err error) {
		err = db.GetEngine(ctx).In("issue_id", chunk).Find(&out)
		return out, err
	}, func(attachment *repo_model.Attachment) error {
		attachments[attachment.IssueID] = append(attachments[attachment.IssueID], attachment)
		return nil
	})
	if err != nil {
		return fmt.Errorf("IssueList.LoadAttachments: %w", err)
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
	err = db.QueryAndProcessByChunk(issuesIDs, func(chunk []int64) (out []*Comment, err error) {
		err = db.GetEngine(ctx).Table("comment").
			Join("INNER", "issue", "issue.id = comment.issue_id").
			In("issue.id", chunk).
			Where(cond).
			NoAutoCondition().
			Find(&out)
		return out, err
	}, func(comment *Comment) error {
		comments[comment.IssueID] = append(comments[comment.IssueID], comment)
		return nil
	})
	if err != nil {
		return fmt.Errorf("IssueList.loadComments: %w", err)
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

	err = db.QueryAndProcessByChunk(ids, func(chunk []int64) (out []*totalTimesByIssue, err error) {
		// select issue_id, sum(time) from tracked_time where issue_id in (<issue ids in current page>) group by issue_id
		err = db.GetEngine(ctx).Table("tracked_time").
			Where("deleted = ?", false).
			Select("issue_id, sum(time) as time").
			In("issue_id", chunk).
			GroupBy("issue_id").
			Find(&out)
		return out, err
	}, func(totalTime *totalTimesByIssue) error {
		trackedTimes[totalTime.IssueID] = totalTime.Time
		return nil
	})
	if err != nil {
		return fmt.Errorf("IssueList.loadTotalTrackedTimes: %w", err)
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
	if len(issueIDs) == 0 {
		return nil
	}
	issueUsers := make([]*IssueUser, 0, len(issueIDs))
	if err := db.GetEngine(ctx).Where("uid =?", userID).
		In("issue_id", issueIDs).
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
