// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/optional"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/forms"
	issue_service "code.gitea.io/gitea/services/issue"
)

const (
	tplAttachment base.TplName = "repo/issue/view_content/attachments"

	tplIssues      base.TplName = "repo/issue/list"
	tplIssueNew    base.TplName = "repo/issue/new"
	tplIssueChoose base.TplName = "repo/issue/choose"
	tplIssueView   base.TplName = "repo/issue/view"

	tplReactions base.TplName = "repo/issue/view_content/reactions"

	issueTemplateKey      = "IssueTemplate"
	issueTemplateTitleKey = "IssueTemplateTitle"
)

// IssueTemplateCandidates issue templates
var IssueTemplateCandidates = []string{
	"ISSUE_TEMPLATE.md",
	"ISSUE_TEMPLATE.yaml",
	"ISSUE_TEMPLATE.yml",
	"issue_template.md",
	"issue_template.yaml",
	"issue_template.yml",
	".gitea/ISSUE_TEMPLATE.md",
	".gitea/ISSUE_TEMPLATE.yaml",
	".gitea/ISSUE_TEMPLATE.yml",
	".gitea/issue_template.md",
	".gitea/issue_template.yaml",
	".gitea/issue_template.yml",
	".github/ISSUE_TEMPLATE.md",
	".github/ISSUE_TEMPLATE.yaml",
	".github/ISSUE_TEMPLATE.yml",
	".github/issue_template.md",
	".github/issue_template.yaml",
	".github/issue_template.yml",
}

// MustAllowUserComment checks to make sure if an issue is locked.
// If locked and user has permissions to write to the repository,
// then the comment is allowed, else it is blocked
func MustAllowUserComment(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if issue.IsLocked && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) && !ctx.Doer.IsAdmin {
		ctx.Flash.Error(ctx.Tr("repo.issues.comment_on_locked"))
		ctx.Redirect(issue.Link())
		return
	}
}

// MustEnableIssues check if repository enable internal issues
func MustEnableIssues(ctx *context.Context) {
	if !ctx.Repo.CanRead(unit.TypeIssues) &&
		!ctx.Repo.CanRead(unit.TypeExternalTracker) {
		ctx.NotFound("MustEnableIssues", nil)
		return
	}

	unit, err := ctx.Repo.Repository.GetUnit(ctx, unit.TypeExternalTracker)
	if err == nil {
		ctx.Redirect(unit.ExternalTrackerConfig().ExternalTrackerURL)
		return
	}
}

// MustAllowPulls check if repository enable pull requests and user have right to do that
func MustAllowPulls(ctx *context.Context) {
	if !ctx.Repo.Repository.CanEnablePulls() || !ctx.Repo.CanRead(unit.TypePullRequests) {
		ctx.NotFound("MustAllowPulls", nil)
		return
	}

	// User can send pull request if owns a forked repository.
	if ctx.IsSigned && repo_model.HasForkedRepo(ctx, ctx.Doer.ID, ctx.Repo.Repository.ID) {
		ctx.Repo.PullRequest.Allowed = true
		ctx.Repo.PullRequest.HeadInfoSubURL = url.PathEscape(ctx.Doer.Name) + ":" + util.PathEscapeSegments(ctx.Repo.BranchName)
	}
}

func retrieveProjectsInternal(ctx *context.Context, repo *repo_model.Repository) (open, closed []*project_model.Project) {
	// Distinguish whether the owner of the repository
	// is an individual or an organization
	repoOwnerType := project_model.TypeIndividual
	if repo.Owner.IsOrganization() {
		repoOwnerType = project_model.TypeOrganization
	}

	projectsUnit := repo.MustGetUnit(ctx, unit.TypeProjects)

	var openProjects []*project_model.Project
	var closedProjects []*project_model.Project
	var err error

	if projectsUnit.ProjectsConfig().IsProjectsAllowed(repo_model.ProjectsModeRepo) {
		openProjects, err = db.Find[project_model.Project](ctx, project_model.SearchOptions{
			ListOptions: db.ListOptionsAll,
			RepoID:      repo.ID,
			IsClosed:    optional.Some(false),
			Type:        project_model.TypeRepository,
		})
		if err != nil {
			ctx.ServerError("GetProjects", err)
			return nil, nil
		}
		closedProjects, err = db.Find[project_model.Project](ctx, project_model.SearchOptions{
			ListOptions: db.ListOptionsAll,
			RepoID:      repo.ID,
			IsClosed:    optional.Some(true),
			Type:        project_model.TypeRepository,
		})
		if err != nil {
			ctx.ServerError("GetProjects", err)
			return nil, nil
		}
	}

	if projectsUnit.ProjectsConfig().IsProjectsAllowed(repo_model.ProjectsModeOwner) {
		openProjects2, err := db.Find[project_model.Project](ctx, project_model.SearchOptions{
			ListOptions: db.ListOptionsAll,
			OwnerID:     repo.OwnerID,
			IsClosed:    optional.Some(false),
			Type:        repoOwnerType,
		})
		if err != nil {
			ctx.ServerError("GetProjects", err)
			return nil, nil
		}
		openProjects = append(openProjects, openProjects2...)
		closedProjects2, err := db.Find[project_model.Project](ctx, project_model.SearchOptions{
			ListOptions: db.ListOptionsAll,
			OwnerID:     repo.OwnerID,
			IsClosed:    optional.Some(true),
			Type:        repoOwnerType,
		})
		if err != nil {
			ctx.ServerError("GetProjects", err)
			return nil, nil
		}
		closedProjects = append(closedProjects, closedProjects2...)
	}

	ctx.Data["OpenProjects"] = openProjects
	ctx.Data["ClosedProjects"] = closedProjects
}

// repoReviewerSelection items to bee shown
type repoReviewerSelection struct {
	IsTeam    bool
	Team      *organization.Team
	User      *user_model.User
	Review    *issues_model.Review
	CanChange bool
	Checked   bool
	ItemID    int64
}

// RetrieveRepoReviewers find all reviewers of a repository
func RetrieveRepoReviewers(ctx *context.Context, repo *repo_model.Repository, issue *issues_model.Issue, canChooseReviewer bool) {
	ctx.Data["CanChooseReviewer"] = canChooseReviewer

	originalAuthorReviews, err := issues_model.GetReviewersFromOriginalAuthorsByIssueID(ctx, issue.ID)
	if err != nil {
		ctx.ServerError("GetReviewersFromOriginalAuthorsByIssueID", err)
		return
	}
	ctx.Data["OriginalReviews"] = originalAuthorReviews

	reviews, err := issues_model.GetReviewsByIssueID(ctx, issue.ID)
	if err != nil {
		ctx.ServerError("GetReviewersByIssueID", err)
		return
	}

	if len(reviews) == 0 && !canChooseReviewer {
		return
	}

	var (
		pullReviews         []*repoReviewerSelection
		reviewersResult     []*repoReviewerSelection
		teamReviewersResult []*repoReviewerSelection
		teamReviewers       []*organization.Team
		reviewers           []*user_model.User
	)

	if canChooseReviewer {
		posterID := issue.PosterID
		if issue.OriginalAuthorID > 0 {
			posterID = 0
		}

		reviewers, err = repo_model.GetReviewers(ctx, repo, ctx.Doer.ID, posterID)
		if err != nil {
			ctx.ServerError("GetReviewers", err)
			return
		}

		teamReviewers, err = repo_service.GetReviewerTeams(ctx, repo)
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
			Checked: review.Type == issues_model.ReviewTypeRequest,
			Review:  review,
			ItemID:  review.ReviewerID,
		}
		if review.ReviewerTeamID > 0 {
			tmp.IsTeam = true
			tmp.ItemID = -review.ReviewerTeamID
		}

		if canChooseReviewer {
			// Users who can choose reviewers can also remove review requests
			tmp.CanChange = true
		} else if ctx.Doer != nil && ctx.Doer.ID == review.ReviewerID && review.Type == issues_model.ReviewTypeRequest {
			// A user can refuse review requests
			tmp.CanChange = true
		}

		pullReviews = append(pullReviews, tmp)

		if canChooseReviewer {
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
				if err = item.Review.LoadReviewer(ctx); err != nil {
					if user_model.IsErrUserNotExist(err) {
						continue
					}
					ctx.ServerError("LoadReviewer", err)
					return
				}
				item.User = item.Review.Reviewer
			} else if item.Review.ReviewerTeamID > 0 {
				if err = item.Review.LoadReviewerTeam(ctx); err != nil {
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

			currentPullReviewers = append(currentPullReviewers, item)
		}
		ctx.Data["PullReviewers"] = currentPullReviewers
	}

	if canChooseReviewer && reviewersResult != nil {
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

		ctx.Data["Reviewers"] = reviewersResult
	}

	if canChooseReviewer && teamReviewersResult != nil {
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

		ctx.Data["TeamReviewers"] = teamReviewersResult
	}
}

// RetrieveRepoMetas find all the meta information of a repository
func RetrieveRepoMetas(ctx *context.Context, repo *repo_model.Repository, isPull bool) []*issues_model.Label {
	if !ctx.Repo.CanWriteIssuesOrPulls(isPull) {
		return nil
	}

	labels, err := issues_model.GetLabelsByRepoID(ctx, repo.ID, "", db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return nil
	}
	ctx.Data["Labels"] = labels
	if repo.Owner.IsOrganization() {
		orgLabels, err := issues_model.GetLabelsByOrgID(ctx, repo.Owner.ID, ctx.FormString("sort"), db.ListOptions{})
		if err != nil {
			return nil
		}

		ctx.Data["OrgLabels"] = orgLabels
		labels = append(labels, orgLabels...)
	}

	RetrieveRepoMilestonesAndAssignees(ctx, repo)
	if ctx.Written() {
		return nil
	}

	retrieveProjects(ctx, repo)
	if ctx.Written() {
		return nil
	}

	PrepareBranchList(ctx)
	if ctx.Written() {
		return nil
	}

	// Contains true if the user can create issue dependencies
	ctx.Data["CanCreateIssueDependencies"] = ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, isPull)

	return labels
}

// Tries to load and set an issue template. The first return value indicates if a template was loaded.
func setTemplateIfExists(ctx *context.Context, ctxDataKey string, possibleFiles []string) (bool, map[string]error) {
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		return false, nil
	}

	templateCandidates := make([]string, 0, 1+len(possibleFiles))
	if t := ctx.FormString("template"); t != "" {
		templateCandidates = append(templateCandidates, t)
	}
	templateCandidates = append(templateCandidates, possibleFiles...) // Append files to the end because they should be fallback

	templateErrs := map[string]error{}
	for _, filename := range templateCandidates {
		if ok, _ := commit.HasFile(filename); !ok {
			continue
		}
		template, err := issue_template.UnmarshalFromCommit(commit, filename)
		if err != nil {
			templateErrs[filename] = err
			continue
		}
		ctx.Data[issueTemplateTitleKey] = template.Title
		ctx.Data[ctxDataKey] = template.Content

		if template.Type() == api.IssueTemplateTypeYaml {
			// Replace field default values by values from query
			for _, field := range template.Fields {
				fieldValue := ctx.FormString("field:" + field.ID)
				if fieldValue != "" {
					field.Attributes["value"] = fieldValue
				}
			}

			ctx.Data["Fields"] = template.Fields
			ctx.Data["TemplateFile"] = template.FileName
		}
		labelIDs := make([]string, 0, len(template.Labels))
		if repoLabels, err := issues_model.GetLabelsByRepoID(ctx, ctx.Repo.Repository.ID, "", db.ListOptions{}); err == nil {
			ctx.Data["Labels"] = repoLabels
			if ctx.Repo.Owner.IsOrganization() {
				if orgLabels, err := issues_model.GetLabelsByOrgID(ctx, ctx.Repo.Owner.ID, ctx.FormString("sort"), db.ListOptions{}); err == nil {
					ctx.Data["OrgLabels"] = orgLabels
					repoLabels = append(repoLabels, orgLabels...)
				}
			}

			for _, metaLabel := range template.Labels {
				for _, repoLabel := range repoLabels {
					if strings.EqualFold(repoLabel.Name, metaLabel) {
						repoLabel.IsChecked = true
						labelIDs = append(labelIDs, strconv.FormatInt(repoLabel.ID, 10))
						break
					}
				}
			}
		}
		selectedAssigneeIDs := make([]int64, 0, len(template.Assignees))
		selectedAssigneeIDStrings := make([]string, 0, len(template.Assignees))
		if userIDs, err := user_model.GetUserIDsByNames(ctx, template.Assignees, false); err == nil {
			for _, userID := range userIDs {
				selectedAssigneeIDs = append(selectedAssigneeIDs, userID)
				selectedAssigneeIDStrings = append(selectedAssigneeIDStrings, strconv.FormatInt(userID, 10))
			}
		}

		if template.Ref != "" && !strings.HasPrefix(template.Ref, "refs/") { // Assume that the ref intended is always a branch - for tags users should use refs/tags/<ref>
			template.Ref = git.BranchPrefix + template.Ref
		}
		ctx.Data["HasSelectedLabel"] = len(labelIDs) > 0
		ctx.Data["label_ids"] = strings.Join(labelIDs, ",")
		ctx.Data["HasSelectedAssignee"] = len(selectedAssigneeIDs) > 0
		ctx.Data["assignee_ids"] = strings.Join(selectedAssigneeIDStrings, ",")
		ctx.Data["SelectedAssigneeIDs"] = selectedAssigneeIDs
		ctx.Data["Reference"] = template.Ref
		ctx.Data["RefEndName"] = git.RefName(template.Ref).ShortName()
		return true, templateErrs
	}
	return false, templateErrs
}

// NewIssue render creating issue page
func NewIssue(ctx *context.Context) {
	issueConfig, _ := issue_service.GetTemplateConfigFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	hasTemplates := issue_service.HasTemplatesOrContactLinks(ctx.Repo.Repository, ctx.Repo.GitRepo)

	ctx.Data["Title"] = ctx.Tr("repo.issues.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["NewIssueChooseTemplate"] = hasTemplates
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	title := ctx.FormString("title")
	ctx.Data["TitleQuery"] = title
	body := ctx.FormString("body")
	ctx.Data["BodyQuery"] = body

	isProjectsEnabled := ctx.Repo.CanRead(unit.TypeProjects)
	ctx.Data["IsProjectsEnabled"] = isProjectsEnabled
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	milestoneID := ctx.FormInt64("milestone")
	if milestoneID > 0 {
		milestone, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, milestoneID)
		if err != nil {
			log.Error("GetMilestoneByID: %d: %v", milestoneID, err)
		} else {
			ctx.Data["milestone_id"] = milestoneID
			ctx.Data["Milestone"] = milestone
		}
	}

	projectID := ctx.FormInt64("project")
	if projectID > 0 && isProjectsEnabled {
		project, err := project_model.GetProjectByID(ctx, projectID)
		if err != nil {
			log.Error("GetProjectByID: %d: %v", projectID, err)
		} else if project.RepoID != ctx.Repo.Repository.ID {
			log.Error("GetProjectByID: %d: %v", projectID, fmt.Errorf("project[%d] not in repo [%d]", project.ID, ctx.Repo.Repository.ID))
		} else {
			ctx.Data["project_id"] = projectID
			ctx.Data["Project"] = project
		}

		if len(ctx.Req.URL.Query().Get("project")) > 0 {
			ctx.Data["redirect_after_creation"] = "project"
		}
	}

	RetrieveRepoMetas(ctx, ctx.Repo.Repository, false)

	tags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetTagNamesByRepoID", err)
		return
	}
	ctx.Data["Tags"] = tags

	ret := issue_service.ParseTemplatesFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	templateLoaded, errs := setTemplateIfExists(ctx, issueTemplateKey, IssueTemplateCandidates)
	for k, v := range errs {
		ret.TemplateErrors[k] = v
	}
	if ctx.Written() {
		return
	}

	if len(ret.TemplateErrors) > 0 {
		ctx.Flash.Warning(renderErrorOfTemplates(ctx, ret.TemplateErrors), true)
	}

	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWrite(unit.TypeIssues)

	if !issueConfig.BlankIssuesEnabled && hasTemplates && !templateLoaded {
		// The "issues/new" and "issues/new/choose" share the same query parameters "project" and "milestone", if blank issues are disabled, just redirect to the "issues/choose" page with these parameters.
		ctx.Redirect(fmt.Sprintf("%s/issues/new/choose?%s", ctx.Repo.Repository.Link(), ctx.Req.URL.RawQuery), http.StatusSeeOther)
		return
	}

	ctx.HTML(http.StatusOK, tplIssueNew)
}

func renderErrorOfTemplates(ctx *context.Context, errs map[string]error) template.HTML {
	var files []string
	for k := range errs {
		files = append(files, k)
	}
	sort.Strings(files) // keep the output stable

	var lines []string
	for _, file := range files {
		lines = append(lines, fmt.Sprintf("%s: %v", file, errs[file]))
	}

	flashError, err := ctx.RenderToHTML(tplAlertDetails, map[string]any{
		"Message": ctx.Tr("repo.issues.choose.ignore_invalid_templates"),
		"Summary": ctx.Tr("repo.issues.choose.invalid_templates", len(errs)),
		"Details": utils.SanitizeFlashErrorString(strings.Join(lines, "\n")),
	})
	if err != nil {
		log.Debug("render flash error: %v", err)
		flashError = ctx.Locale.Tr("repo.issues.choose.ignore_invalid_templates")
	}
	return flashError
}

// NewIssueChooseTemplate render creating issue from template page
func NewIssueChooseTemplate(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.issues.new")
	ctx.Data["PageIsIssueList"] = true

	ret := issue_service.ParseTemplatesFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	ctx.Data["IssueTemplates"] = ret.IssueTemplates

	if len(ret.TemplateErrors) > 0 {
		ctx.Flash.Warning(renderErrorOfTemplates(ctx, ret.TemplateErrors), true)
	}

	if !issue_service.HasTemplatesOrContactLinks(ctx.Repo.Repository, ctx.Repo.GitRepo) {
		// The "issues/new" and "issues/new/choose" share the same query parameters "project" and "milestone", if no template here, just redirect to the "issues/new" page with these parameters.
		ctx.Redirect(fmt.Sprintf("%s/issues/new?%s", ctx.Repo.Repository.Link(), ctx.Req.URL.RawQuery), http.StatusSeeOther)
		return
	}

	issueConfig, err := issue_service.GetTemplateConfigFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	ctx.Data["IssueConfig"] = issueConfig
	ctx.Data["IssueConfigError"] = err // ctx.Flash.Err makes problems here

	ctx.Data["milestone"] = ctx.FormInt64("milestone")
	ctx.Data["project"] = ctx.FormInt64("project")

	ctx.HTML(http.StatusOK, tplIssueChoose)
}

// DeleteIssue deletes an issue
func DeleteIssue(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if err := issue_service.DeleteIssue(ctx, ctx.Doer, ctx.Repo.GitRepo, issue); err != nil {
		ctx.ServerError("DeleteIssueByID", err)
		return
	}

	if issue.IsPull {
		ctx.Redirect(fmt.Sprintf("%s/pulls", ctx.Repo.Repository.Link()), http.StatusSeeOther)
		return
	}

	ctx.Redirect(fmt.Sprintf("%s/issues", ctx.Repo.Repository.Link()), http.StatusSeeOther)
}

// ValidateRepoMetas check and returns repository's meta information
func ValidateRepoMetas(ctx *context.Context, form forms.CreateIssueForm, isPull bool) ([]int64, []int64, int64, int64) {
	var (
		repo = ctx.Repo.Repository
		err  error
	)

	labels := RetrieveRepoMetas(ctx, ctx.Repo.Repository, isPull)
	if ctx.Written() {
		return nil, nil, 0, 0
	}

	var labelIDs []int64
	hasSelected := false
	// Check labels.
	if len(form.LabelIDs) > 0 {
		labelIDs, err = base.StringsToInt64s(strings.Split(form.LabelIDs, ","))
		if err != nil {
			return nil, nil, 0, 0
		}
		labelIDMark := make(container.Set[int64])
		labelIDMark.AddMultiple(labelIDs...)

		for i := range labels {
			if labelIDMark.Contains(labels[i].ID) {
				labels[i].IsChecked = true
				hasSelected = true
			}
		}
	}

	ctx.Data["Labels"] = labels
	ctx.Data["HasSelectedLabel"] = hasSelected
	ctx.Data["label_ids"] = form.LabelIDs

	// Check milestone.
	milestoneID := form.MilestoneID
	if milestoneID > 0 {
		milestone, err := issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, milestoneID)
		if err != nil {
			ctx.ServerError("GetMilestoneByID", err)
			return nil, nil, 0, 0
		}
		if milestone.RepoID != repo.ID {
			ctx.ServerError("GetMilestoneByID", err)
			return nil, nil, 0, 0
		}
		ctx.Data["Milestone"] = milestone
		ctx.Data["milestone_id"] = milestoneID
	}

	if form.ProjectID > 0 {
		p, err := project_model.GetProjectByID(ctx, form.ProjectID)
		if err != nil {
			ctx.ServerError("GetProjectByID", err)
			return nil, nil, 0, 0
		}
		if p.RepoID != ctx.Repo.Repository.ID && p.OwnerID != ctx.Repo.Repository.OwnerID {
			ctx.NotFound("", nil)
			return nil, nil, 0, 0
		}

		ctx.Data["Project"] = p
		ctx.Data["project_id"] = form.ProjectID
	}

	// Check assignees
	var assigneeIDs []int64
	if len(form.AssigneeIDs) > 0 {
		assigneeIDs, err = base.StringsToInt64s(strings.Split(form.AssigneeIDs, ","))
		if err != nil {
			return nil, nil, 0, 0
		}

		// Check if the passed assignees actually exists and is assignable
		for _, aID := range assigneeIDs {
			assignee, err := user_model.GetUserByID(ctx, aID)
			if err != nil {
				ctx.ServerError("GetUserByID", err)
				return nil, nil, 0, 0
			}

			valid, err := access_model.CanBeAssigned(ctx, assignee, repo, isPull)
			if err != nil {
				ctx.ServerError("CanBeAssigned", err)
				return nil, nil, 0, 0
			}

			if !valid {
				ctx.ServerError("canBeAssigned", repo_model.ErrUserDoesNotHaveAccessToRepo{UserID: aID, RepoName: repo.Name})
				return nil, nil, 0, 0
			}
		}
	}

	// Keep the old assignee id thingy for compatibility reasons
	if form.AssigneeID > 0 {
		assigneeIDs = append(assigneeIDs, form.AssigneeID)
	}

	return labelIDs, assigneeIDs, milestoneID, form.ProjectID
}

// NewIssuePost response for creating new issue
func NewIssuePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateIssueForm)
	ctx.Data["Title"] = ctx.Tr("repo.issues.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["NewIssueChooseTemplate"] = issue_service.HasTemplatesOrContactLinks(ctx.Repo.Repository, ctx.Repo.GitRepo)
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	var (
		repo        = ctx.Repo.Repository
		attachments []string
	)

	labelIDs, assigneeIDs, milestoneID, projectID := ValidateRepoMetas(ctx, *form, false)
	if ctx.Written() {
		return
	}

	if projectID > 0 {
		if !ctx.Repo.CanRead(unit.TypeProjects) {
			// User must also be able to see the project.
			ctx.Error(http.StatusBadRequest, "user hasn't permissions to read projects")
			return
		}
	}

	if setting.Attachment.Enabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.JSONError(ctx.GetErrMsg())
		return
	}

	if util.IsEmptyString(form.Title) {
		ctx.JSONError(ctx.Tr("repo.issues.new.title_empty"))
		return
	}

	content := form.Content
	if filename := ctx.Req.Form.Get("template-file"); filename != "" {
		if template, err := issue_template.UnmarshalFromRepo(ctx.Repo.GitRepo, ctx.Repo.Repository.DefaultBranch, filename); err == nil {
			content = issue_template.RenderToMarkdown(template, ctx.Req.Form)
		}
	}

	issue := &issues_model.Issue{
		RepoID:      repo.ID,
		Repo:        repo,
		Title:       form.Title,
		PosterID:    ctx.Doer.ID,
		Poster:      ctx.Doer,
		MilestoneID: milestoneID,
		Content:     content,
		Ref:         form.Ref,
	}

	if err := issue_service.NewIssue(ctx, repo, issue, labelIDs, attachments, assigneeIDs, projectID); err != nil {
		if repo_model.IsErrUserDoesNotHaveAccessToRepo(err) {
			ctx.Error(http.StatusBadRequest, "UserDoesNotHaveAccessToRepo", err.Error())
		} else if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.JSONError(ctx.Tr("repo.issues.new.blocked_user"))
		} else {
			ctx.ServerError("NewIssue", err)
		}
		return
	}

	log.Trace("Issue created: %d/%d", repo.ID, issue.ID)
	if ctx.FormString("redirect_after_creation") == "project" && projectID > 0 {
		ctx.JSONRedirect(ctx.Repo.RepoLink + "/projects/" + strconv.FormatInt(projectID, 10))
	} else {
		ctx.JSONRedirect(issue.Link())
	}
}

// roleDescriptor returns the role descriptor for a comment in/with the given repo, poster and issue
func roleDescriptor(ctx stdCtx.Context, repo *repo_model.Repository, poster *user_model.User, issue *issues_model.Issue, hasOriginalAuthor bool) (issues_model.RoleDescriptor, error) {
	roleDescriptor := issues_model.RoleDescriptor{}

	if hasOriginalAuthor {
		return roleDescriptor, nil
	}

	perm, err := access_model.GetUserRepoPermission(ctx, repo, poster)
	if err != nil {
		return roleDescriptor, err
	}

	// If the poster is the actual poster of the issue, enable Poster role.
	roleDescriptor.IsPoster = issue.IsPoster(poster.ID)

	// Check if the poster is owner of the repo.
	if perm.IsOwner() {
		// If the poster isn't an admin, enable the owner role.
		if !poster.IsAdmin {
			roleDescriptor.RoleInRepo = issues_model.RoleRepoOwner
			return roleDescriptor, nil
		}

		// Otherwise check if poster is the real repo admin.
		ok, err := access_model.IsUserRealRepoAdmin(ctx, repo, poster)
		if err != nil {
			return roleDescriptor, err
		}
		if ok {
			roleDescriptor.RoleInRepo = issues_model.RoleRepoOwner
			return roleDescriptor, nil
		}
	}

	// If repo is organization, check Member role
	if err := repo.LoadOwner(ctx); err != nil {
		return roleDescriptor, err
	}
	if repo.Owner.IsOrganization() {
		if isMember, err := organization.IsOrganizationMember(ctx, repo.Owner.ID, poster.ID); err != nil {
			return roleDescriptor, err
		} else if isMember {
			roleDescriptor.RoleInRepo = issues_model.RoleRepoMember
			return roleDescriptor, nil
		}
	}

	// If the poster is the collaborator of the repo
	if isCollaborator, err := repo_model.IsCollaborator(ctx, repo.ID, poster.ID); err != nil {
		return roleDescriptor, err
	} else if isCollaborator {
		roleDescriptor.RoleInRepo = issues_model.RoleRepoCollaborator
		return roleDescriptor, nil
	}

	hasMergedPR, err := issues_model.HasMergedPullRequestInRepo(ctx, repo.ID, poster.ID)
	if err != nil {
		return roleDescriptor, err
	} else if hasMergedPR {
		roleDescriptor.RoleInRepo = issues_model.RoleRepoContributor
	} else if issue.IsPull {
		// only display first time contributor in the first opening pull request
		roleDescriptor.RoleInRepo = issues_model.RoleRepoFirstTimeContributor
	}

	return roleDescriptor, nil
}

func getBranchData(ctx *context.Context, issue *issues_model.Issue) {
	ctx.Data["BaseBranch"] = nil
	ctx.Data["HeadBranch"] = nil
	ctx.Data["HeadUserName"] = nil
	ctx.Data["BaseName"] = ctx.Repo.Repository.OwnerName
	if issue.IsPull {
		pull := issue.PullRequest
		ctx.Data["BaseBranch"] = pull.BaseBranch
		ctx.Data["HeadBranch"] = pull.HeadBranch
		ctx.Data["HeadUserName"] = pull.MustHeadUserName(ctx)
	}
}

// ViewIssue render issue view page
func ViewIssue(ctx *context.Context) {
	if ctx.PathParam(":type") == "issues" {
		// If issue was requested we check if repo has external tracker and redirect
		extIssueUnit, err := ctx.Repo.Repository.GetUnit(ctx, unit.TypeExternalTracker)
		if err == nil && extIssueUnit != nil {
			if extIssueUnit.ExternalTrackerConfig().ExternalTrackerStyle == markup.IssueNameStyleNumeric || extIssueUnit.ExternalTrackerConfig().ExternalTrackerStyle == "" {
				metas := ctx.Repo.Repository.ComposeMetas(ctx)
				metas["index"] = ctx.PathParam(":index")
				res, err := vars.Expand(extIssueUnit.ExternalTrackerConfig().ExternalTrackerFormat, metas)
				if err != nil {
					log.Error("unable to expand template vars for issue url. issue: %s, err: %v", metas["index"], err)
					ctx.ServerError("Expand", err)
					return
				}
				ctx.Redirect(res)
				return
			}
		} else if err != nil && !repo_model.IsErrUnitTypeNotExist(err) {
			ctx.ServerError("GetUnit", err)
			return
		}
	}

	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("GetIssueByIndex", err)
		} else {
			ctx.ServerError("GetIssueByIndex", err)
		}
		return
	}
	if issue.Repo == nil {
		issue.Repo = ctx.Repo.Repository
	}

	// Make sure type and URL matches.
	if ctx.PathParam(":type") == "issues" && issue.IsPull {
		ctx.Redirect(issue.Link())
		return
	} else if ctx.PathParam(":type") == "pulls" && !issue.IsPull {
		ctx.Redirect(issue.Link())
		return
	}

	if issue.IsPull {
		MustAllowPulls(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["PageIsPullList"] = true
		ctx.Data["PageIsPullConversation"] = true
	} else {
		MustEnableIssues(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["PageIsIssueList"] = true
		ctx.Data["NewIssueChooseTemplate"] = issue_service.HasTemplatesOrContactLinks(ctx.Repo.Repository, ctx.Repo.GitRepo)
	}

	if issue.IsPull && !ctx.Repo.CanRead(unit.TypeIssues) {
		ctx.Data["IssueType"] = "pulls"
	} else if !issue.IsPull && !ctx.Repo.CanRead(unit.TypePullRequests) {
		ctx.Data["IssueType"] = "issues"
	} else {
		ctx.Data["IssueType"] = "all"
	}

	ctx.Data["IsProjectsEnabled"] = ctx.Repo.CanRead(unit.TypeProjects)
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	if err = issue.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	if err = filterXRefComments(ctx, issue); err != nil {
		ctx.ServerError("filterXRefComments", err)
		return
	}

	ctx.Data["Title"] = fmt.Sprintf("#%d - %s", issue.Index, emoji.ReplaceAliases(issue.Title))

	iw := new(issues_model.IssueWatch)
	if ctx.Doer != nil {
		iw.UserID = ctx.Doer.ID
		iw.IssueID = issue.ID
		iw.IsWatching, err = issues_model.CheckIssueWatch(ctx, ctx.Doer, issue)
		if err != nil {
			ctx.ServerError("CheckIssueWatch", err)
			return
		}
	}
	ctx.Data["IssueWatch"] = iw
	issue.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
		Links: markup.Links{
			Base: ctx.Repo.RepoLink,
		},
		Metas:   ctx.Repo.Repository.ComposeMetas(ctx),
		GitRepo: ctx.Repo.GitRepo,
		Repo:    ctx.Repo.Repository,
		Ctx:     ctx,
	}, issue.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	repo := ctx.Repo.Repository

	// Get more information if it's a pull request.
	if issue.IsPull {
		if issue.PullRequest.HasMerged {
			ctx.Data["DisableStatusChange"] = issue.PullRequest.HasMerged
			PrepareMergedViewPullInfo(ctx, issue)
		} else {
			PrepareViewPullInfo(ctx, issue)
			ctx.Data["DisableStatusChange"] = ctx.Data["IsPullRequestBroken"] == true && issue.IsClosed
		}
		if ctx.Written() {
			return
		}
	}

	// Metas.
	// Check labels.
	labelIDMark := make(container.Set[int64])
	for _, label := range issue.Labels {
		labelIDMark.Add(label.ID)
	}
	labels, err := issues_model.GetLabelsByRepoID(ctx, repo.ID, "", db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return
	}
	ctx.Data["Labels"] = labels

	if repo.Owner.IsOrganization() {
		orgLabels, err := issues_model.GetLabelsByOrgID(ctx, repo.Owner.ID, ctx.FormString("sort"), db.ListOptions{})
		if err != nil {
			ctx.ServerError("GetLabelsByOrgID", err)
			return
		}
		ctx.Data["OrgLabels"] = orgLabels

		labels = append(labels, orgLabels...)
	}

	hasSelected := false
	for i := range labels {
		if labelIDMark.Contains(labels[i].ID) {
			labels[i].IsChecked = true
			hasSelected = true
		}
	}
	ctx.Data["HasSelectedLabel"] = hasSelected

	// Check milestone and assignee.
	if ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		RetrieveRepoMilestonesAndAssignees(ctx, repo)
		retrieveProjects(ctx, repo)

		if ctx.Written() {
			return
		}
	}

	if issue.IsPull {
		canChooseReviewer := false
		if ctx.Doer != nil && ctx.IsSigned {
			canChooseReviewer = issue_service.CanDoerChangeReviewRequests(ctx, ctx.Doer, repo, issue)
		}

		RetrieveRepoReviewers(ctx, repo, issue, canChooseReviewer)
		if ctx.Written() {
			return
		}
	}

	if ctx.IsSigned {
		// Update issue-user.
		if err = activities_model.SetIssueReadBy(ctx, issue.ID, ctx.Doer.ID); err != nil {
			ctx.ServerError("ReadBy", err)
			return
		}
	}

	var (
		role                 issues_model.RoleDescriptor
		ok                   bool
		marked               = make(map[int64]issues_model.RoleDescriptor)
		comment              *issues_model.Comment
		participants         = make([]*user_model.User, 1, 10)
		latestCloseCommentID int64
	)
	if ctx.Repo.Repository.IsTimetrackerEnabled(ctx) {
		if ctx.IsSigned {
			// Deal with the stopwatch
			ctx.Data["IsStopwatchRunning"] = issues_model.StopwatchExists(ctx, ctx.Doer.ID, issue.ID)
			if !ctx.Data["IsStopwatchRunning"].(bool) {
				var exists bool
				var swIssue *issues_model.Issue
				if exists, _, swIssue, err = issues_model.HasUserStopwatch(ctx, ctx.Doer.ID); err != nil {
					ctx.ServerError("HasUserStopwatch", err)
					return
				}
				ctx.Data["HasUserStopwatch"] = exists
				if exists {
					// Add warning if the user has already a stopwatch
					// Add link to the issue of the already running stopwatch
					ctx.Data["OtherStopwatchURL"] = swIssue.Link()
				}
			}
			ctx.Data["CanUseTimetracker"] = ctx.Repo.CanUseTimetracker(ctx, issue, ctx.Doer)
		} else {
			ctx.Data["CanUseTimetracker"] = false
		}
		if ctx.Data["WorkingUsers"], err = issues_model.TotalTimesForEachUser(ctx, &issues_model.FindTrackedTimesOptions{IssueID: issue.ID}); err != nil {
			ctx.ServerError("TotalTimesForEachUser", err)
			return
		}
	}

	// Check if the user can use the dependencies
	ctx.Data["CanCreateIssueDependencies"] = ctx.Repo.CanCreateIssueDependencies(ctx, ctx.Doer, issue.IsPull)

	// check if dependencies can be created across repositories
	ctx.Data["AllowCrossRepositoryDependencies"] = setting.Service.AllowCrossRepositoryDependencies

	if issue.ShowRole, err = roleDescriptor(ctx, repo, issue.Poster, issue, issue.HasOriginalAuthor()); err != nil {
		ctx.ServerError("roleDescriptor", err)
		return
	}
	marked[issue.PosterID] = issue.ShowRole

	// Render comments and fetch participants.
	participants[0] = issue.Poster

	if err := issue.Comments.LoadAttachmentsByIssue(ctx); err != nil {
		ctx.ServerError("LoadAttachmentsByIssue", err)
		return
	}
	if err := issue.Comments.LoadPosters(ctx); err != nil {
		ctx.ServerError("LoadPosters", err)
		return
	}

	for _, comment = range issue.Comments {
		comment.Issue = issue

		if comment.Type == issues_model.CommentTypeComment || comment.Type == issues_model.CommentTypeReview {
			comment.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
				Links: markup.Links{
					Base: ctx.Repo.RepoLink,
				},
				Metas:   ctx.Repo.Repository.ComposeMetas(ctx),
				GitRepo: ctx.Repo.GitRepo,
				Repo:    ctx.Repo.Repository,
				Ctx:     ctx,
			}, comment.Content)
			if err != nil {
				ctx.ServerError("RenderString", err)
				return
			}
			// Check tag.
			role, ok = marked[comment.PosterID]
			if ok {
				comment.ShowRole = role
				continue
			}

			comment.ShowRole, err = roleDescriptor(ctx, repo, comment.Poster, issue, comment.HasOriginalAuthor())
			if err != nil {
				ctx.ServerError("roleDescriptor", err)
				return
			}
			marked[comment.PosterID] = comment.ShowRole
			participants = addParticipant(comment.Poster, participants)
		} else if comment.Type == issues_model.CommentTypeLabel {
			if err = comment.LoadLabel(ctx); err != nil {
				ctx.ServerError("LoadLabel", err)
				return
			}
		} else if comment.Type == issues_model.CommentTypeMilestone {
			if err = comment.LoadMilestone(ctx); err != nil {
				ctx.ServerError("LoadMilestone", err)
				return
			}
			ghostMilestone := &issues_model.Milestone{
				ID:   -1,
				Name: ctx.Locale.TrString("repo.issues.deleted_milestone"),
			}
			if comment.OldMilestoneID > 0 && comment.OldMilestone == nil {
				comment.OldMilestone = ghostMilestone
			}
			if comment.MilestoneID > 0 && comment.Milestone == nil {
				comment.Milestone = ghostMilestone
			}
		} else if comment.Type == issues_model.CommentTypeProject {
			if err = comment.LoadProject(ctx); err != nil {
				ctx.ServerError("LoadProject", err)
				return
			}

			ghostProject := &project_model.Project{
				ID:    project_model.GhostProjectID,
				Title: ctx.Locale.TrString("repo.issues.deleted_project"),
			}

			if comment.OldProjectID > 0 && comment.OldProject == nil {
				comment.OldProject = ghostProject
			}

			if comment.ProjectID > 0 && comment.Project == nil {
				comment.Project = ghostProject
			}
		} else if comment.Type == issues_model.CommentTypeProjectColumn {
			if err = comment.LoadProject(ctx); err != nil {
				ctx.ServerError("LoadProject", err)
				return
			}
		} else if comment.Type == issues_model.CommentTypeAssignees || comment.Type == issues_model.CommentTypeReviewRequest {
			if err = comment.LoadAssigneeUserAndTeam(ctx); err != nil {
				ctx.ServerError("LoadAssigneeUserAndTeam", err)
				return
			}
		} else if comment.Type == issues_model.CommentTypeRemoveDependency || comment.Type == issues_model.CommentTypeAddDependency {
			if err = comment.LoadDepIssueDetails(ctx); err != nil {
				if !issues_model.IsErrIssueNotExist(err) {
					ctx.ServerError("LoadDepIssueDetails", err)
					return
				}
			}
		} else if comment.Type.HasContentSupport() {
			comment.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
				Links: markup.Links{
					Base: ctx.Repo.RepoLink,
				},
				Metas:   ctx.Repo.Repository.ComposeMetas(ctx),
				GitRepo: ctx.Repo.GitRepo,
				Repo:    ctx.Repo.Repository,
				Ctx:     ctx,
			}, comment.Content)
			if err != nil {
				ctx.ServerError("RenderString", err)
				return
			}
			if err = comment.LoadReview(ctx); err != nil && !issues_model.IsErrReviewNotExist(err) {
				ctx.ServerError("LoadReview", err)
				return
			}
			participants = addParticipant(comment.Poster, participants)
			if comment.Review == nil {
				continue
			}
			if err = comment.Review.LoadAttributes(ctx); err != nil {
				if !user_model.IsErrUserNotExist(err) {
					ctx.ServerError("Review.LoadAttributes", err)
					return
				}
				comment.Review.Reviewer = user_model.NewGhostUser()
			}
			if err = comment.Review.LoadCodeComments(ctx); err != nil {
				ctx.ServerError("Review.LoadCodeComments", err)
				return
			}
			for _, codeComments := range comment.Review.CodeComments {
				for _, lineComments := range codeComments {
					for _, c := range lineComments {
						// Check tag.
						role, ok = marked[c.PosterID]
						if ok {
							c.ShowRole = role
							continue
						}

						c.ShowRole, err = roleDescriptor(ctx, repo, c.Poster, issue, c.HasOriginalAuthor())
						if err != nil {
							ctx.ServerError("roleDescriptor", err)
							return
						}
						marked[c.PosterID] = c.ShowRole
						participants = addParticipant(c.Poster, participants)
					}
				}
			}
			if err = comment.LoadResolveDoer(ctx); err != nil {
				ctx.ServerError("LoadResolveDoer", err)
				return
			}
		} else if comment.Type == issues_model.CommentTypePullRequestPush {
			participants = addParticipant(comment.Poster, participants)
			if err = comment.LoadPushCommits(ctx); err != nil {
				ctx.ServerError("LoadPushCommits", err)
				return
			}
			if !ctx.Repo.CanRead(unit.TypeActions) {
				for _, commit := range comment.Commits {
					commit.Status.HideActionsURL(ctx)
					git_model.CommitStatusesHideActionsURL(ctx, commit.Statuses)
				}
			}
		} else if comment.Type == issues_model.CommentTypeAddTimeManual ||
			comment.Type == issues_model.CommentTypeStopTracking ||
			comment.Type == issues_model.CommentTypeDeleteTimeManual {
			// drop error since times could be pruned from DB..
			_ = comment.LoadTime(ctx)
			if comment.Content != "" {
				// Content before v1.21 did store the formatted string instead of seconds,
				// so "|" is used as delimiter to mark the new format
				if comment.Content[0] != '|' {
					// handle old time comments that have formatted text stored
					comment.RenderedContent = templates.SanitizeHTML(comment.Content)
					comment.Content = ""
				} else {
					// else it's just a duration in seconds to pass on to the frontend
					comment.Content = comment.Content[1:]
				}
			}
		}

		if comment.Type == issues_model.CommentTypeClose || comment.Type == issues_model.CommentTypeMergePull {
			// record ID of the latest closed/merged comment.
			// if PR is closed, the comments whose type is CommentTypePullRequestPush(29) after latestCloseCommentID won't be rendered.
			latestCloseCommentID = comment.ID
		}
	}

	ctx.Data["LatestCloseCommentID"] = latestCloseCommentID

	// Combine multiple label assignments into a single comment
	combineLabelComments(issue)

	getBranchData(ctx, issue)
	if issue.IsPull {
		pull := issue.PullRequest
		pull.Issue = issue
		canDelete := false
		allowMerge := false
		canWriteToHeadRepo := false

		if ctx.IsSigned {
			if err := pull.LoadHeadRepo(ctx); err != nil {
				log.Error("LoadHeadRepo: %v", err)
			} else if pull.HeadRepo != nil {
				perm, err := access_model.GetUserRepoPermission(ctx, pull.HeadRepo, ctx.Doer)
				if err != nil {
					ctx.ServerError("GetUserRepoPermission", err)
					return
				}
				if perm.CanWrite(unit.TypeCode) {
					// Check if branch is not protected
					if pull.HeadBranch != pull.HeadRepo.DefaultBranch {
						if protected, err := git_model.IsBranchProtected(ctx, pull.HeadRepo.ID, pull.HeadBranch); err != nil {
							log.Error("IsProtectedBranch: %v", err)
						} else if !protected {
							canDelete = true
							ctx.Data["DeleteBranchLink"] = issue.Link() + "/cleanup"
						}
					}
					canWriteToHeadRepo = true
				}
			}

			if err := pull.LoadBaseRepo(ctx); err != nil {
				log.Error("LoadBaseRepo: %v", err)
			}
			perm, err := access_model.GetUserRepoPermission(ctx, pull.BaseRepo, ctx.Doer)
			if err != nil {
				ctx.ServerError("GetUserRepoPermission", err)
				return
			}
			if !canWriteToHeadRepo { // maintainers maybe allowed to push to head repo even if they can't write to it
				canWriteToHeadRepo = pull.AllowMaintainerEdit && perm.CanWrite(unit.TypeCode)
			}
			allowMerge, err = pull_service.IsUserAllowedToMerge(ctx, pull, perm, ctx.Doer)
			if err != nil {
				ctx.ServerError("IsUserAllowedToMerge", err)
				return
			}

			if ctx.Data["CanMarkConversation"], err = issues_model.CanMarkConversation(ctx, issue, ctx.Doer); err != nil {
				ctx.ServerError("CanMarkConversation", err)
				return
			}
		}

		ctx.Data["CanWriteToHeadRepo"] = canWriteToHeadRepo
		ctx.Data["ShowMergeInstructions"] = canWriteToHeadRepo
		ctx.Data["AllowMerge"] = allowMerge

		prUnit, err := repo.GetUnit(ctx, unit.TypePullRequests)
		if err != nil {
			ctx.ServerError("GetUnit", err)
			return
		}
		prConfig := prUnit.PullRequestsConfig()

		ctx.Data["AutodetectManualMerge"] = prConfig.AutodetectManualMerge

		var mergeStyle repo_model.MergeStyle
		// Check correct values and select default
		if ms, ok := ctx.Data["MergeStyle"].(repo_model.MergeStyle); !ok ||
			!prConfig.IsMergeStyleAllowed(ms) {
			defaultMergeStyle := prConfig.GetDefaultMergeStyle()
			if prConfig.IsMergeStyleAllowed(defaultMergeStyle) && !ok {
				mergeStyle = defaultMergeStyle
			} else if prConfig.AllowMerge {
				mergeStyle = repo_model.MergeStyleMerge
			} else if prConfig.AllowRebase {
				mergeStyle = repo_model.MergeStyleRebase
			} else if prConfig.AllowRebaseMerge {
				mergeStyle = repo_model.MergeStyleRebaseMerge
			} else if prConfig.AllowSquash {
				mergeStyle = repo_model.MergeStyleSquash
			} else if prConfig.AllowFastForwardOnly {
				mergeStyle = repo_model.MergeStyleFastForwardOnly
			} else if prConfig.AllowManualMerge {
				mergeStyle = repo_model.MergeStyleManuallyMerged
			}
		}

		ctx.Data["MergeStyle"] = mergeStyle

		defaultMergeMessage, defaultMergeBody, err := pull_service.GetDefaultMergeMessage(ctx, ctx.Repo.GitRepo, pull, mergeStyle)
		if err != nil {
			ctx.ServerError("GetDefaultMergeMessage", err)
			return
		}
		ctx.Data["DefaultMergeMessage"] = defaultMergeMessage
		ctx.Data["DefaultMergeBody"] = defaultMergeBody

		defaultSquashMergeMessage, defaultSquashMergeBody, err := pull_service.GetDefaultMergeMessage(ctx, ctx.Repo.GitRepo, pull, repo_model.MergeStyleSquash)
		if err != nil {
			ctx.ServerError("GetDefaultSquashMergeMessage", err)
			return
		}
		ctx.Data["DefaultSquashMergeMessage"] = defaultSquashMergeMessage
		ctx.Data["DefaultSquashMergeBody"] = defaultSquashMergeBody

		pb, err := git_model.GetFirstMatchProtectedBranchRule(ctx, pull.BaseRepoID, pull.BaseBranch)
		if err != nil {
			ctx.ServerError("LoadProtectedBranch", err)
			return
		}

		if pb != nil {
			pb.Repo = pull.BaseRepo
			ctx.Data["ProtectedBranch"] = pb
			ctx.Data["IsBlockedByApprovals"] = !issues_model.HasEnoughApprovals(ctx, pb, pull)
			ctx.Data["IsBlockedByRejection"] = issues_model.MergeBlockedByRejectedReview(ctx, pb, pull)
			ctx.Data["IsBlockedByOfficialReviewRequests"] = issues_model.MergeBlockedByOfficialReviewRequests(ctx, pb, pull)
			ctx.Data["IsBlockedByOutdatedBranch"] = issues_model.MergeBlockedByOutdatedBranch(pb, pull)
			ctx.Data["GrantedApprovals"] = issues_model.GetGrantedApprovalsCount(ctx, pb, pull)
			ctx.Data["RequireSigned"] = pb.RequireSignedCommits
			ctx.Data["ChangedProtectedFiles"] = pull.ChangedProtectedFiles
			ctx.Data["IsBlockedByChangedProtectedFiles"] = len(pull.ChangedProtectedFiles) != 0
			ctx.Data["ChangedProtectedFilesNum"] = len(pull.ChangedProtectedFiles)
			ctx.Data["RequireApprovalsWhitelist"] = pb.EnableApprovalsWhitelist
		}
		ctx.Data["WillSign"] = false
		if ctx.Doer != nil {
			sign, key, _, err := asymkey_service.SignMerge(ctx, pull, ctx.Doer, pull.BaseRepo.RepoPath(), pull.BaseBranch, pull.GetGitRefName())
			ctx.Data["WillSign"] = sign
			ctx.Data["SigningKey"] = key
			if err != nil {
				if asymkey_service.IsErrWontSign(err) {
					ctx.Data["WontSignReason"] = err.(*asymkey_service.ErrWontSign).Reason
				} else {
					ctx.Data["WontSignReason"] = "error"
					log.Error("Error whilst checking if could sign pr %d in repo %s. Error: %v", pull.ID, pull.BaseRepo.FullName(), err)
				}
			}
		} else {
			ctx.Data["WontSignReason"] = "not_signed_in"
		}

		isPullBranchDeletable := canDelete &&
			pull.HeadRepo != nil &&
			git.IsBranchExist(ctx, pull.HeadRepo.RepoPath(), pull.HeadBranch) &&
			(!pull.HasMerged || ctx.Data["HeadBranchCommitID"] == ctx.Data["PullHeadCommitID"])

		if isPullBranchDeletable && pull.HasMerged {
			exist, err := issues_model.HasUnmergedPullRequestsByHeadInfo(ctx, pull.HeadRepoID, pull.HeadBranch)
			if err != nil {
				ctx.ServerError("HasUnmergedPullRequestsByHeadInfo", err)
				return
			}

			isPullBranchDeletable = !exist
		}
		ctx.Data["IsPullBranchDeletable"] = isPullBranchDeletable

		stillCanManualMerge := func() bool {
			if pull.HasMerged || issue.IsClosed || !ctx.IsSigned {
				return false
			}
			if pull.CanAutoMerge() || pull.IsWorkInProgress(ctx) || pull.IsChecking() {
				return false
			}
			if allowMerge && prConfig.AllowManualMerge {
				return true
			}

			return false
		}

		ctx.Data["StillCanManualMerge"] = stillCanManualMerge()

		// Check if there is a pending pr merge
		ctx.Data["HasPendingPullRequestMerge"], ctx.Data["PendingPullRequestMerge"], err = pull_model.GetScheduledMergeByPullID(ctx, pull.ID)
		if err != nil {
			ctx.ServerError("GetScheduledMergeByPullID", err)
			return
		}
	}

	// Get Dependencies
	blockedBy, err := issue.BlockedByDependencies(ctx, db.ListOptions{})
	if err != nil {
		ctx.ServerError("BlockedByDependencies", err)
		return
	}
	ctx.Data["BlockedByDependencies"], ctx.Data["BlockedByDependenciesNotPermitted"] = checkBlockedByIssues(ctx, blockedBy)
	if ctx.Written() {
		return
	}

	blocking, err := issue.BlockingDependencies(ctx)
	if err != nil {
		ctx.ServerError("BlockingDependencies", err)
		return
	}

	ctx.Data["BlockingDependencies"], ctx.Data["BlockingDependenciesNotPermitted"] = checkBlockedByIssues(ctx, blocking)
	if ctx.Written() {
		return
	}

	var pinAllowed bool
	if !issue.IsPinned() {
		pinAllowed, err = issues_model.IsNewPinAllowed(ctx, issue.RepoID, issue.IsPull)
		if err != nil {
			ctx.ServerError("IsNewPinAllowed", err)
			return
		}
	} else {
		pinAllowed = true
	}

	ctx.Data["Participants"] = participants
	ctx.Data["NumParticipants"] = len(participants)
	ctx.Data["Issue"] = issue
	ctx.Data["Reference"] = issue.Ref
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login?redirect_to=" + url.QueryEscape(ctx.Data["Link"].(string))
	ctx.Data["IsIssuePoster"] = ctx.IsSigned && issue.IsPoster(ctx.Doer.ID)
	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)
	ctx.Data["HasProjectsWritePermission"] = ctx.Repo.CanWrite(unit.TypeProjects)
	ctx.Data["IsRepoAdmin"] = ctx.IsSigned && (ctx.Repo.IsAdmin() || ctx.Doer.IsAdmin)
	ctx.Data["LockReasons"] = setting.Repository.Issue.LockReasons
	ctx.Data["RefEndName"] = git.RefName(issue.Ref).ShortName()
	ctx.Data["NewPinAllowed"] = pinAllowed
	ctx.Data["PinEnabled"] = setting.Repository.Issue.MaxPinned != 0

	var hiddenCommentTypes *big.Int
	if ctx.IsSigned {
		val, err := user_model.GetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyHiddenCommentTypes)
		if err != nil {
			ctx.ServerError("GetUserSetting", err)
			return
		}
		hiddenCommentTypes, _ = new(big.Int).SetString(val, 10) // we can safely ignore the failed conversion here
	}
	ctx.Data["ShouldShowCommentType"] = func(commentType issues_model.CommentType) bool {
		return hiddenCommentTypes == nil || hiddenCommentTypes.Bit(int(commentType)) == 0
	}
	// For sidebar
	PrepareBranchList(ctx)

	if ctx.Written() {
		return
	}

	tags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetTagNamesByRepoID", err)
		return
	}
	ctx.Data["Tags"] = tags

	ctx.Data["CanBlockUser"] = func(blocker, blockee *user_model.User) bool {
		return user_service.CanBlockUser(ctx, ctx.Doer, blocker, blockee)
	}

	ctx.HTML(http.StatusOK, tplIssueView)
}

// checkBlockedByIssues return canRead and notPermitted
func checkBlockedByIssues(ctx *context.Context, blockers []*issues_model.DependencyInfo) (canRead, notPermitted []*issues_model.DependencyInfo) {
	repoPerms := make(map[int64]access_model.Permission)
	repoPerms[ctx.Repo.Repository.ID] = ctx.Repo.Permission
	for _, blocker := range blockers {
		// Get the permissions for this repository
		// If the repo ID exists in the map, return the exist permissions
		// else get the permission and add it to the map
		var perm access_model.Permission
		existPerm, ok := repoPerms[blocker.RepoID]
		if ok {
			perm = existPerm
		} else {
			var err error
			perm, err = access_model.GetUserRepoPermission(ctx, &blocker.Repository, ctx.Doer)
			if err != nil {
				ctx.ServerError("GetUserRepoPermission", err)
				return nil, nil
			}
			repoPerms[blocker.RepoID] = perm
		}
		if perm.CanReadIssuesOrPulls(blocker.Issue.IsPull) {
			canRead = append(canRead, blocker)
		} else {
			notPermitted = append(notPermitted, blocker)
		}
	}
	sortDependencyInfo(canRead)
	sortDependencyInfo(notPermitted)
	return canRead, notPermitted
}

func sortDependencyInfo(blockers []*issues_model.DependencyInfo) {
	sort.Slice(blockers, func(i, j int) bool {
		if blockers[i].RepoID == blockers[j].RepoID {
			return blockers[i].Issue.CreatedUnix < blockers[j].Issue.CreatedUnix
		}
		return blockers[i].RepoID < blockers[j].RepoID
	})
}

// GetActionIssue will return the issue which is used in the context.
func GetActionIssue(ctx *context.Context) *issues_model.Issue {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64(":index"))
	if err != nil {
		ctx.NotFoundOrServerError("GetIssueByIndex", issues_model.IsErrIssueNotExist, err)
		return nil
	}
	issue.Repo = ctx.Repo.Repository
	checkIssueRights(ctx, issue)
	if ctx.Written() {
		return nil
	}
	if err = issue.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return nil
	}
	return issue
}

func checkIssueRights(ctx *context.Context, issue *issues_model.Issue) {
	if issue.IsPull && !ctx.Repo.CanRead(unit.TypePullRequests) ||
		!issue.IsPull && !ctx.Repo.CanRead(unit.TypeIssues) {
		ctx.NotFound("IssueOrPullRequestUnitNotAllowed", nil)
	}
}

func getActionIssues(ctx *context.Context) issues_model.IssueList {
	commaSeparatedIssueIDs := ctx.FormString("issue_ids")
	if len(commaSeparatedIssueIDs) == 0 {
		return nil
	}
	issueIDs := make([]int64, 0, 10)
	for _, stringIssueID := range strings.Split(commaSeparatedIssueIDs, ",") {
		issueID, err := strconv.ParseInt(stringIssueID, 10, 64)
		if err != nil {
			ctx.ServerError("ParseInt", err)
			return nil
		}
		issueIDs = append(issueIDs, issueID)
	}
	issues, err := issues_model.GetIssuesByIDs(ctx, issueIDs)
	if err != nil {
		ctx.ServerError("GetIssuesByIDs", err)
		return nil
	}
	// Check access rights for all issues
	issueUnitEnabled := ctx.Repo.CanRead(unit.TypeIssues)
	prUnitEnabled := ctx.Repo.CanRead(unit.TypePullRequests)
	for _, issue := range issues {
		if issue.RepoID != ctx.Repo.Repository.ID {
			ctx.NotFound("some issue's RepoID is incorrect", errors.New("some issue's RepoID is incorrect"))
			return nil
		}
		if issue.IsPull && !prUnitEnabled || !issue.IsPull && !issueUnitEnabled {
			ctx.NotFound("IssueOrPullRequestUnitNotAllowed", nil)
			return nil
		}
		if err = issue.LoadAttributes(ctx); err != nil {
			ctx.ServerError("LoadAttributes", err)
			return nil
		}
	}
	return issues
}

// GetIssueInfo get an issue of a repository
func GetIssueInfo(ctx *context.Context) {
	issue, err := issues_model.GetIssueWithAttrsByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.Error(http.StatusNotFound)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err.Error())
		}
		return
	}

	if issue.IsPull {
		// Need to check if Pulls are enabled and we can read Pulls
		if !ctx.Repo.Repository.CanEnablePulls() || !ctx.Repo.CanRead(unit.TypePullRequests) {
			ctx.Error(http.StatusNotFound)
			return
		}
	} else {
		// Need to check if Issues are enabled and we can read Issues
		if !ctx.Repo.CanRead(unit.TypeIssues) {
			ctx.Error(http.StatusNotFound)
			return
		}
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"convertedIssue": convert.ToIssue(ctx, ctx.Doer, issue),
		"renderedLabels": templates.NewRenderUtils(ctx).RenderLabels(issue.Labels, ctx.Repo.RepoLink, issue),
	})
}

// UpdateIssueTitle change issue's title
func UpdateIssueTitle(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (!issue.IsPoster(ctx.Doer.ID) && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) {
		ctx.Error(http.StatusForbidden)
		return
	}

	title := ctx.FormTrim("title")
	if len(title) == 0 {
		ctx.Error(http.StatusNoContent)
		return
	}

	if err := issue_service.ChangeTitle(ctx, issue, ctx.Doer, title); err != nil {
		ctx.ServerError("ChangeTitle", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"title": issue.Title,
	})
}

// UpdateIssueTimeEstimate change issue's planned time
var (
	rTimeEstimateStr          = regexp.MustCompile(`^([\d]+w)?\s?([\d]+d)?\s?([\d]+h)?\s?([\d]+m)?$`)
	rTimeEstimateStrHoursOnly = regexp.MustCompile(`^([\d]+)$`)
)

func UpdateIssueTimeEstimate(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (!issue.IsPoster(ctx.Doer.ID) && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) {
		ctx.Error(http.StatusForbidden)
		return
	}

	url := issue.Link()

	timeStr := ctx.FormString("time_estimate")

	// Validate input
	if !rTimeEstimateStr.MatchString(timeStr) && !rTimeEstimateStrHoursOnly.MatchString(timeStr) {
		ctx.Flash.Error(ctx.Tr("repo.issues.time_estimate_invalid"))
		ctx.Redirect(url, http.StatusSeeOther)
		return
	}

	total := util.TimeEstimateFromStr(timeStr)

	// User entered something wrong
	if total == 0 && len(timeStr) != 0 {
		ctx.Flash.Error(ctx.Tr("repo.issues.time_estimate_invalid"))
		ctx.Redirect(url, http.StatusSeeOther)
		return
	}

	// No time changed
	if issue.TimeEstimate == total {
		ctx.Redirect(url, http.StatusSeeOther)
		return
	}

	if err := issue_service.ChangeTimeEstimate(ctx, issue, ctx.Doer, total); err != nil {
		ctx.ServerError("ChangeTimeEstimate", err)
		return
	}

	ctx.Redirect(url, http.StatusSeeOther)
}

// UpdateIssueRef change issue's ref (branch)
func UpdateIssueRef(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (!issue.IsPoster(ctx.Doer.ID) && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) || issue.IsPull {
		ctx.Error(http.StatusForbidden)
		return
	}

	ref := ctx.FormTrim("ref")

	if err := issue_service.ChangeIssueRef(ctx, issue, ctx.Doer, ref); err != nil {
		ctx.ServerError("ChangeRef", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"ref": ref,
	})
}

// UpdateIssueContent change issue's content
func UpdateIssueContent(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != issue.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) {
		ctx.Error(http.StatusForbidden)
		return
	}

	if err := issue_service.ChangeContent(ctx, issue, ctx.Doer, ctx.Req.FormValue("content"), ctx.FormInt("content_version")); err != nil {
		if errors.Is(err, user_model.ErrBlockedUser) {
			ctx.JSONError(ctx.Tr("repo.issues.edit.blocked_user"))
		} else if errors.Is(err, issues_model.ErrIssueAlreadyChanged) {
			if issue.IsPull {
				ctx.JSONError(ctx.Tr("repo.pulls.edit.already_changed"))
			} else {
				ctx.JSONError(ctx.Tr("repo.issues.edit.already_changed"))
			}
		} else {
			ctx.ServerError("ChangeContent", err)
		}
		return
	}

	// when update the request doesn't intend to update attachments (eg: change checkbox state), ignore attachment updates
	if !ctx.FormBool("ignore_attachments") {
		if err := updateAttachments(ctx, issue, ctx.FormStrings("files[]")); err != nil {
			ctx.ServerError("UpdateAttachments", err)
			return
		}
	}

	content, err := markdown.RenderString(markup.NewRenderContext(ctx).
		WithLinks(markup.Links{Base: ctx.FormString("context")}).
		WithMetas(ctx.Repo.Repository.ComposeMetas(ctx)).
		WithGitRepo(ctx.Repo.GitRepo).
		WithRepoFacade(ctx.Repo.Repository),
		issue.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"content":        content,
		"contentVersion": issue.ContentVersion,
		"attachments":    attachmentsHTML(ctx, issue.Attachments, issue.Content),
	})
}

// UpdateIssueDeadline updates an issue deadline
func UpdateIssueDeadline(ctx *context.Context) {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64(":index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound("GetIssueByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err.Error())
		}
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.Error(http.StatusForbidden, "", "Not repo writer")
		return
	}

	deadlineUnix, _ := common.ParseDeadlineDateToEndOfDay(ctx.FormString("deadline"))
	if err := issues_model.UpdateIssueDeadline(ctx, issue, deadlineUnix, ctx.Doer); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateIssueDeadline", err.Error())
		return
	}

	ctx.JSONRedirect("")
}

// UpdateIssueMilestone change issue's milestone
func UpdateIssueMilestone(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	milestoneID := ctx.FormInt64("id")
	for _, issue := range issues {
		oldMilestoneID := issue.MilestoneID
		if oldMilestoneID == milestoneID {
			continue
		}
		issue.MilestoneID = milestoneID
		if err := issue_service.ChangeMilestoneAssign(ctx, issue, ctx.Doer, oldMilestoneID); err != nil {
			ctx.ServerError("ChangeMilestoneAssign", err)
			return
		}
	}

	ctx.JSONOK()
}

// UpdateIssueAssignee change issue's or pull's assignee
func UpdateIssueAssignee(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	assigneeID := ctx.FormInt64("id")
	action := ctx.FormString("action")

	for _, issue := range issues {
		switch action {
		case "clear":
			if err := issue_service.DeleteNotPassedAssignee(ctx, issue, ctx.Doer, []*user_model.User{}); err != nil {
				ctx.ServerError("ClearAssignees", err)
				return
			}
		default:
			assignee, err := user_model.GetUserByID(ctx, assigneeID)
			if err != nil {
				ctx.ServerError("GetUserByID", err)
				return
			}

			valid, err := access_model.CanBeAssigned(ctx, assignee, issue.Repo, issue.IsPull)
			if err != nil {
				ctx.ServerError("canBeAssigned", err)
				return
			}
			if !valid {
				ctx.ServerError("canBeAssigned", repo_model.ErrUserDoesNotHaveAccessToRepo{UserID: assigneeID, RepoName: issue.Repo.Name})
				return
			}

			_, _, err = issue_service.ToggleAssigneeWithNotify(ctx, issue, ctx.Doer, assigneeID)
			if err != nil {
				ctx.ServerError("ToggleAssignee", err)
				return
			}
		}
	}
	ctx.JSONOK()
}

// ChangeIssueReaction create a reaction for issue
func ChangeIssueReaction(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.ReactionForm)
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != issue.PosterID && !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull)) {
		if log.IsTrace() {
			if ctx.IsSigned {
				issueType := "issues"
				if issue.IsPull {
					issueType = "pulls"
				}
				log.Trace("Permission Denied: User %-v not the Poster (ID: %d) and cannot read %s in Repo %-v.\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					issue.PosterID,
					issueType,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Not logged in")
			}
		}

		ctx.Error(http.StatusForbidden)
		return
	}

	if ctx.HasError() {
		ctx.ServerError("ChangeIssueReaction", errors.New(ctx.GetErrMsg()))
		return
	}

	switch ctx.PathParam(":action") {
	case "react":
		reaction, err := issue_service.CreateIssueReaction(ctx, ctx.Doer, issue, form.Content)
		if err != nil {
			if issues_model.IsErrForbiddenIssueReaction(err) || errors.Is(err, user_model.ErrBlockedUser) {
				ctx.ServerError("ChangeIssueReaction", err)
				return
			}
			log.Info("CreateIssueReaction: %s", err)
			break
		}
		// Reload new reactions
		issue.Reactions = nil
		if err = issue.LoadAttributes(ctx); err != nil {
			log.Info("issue.LoadAttributes: %s", err)
			break
		}

		log.Trace("Reaction for issue created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, reaction.ID)
	case "unreact":
		if err := issues_model.DeleteIssueReaction(ctx, ctx.Doer.ID, issue.ID, form.Content); err != nil {
			ctx.ServerError("DeleteIssueReaction", err)
			return
		}

		// Reload new reactions
		issue.Reactions = nil
		if err := issue.LoadAttributes(ctx); err != nil {
			log.Info("issue.LoadAttributes: %s", err)
			break
		}

		log.Trace("Reaction for issue removed: %d/%d", ctx.Repo.Repository.ID, issue.ID)
	default:
		ctx.NotFound(fmt.Sprintf("Unknown action %s", ctx.PathParam(":action")), nil)
		return
	}

	if len(issue.Reactions) == 0 {
		ctx.JSON(http.StatusOK, map[string]any{
			"empty": true,
			"html":  "",
		})
		return
	}

	html, err := ctx.RenderToHTML(tplReactions, map[string]any{
		"ActionURL": fmt.Sprintf("%s/issues/%d/reactions", ctx.Repo.RepoLink, issue.Index),
		"Reactions": issue.Reactions.GroupByType(),
	})
	if err != nil {
		ctx.ServerError("ChangeIssueReaction.HTMLString", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"html": html,
	})
}

// GetIssueAttachments returns attachments for the issue
func GetIssueAttachments(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}
	attachments := make([]*api.Attachment, len(issue.Attachments))
	for i := 0; i < len(issue.Attachments); i++ {
		attachments[i] = convert.ToAttachment(ctx.Repo.Repository, issue.Attachments[i])
	}
	ctx.JSON(http.StatusOK, attachments)
}

func updateAttachments(ctx *context.Context, item any, files []string) error {
	var attachments []*repo_model.Attachment
	switch content := item.(type) {
	case *issues_model.Issue:
		attachments = content.Attachments
	case *issues_model.Comment:
		attachments = content.Attachments
	default:
		return fmt.Errorf("unknown Type: %T", content)
	}
	for i := 0; i < len(attachments); i++ {
		if util.SliceContainsString(files, attachments[i].UUID) {
			continue
		}
		if err := repo_model.DeleteAttachment(ctx, attachments[i], true); err != nil {
			return err
		}
	}
	var err error
	if len(files) > 0 {
		switch content := item.(type) {
		case *issues_model.Issue:
			err = issues_model.UpdateIssueAttachments(ctx, content.ID, files)
		case *issues_model.Comment:
			err = content.UpdateAttachments(ctx, files)
		default:
			return fmt.Errorf("unknown Type: %T", content)
		}
		if err != nil {
			return err
		}
	}
	switch content := item.(type) {
	case *issues_model.Issue:
		content.Attachments, err = repo_model.GetAttachmentsByIssueID(ctx, content.ID)
	case *issues_model.Comment:
		content.Attachments, err = repo_model.GetAttachmentsByCommentID(ctx, content.ID)
	default:
		return fmt.Errorf("unknown Type: %T", content)
	}
	return err
}

func attachmentsHTML(ctx *context.Context, attachments []*repo_model.Attachment, content string) template.HTML {
	attachHTML, err := ctx.RenderToHTML(tplAttachment, map[string]any{
		"ctxData":     ctx.Data,
		"Attachments": attachments,
		"Content":     content,
	})
	if err != nil {
		ctx.ServerError("attachmentsHTML.HTMLString", err)
		return ""
	}
	return attachHTML
}

// get all teams that current user can mention
func handleTeamMentions(ctx *context.Context) {
	if ctx.Doer == nil || !ctx.Repo.Owner.IsOrganization() {
		return
	}

	var isAdmin bool
	var err error
	var teams []*organization.Team
	org := organization.OrgFromUser(ctx.Repo.Owner)
	// Admin has super access.
	if ctx.Doer.IsAdmin {
		isAdmin = true
	} else {
		isAdmin, err = org.IsOwnedBy(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("IsOwnedBy", err)
			return
		}
	}

	if isAdmin {
		teams, err = org.LoadTeams(ctx)
		if err != nil {
			ctx.ServerError("LoadTeams", err)
			return
		}
	} else {
		teams, err = org.GetUserTeams(ctx, ctx.Doer.ID)
		if err != nil {
			ctx.ServerError("GetUserTeams", err)
			return
		}
	}

	ctx.Data["MentionableTeams"] = teams
	ctx.Data["MentionableTeamsOrg"] = ctx.Repo.Owner.Name
	ctx.Data["MentionableTeamsOrgAvatar"] = ctx.Repo.Owner.AvatarLink(ctx)
}
