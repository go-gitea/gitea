// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/git"
	issue_template "code.gitea.io/gitea/modules/issue/template"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/utils"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	"code.gitea.io/gitea/services/forms"
	issue_service "code.gitea.io/gitea/services/issue"
)

// Tries to load and set an issue template. The first return value indicates if a template was loaded.
func setTemplateIfExists(ctx *context.Context, ctxDataKey string, possibleFiles []string, metaData *IssuePageMetaData) (bool, map[string]error) {
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

		metaData.LabelsData.SetSelectedLabelNames(template.Labels)

		selectedAssigneeIDStrings := make([]string, 0, len(template.Assignees))
		if userIDs, err := user_model.GetUserIDsByNames(ctx, template.Assignees, true); err == nil {
			for _, userID := range userIDs {
				selectedAssigneeIDStrings = append(selectedAssigneeIDStrings, strconv.FormatInt(userID, 10))
			}
		}
		metaData.AssigneesData.SelectedAssigneeIDs = strings.Join(selectedAssigneeIDStrings, ",")

		if template.Ref != "" && !strings.HasPrefix(template.Ref, "refs/") { // Assume that the ref intended is always a branch - for tags users should use refs/tags/<ref>
			template.Ref = git.BranchPrefix + template.Ref
		}

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

	pageMetaData := retrieveRepoIssueMetaData(ctx, ctx.Repo.Repository, nil, false)
	if ctx.Written() {
		return
	}

	pageMetaData.MilestonesData.SelectedMilestoneID = ctx.FormInt64("milestone")
	pageMetaData.ProjectsData.SelectedProjectID = ctx.FormInt64("project")
	if pageMetaData.ProjectsData.SelectedProjectID > 0 {
		if len(ctx.Req.URL.Query().Get("project")) > 0 {
			ctx.Data["redirect_after_creation"] = "project"
		}
	}

	tags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetTagNamesByRepoID", err)
		return
	}
	ctx.Data["Tags"] = tags

	ret := issue_service.ParseTemplatesFromDefaultBranch(ctx.Repo.Repository, ctx.Repo.GitRepo)
	templateLoaded, errs := setTemplateIfExists(ctx, issueTemplateKey, IssueTemplateCandidates, pageMetaData)
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

func toSet[ItemType any, KeyType comparable](slice []ItemType, keyFunc func(ItemType) KeyType) container.Set[KeyType] {
	s := make(container.Set[KeyType])
	for _, item := range slice {
		s.Add(keyFunc(item))
	}
	return s
}

// ValidateRepoMetasForNewIssue check and returns repository's meta information
func ValidateRepoMetasForNewIssue(ctx *context.Context, form forms.CreateIssueForm, isPull bool) (ret struct {
	LabelIDs, AssigneeIDs  []int64
	MilestoneID, ProjectID int64

	Reviewers     []*user_model.User
	TeamReviewers []*organization.Team
},
) {
	pageMetaData := retrieveRepoIssueMetaData(ctx, ctx.Repo.Repository, nil, isPull)
	if ctx.Written() {
		return ret
	}

	inputLabelIDs, _ := base.StringsToInt64s(strings.Split(form.LabelIDs, ","))
	candidateLabels := toSet(pageMetaData.LabelsData.AllLabels, func(label *issues_model.Label) int64 { return label.ID })
	if len(inputLabelIDs) > 0 && !candidateLabels.Contains(inputLabelIDs...) {
		ctx.NotFound("", nil)
		return ret
	}
	pageMetaData.LabelsData.SetSelectedLabelIDs(inputLabelIDs)

	allMilestones := append(slices.Clone(pageMetaData.MilestonesData.OpenMilestones), pageMetaData.MilestonesData.ClosedMilestones...)
	candidateMilestones := toSet(allMilestones, func(milestone *issues_model.Milestone) int64 { return milestone.ID })
	if form.MilestoneID > 0 && !candidateMilestones.Contains(form.MilestoneID) {
		ctx.NotFound("", nil)
		return ret
	}
	pageMetaData.MilestonesData.SelectedMilestoneID = form.MilestoneID

	allProjects := append(slices.Clone(pageMetaData.ProjectsData.OpenProjects), pageMetaData.ProjectsData.ClosedProjects...)
	candidateProjects := toSet(allProjects, func(project *project_model.Project) int64 { return project.ID })
	if form.ProjectID > 0 && !candidateProjects.Contains(form.ProjectID) {
		ctx.NotFound("", nil)
		return ret
	}
	pageMetaData.ProjectsData.SelectedProjectID = form.ProjectID

	// prepare assignees
	candidateAssignees := toSet(pageMetaData.AssigneesData.CandidateAssignees, func(user *user_model.User) int64 { return user.ID })
	inputAssigneeIDs, _ := base.StringsToInt64s(strings.Split(form.AssigneeIDs, ","))
	var assigneeIDStrings []string
	for _, inputAssigneeID := range inputAssigneeIDs {
		if candidateAssignees.Contains(inputAssigneeID) {
			assigneeIDStrings = append(assigneeIDStrings, strconv.FormatInt(inputAssigneeID, 10))
		}
	}
	pageMetaData.AssigneesData.SelectedAssigneeIDs = strings.Join(assigneeIDStrings, ",")

	// Check if the passed reviewers (user/team) actually exist
	var reviewers []*user_model.User
	var teamReviewers []*organization.Team
	reviewerIDs, _ := base.StringsToInt64s(strings.Split(form.ReviewerIDs, ","))
	if isPull && len(reviewerIDs) > 0 {
		userReviewersMap := map[int64]*user_model.User{}
		teamReviewersMap := map[int64]*organization.Team{}
		for _, r := range pageMetaData.ReviewersData.Reviewers {
			userReviewersMap[r.User.ID] = r.User
		}
		for _, r := range pageMetaData.ReviewersData.TeamReviewers {
			teamReviewersMap[r.Team.ID] = r.Team
		}
		for _, rID := range reviewerIDs {
			if rID < 0 { // negative reviewIDs represent team requests
				team, ok := teamReviewersMap[-rID]
				if !ok {
					ctx.NotFound("", nil)
					return ret
				}
				teamReviewers = append(teamReviewers, team)
			} else {
				user, ok := userReviewersMap[rID]
				if !ok {
					ctx.NotFound("", nil)
					return ret
				}
				reviewers = append(reviewers, user)
			}
		}
	}

	ret.LabelIDs, ret.AssigneeIDs, ret.MilestoneID, ret.ProjectID = inputLabelIDs, inputAssigneeIDs, form.MilestoneID, form.ProjectID
	ret.Reviewers, ret.TeamReviewers = reviewers, teamReviewers
	return ret
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

	validateRet := ValidateRepoMetasForNewIssue(ctx, *form, false)
	if ctx.Written() {
		return
	}

	labelIDs, assigneeIDs, milestoneID, projectID := validateRet.LabelIDs, validateRet.AssigneeIDs, validateRet.MilestoneID, validateRet.ProjectID

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
		project, err := project_model.GetProjectByID(ctx, projectID)
		if err == nil {
			if project.Type == project_model.TypeOrganization {
				ctx.JSONRedirect(project_model.ProjectLinkForOrg(ctx.Repo.Owner, project.ID))
			} else {
				ctx.JSONRedirect(project_model.ProjectLinkForRepo(repo, project.ID))
			}
			return
		}
	}
	ctx.JSONRedirect(issue.Link())
}
