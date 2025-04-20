// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"sort"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/optional"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
)

type issueSidebarMilestoneData struct {
	SelectedMilestoneID int64
	OpenMilestones      []*issues_model.Milestone
	ClosedMilestones    []*issues_model.Milestone
}

type issueSidebarAssigneesData struct {
	SelectedAssigneeIDs string
	CandidateAssignees  []*user_model.User
}

type issueSidebarProjectsData struct {
	SelectedProjectID int64
	OpenProjects      []*project_model.Project
	ClosedProjects    []*project_model.Project
}

type IssuePageMetaData struct {
	RepoLink             string
	Repository           *repo_model.Repository
	Issue                *issues_model.Issue
	IsPullRequest        bool
	CanModifyIssueOrPull bool

	ReviewersData  *issueSidebarReviewersData
	LabelsData     *issueSidebarLabelsData
	MilestonesData *issueSidebarMilestoneData
	ProjectsData   *issueSidebarProjectsData
	AssigneesData  *issueSidebarAssigneesData
}

func retrieveRepoIssueMetaData(ctx *context.Context, repo *repo_model.Repository, issue *issues_model.Issue, isPull bool) *IssuePageMetaData {
	data := &IssuePageMetaData{
		RepoLink:      ctx.Repo.RepoLink,
		Repository:    repo,
		Issue:         issue,
		IsPullRequest: isPull,

		ReviewersData:  &issueSidebarReviewersData{},
		LabelsData:     &issueSidebarLabelsData{},
		MilestonesData: &issueSidebarMilestoneData{},
		ProjectsData:   &issueSidebarProjectsData{},
		AssigneesData:  &issueSidebarAssigneesData{},
	}
	ctx.Data["IssuePageMetaData"] = data

	if isPull {
		data.retrieveReviewersData(ctx)
		if ctx.Written() {
			return data
		}
	}
	data.retrieveLabelsData(ctx)
	if ctx.Written() {
		return data
	}

	// it sets "Branches" template data,
	// it is used to render the "edit PR target branches" dropdown, and the "branch selector" in the issue's sidebar.
	PrepareBranchList(ctx)
	if ctx.Written() {
		return data
	}

	// it sets the "Assignees" template data, and the data is also used to "mention" users.
	data.retrieveAssigneesData(ctx)
	if ctx.Written() {
		return data
	}

	// TODO: the issue/pull permissions are quite complex and unclear
	// A reader could create an issue/PR with setting some meta (eg: assignees from issue template, reviewers, target branch)
	// A reader(creator) could update some meta (eg: target branch), but can't change assignees anymore.
	// For non-creator users, only writers could update some meta (eg: assignees, milestone, project)
	// Need to clarify the logic and add some tests in the future
	data.CanModifyIssueOrPull = ctx.Repo.CanWriteIssuesOrPulls(isPull) && !ctx.Repo.Repository.IsArchived
	if !data.CanModifyIssueOrPull {
		return data
	}

	data.retrieveMilestonesDataForIssueWriter(ctx)
	if ctx.Written() {
		return data
	}

	data.retrieveProjectsDataForIssueWriter(ctx)
	if ctx.Written() {
		return data
	}

	ctx.Data["CanCreateIssueDependencies"] = ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, isPull)
	return data
}

func (d *IssuePageMetaData) retrieveMilestonesDataForIssueWriter(ctx *context.Context) {
	var err error
	if d.Issue != nil {
		d.MilestonesData.SelectedMilestoneID = d.Issue.MilestoneID
	}
	d.MilestonesData.OpenMilestones, err = db.Find[issues_model.Milestone](ctx, issues_model.FindMilestoneOptions{
		RepoID:   d.Repository.ID,
		IsClosed: optional.Some(false),
	})
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}
	d.MilestonesData.ClosedMilestones, err = db.Find[issues_model.Milestone](ctx, issues_model.FindMilestoneOptions{
		RepoID:   d.Repository.ID,
		IsClosed: optional.Some(true),
	})
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}
}

func (d *IssuePageMetaData) retrieveAssigneesData(ctx *context.Context) {
	var err error
	d.AssigneesData.CandidateAssignees, err = repo_model.GetRepoAssignees(ctx, d.Repository)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	d.AssigneesData.CandidateAssignees = shared_user.MakeSelfOnTop(ctx.Doer, d.AssigneesData.CandidateAssignees)
	if d.Issue != nil {
		_ = d.Issue.LoadAssignees(ctx)
		ids := make([]string, 0, len(d.Issue.Assignees))
		for _, a := range d.Issue.Assignees {
			ids = append(ids, strconv.FormatInt(a.ID, 10))
		}
		d.AssigneesData.SelectedAssigneeIDs = strings.Join(ids, ",")
	}
	// FIXME: this is a tricky part which writes ctx.Data["Mentionable*"]
	handleMentionableAssigneesAndTeams(ctx, d.AssigneesData.CandidateAssignees)
}

func (d *IssuePageMetaData) retrieveProjectsDataForIssueWriter(ctx *context.Context) {
	if d.Issue != nil && d.Issue.Project != nil {
		d.ProjectsData.SelectedProjectID = d.Issue.Project.ID
	}
	d.ProjectsData.OpenProjects, d.ProjectsData.ClosedProjects = retrieveProjectsInternal(ctx, ctx.Repo.Repository)
}

// repoReviewerSelection items to bee shown
type repoReviewerSelection struct {
	IsTeam         bool
	Team           *organization.Team
	User           *user_model.User
	Review         *issues_model.Review
	CanBeDismissed bool
	CanChange      bool
	Requested      bool
	ItemID         int64
}

type issueSidebarReviewersData struct {
	CanChooseReviewer    bool
	OriginalReviews      issues_model.ReviewList
	TeamReviewers        []*repoReviewerSelection
	Reviewers            []*repoReviewerSelection
	CurrentPullReviewers []*repoReviewerSelection
}

// RetrieveRepoReviewers find all reviewers of a repository. If issue is nil, it means the doer is creating a new PR.
func (d *IssuePageMetaData) retrieveReviewersData(ctx *context.Context) {
	data := d.ReviewersData
	repo := d.Repository
	if ctx.Doer != nil && ctx.IsSigned {
		if d.Issue == nil {
			data.CanChooseReviewer = true
		} else {
			data.CanChooseReviewer = issue_service.CanDoerChangeReviewRequests(ctx, ctx.Doer, repo, d.Issue.PosterID)
		}
	}

	var posterID int64
	var isClosed bool
	var reviews issues_model.ReviewList
	var err error

	if d.Issue == nil {
		if ctx.Doer != nil {
			posterID = ctx.Doer.ID
		}
	} else {
		posterID = d.Issue.PosterID
		if d.Issue.OriginalAuthorID > 0 {
			posterID = 0 // for migrated PRs, no poster ID
		}

		isClosed = d.Issue.IsClosed || d.Issue.PullRequest.HasMerged

		reviews, data.OriginalReviews, err = issues_model.GetReviewsByIssueID(ctx, d.Issue.ID)
		if err != nil {
			ctx.ServerError("GetReviewersByIssueID", err)
			return
		}
		if len(reviews) == 0 && !data.CanChooseReviewer {
			return
		}
	}

	var (
		pullReviews         []*repoReviewerSelection
		reviewersResult     []*repoReviewerSelection
		teamReviewersResult []*repoReviewerSelection
		teamReviewers       []*organization.Team
		reviewers           []*user_model.User
	)

	if data.CanChooseReviewer {
		var err error
		reviewers, err = pull_service.GetReviewers(ctx, repo, ctx.Doer.ID, posterID)
		if err != nil {
			ctx.ServerError("GetReviewers", err)
			return
		}

		teamReviewers, err = pull_service.GetReviewerTeams(ctx, repo)
		if err != nil {
			ctx.ServerError("GetReviewerTeams", err)
			return
		}

		if len(reviewers) > 0 {
			reviewersResult = make([]*repoReviewerSelection, 0, len(reviewers))
		}

		if len(teamReviewers) > 0 {
			teamReviewersResult = make([]*repoReviewerSelection, 0, len(teamReviewers))
		}
	}

	pullReviews = make([]*repoReviewerSelection, 0, len(reviews))

	for _, review := range reviews {
		tmp := &repoReviewerSelection{
			Requested: review.Type == issues_model.ReviewTypeRequest,
			Review:    review,
			ItemID:    review.ReviewerID,
		}
		if review.ReviewerTeamID > 0 {
			tmp.IsTeam = true
			tmp.ItemID = -review.ReviewerTeamID
		}

		if data.CanChooseReviewer {
			// Users who can choose reviewers can also remove review requests
			tmp.CanChange = true
		} else if ctx.Doer != nil && ctx.Doer.ID == review.ReviewerID && review.Type == issues_model.ReviewTypeRequest {
			// A user can refuse review requests
			tmp.CanChange = true
		}

		pullReviews = append(pullReviews, tmp)

		if data.CanChooseReviewer {
			if tmp.IsTeam {
				teamReviewersResult = append(teamReviewersResult, tmp)
			} else {
				reviewersResult = append(reviewersResult, tmp)
			}
		}
	}

	if len(pullReviews) > 0 {
		// Drop all non-existing users and teams from the reviews
		currentPullReviewers := make([]*repoReviewerSelection, 0, len(pullReviews))
		for _, item := range pullReviews {
			if item.Review.ReviewerID > 0 {
				if err := item.Review.LoadReviewer(ctx); err != nil {
					if user_model.IsErrUserNotExist(err) {
						continue
					}
					ctx.ServerError("LoadReviewer", err)
					return
				}
				item.User = item.Review.Reviewer
			} else if item.Review.ReviewerTeamID > 0 {
				if err := item.Review.LoadReviewerTeam(ctx); err != nil {
					if organization.IsErrTeamNotExist(err) {
						continue
					}
					ctx.ServerError("LoadReviewerTeam", err)
					return
				}
				item.Team = item.Review.ReviewerTeam
			} else {
				continue
			}
			item.CanBeDismissed = ctx.Repo.Permission.IsAdmin() && !isClosed &&
				(item.Review.Type == issues_model.ReviewTypeApprove || item.Review.Type == issues_model.ReviewTypeReject)
			currentPullReviewers = append(currentPullReviewers, item)
		}
		data.CurrentPullReviewers = currentPullReviewers
	}

	if data.CanChooseReviewer && reviewersResult != nil {
		preadded := len(reviewersResult)
		for _, reviewer := range reviewers {
			found := false
		reviewAddLoop:
			for _, tmp := range reviewersResult[:preadded] {
				if tmp.ItemID == reviewer.ID {
					tmp.User = reviewer
					found = true
					break reviewAddLoop
				}
			}

			if found {
				continue
			}

			reviewersResult = append(reviewersResult, &repoReviewerSelection{
				IsTeam:    false,
				CanChange: true,
				User:      reviewer,
				ItemID:    reviewer.ID,
			})
		}

		data.Reviewers = reviewersResult
	}

	if data.CanChooseReviewer && teamReviewersResult != nil {
		preadded := len(teamReviewersResult)
		for _, team := range teamReviewers {
			found := false
		teamReviewAddLoop:
			for _, tmp := range teamReviewersResult[:preadded] {
				if tmp.ItemID == -team.ID {
					tmp.Team = team
					found = true
					break teamReviewAddLoop
				}
			}

			if found {
				continue
			}

			teamReviewersResult = append(teamReviewersResult, &repoReviewerSelection{
				IsTeam:    true,
				CanChange: true,
				Team:      team,
				ItemID:    -team.ID,
			})
		}

		data.TeamReviewers = teamReviewersResult
	}
}

type issueSidebarLabelsData struct {
	AllLabels        []*issues_model.Label
	RepoLabels       []*issues_model.Label
	OrgLabels        []*issues_model.Label
	SelectedLabelIDs string
}

func makeSelectedStringIDs[KeyType, ItemType comparable](
	allLabels []*issues_model.Label, candidateKey func(candidate *issues_model.Label) KeyType,
	selectedItems []ItemType, selectedKey func(selected ItemType) KeyType,
) string {
	selectedIDSet := make(container.Set[string])
	allLabelMap := map[KeyType]*issues_model.Label{}
	for _, label := range allLabels {
		allLabelMap[candidateKey(label)] = label
	}
	for _, item := range selectedItems {
		if label, ok := allLabelMap[selectedKey(item)]; ok {
			label.IsChecked = true
			selectedIDSet.Add(strconv.FormatInt(label.ID, 10))
		}
	}
	ids := selectedIDSet.Values()
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

func (d *issueSidebarLabelsData) SetSelectedLabels(labels []*issues_model.Label) {
	d.SelectedLabelIDs = makeSelectedStringIDs(
		d.AllLabels, func(label *issues_model.Label) int64 { return label.ID },
		labels, func(label *issues_model.Label) int64 { return label.ID },
	)
}

func (d *issueSidebarLabelsData) SetSelectedLabelNames(labelNames []string) {
	d.SelectedLabelIDs = makeSelectedStringIDs(
		d.AllLabels, func(label *issues_model.Label) string { return strings.ToLower(label.Name) },
		labelNames, strings.ToLower,
	)
}

func (d *issueSidebarLabelsData) SetSelectedLabelIDs(labelIDs []int64) {
	d.SelectedLabelIDs = makeSelectedStringIDs(
		d.AllLabels, func(label *issues_model.Label) int64 { return label.ID },
		labelIDs, func(labelID int64) int64 { return labelID },
	)
}

func (d *IssuePageMetaData) retrieveLabelsData(ctx *context.Context) {
	repo := d.Repository
	labelsData := d.LabelsData

	labels, err := issues_model.GetLabelsByRepoID(ctx, repo.ID, "", db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return
	}
	labelsData.RepoLabels = labels

	if repo.Owner.IsOrganization() {
		orgLabels, err := issues_model.GetLabelsByOrgID(ctx, repo.Owner.ID, ctx.FormString("sort"), db.ListOptions{})
		if err != nil {
			return
		}
		labelsData.OrgLabels = orgLabels
	}
	labelsData.AllLabels = append(labelsData.AllLabels, labelsData.RepoLabels...)
	labelsData.AllLabels = append(labelsData.AllLabels, labelsData.OrgLabels...)
}
