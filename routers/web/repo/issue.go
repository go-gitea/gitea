// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
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
	tplAttachment templates.TplName = "repo/issue/view_content/attachments"

	tplIssues      templates.TplName = "repo/issue/list"
	tplIssueNew    templates.TplName = "repo/issue/new"
	tplIssueChoose templates.TplName = "repo/issue/choose"
	tplIssueView   templates.TplName = "repo/issue/view"

	tplReactions templates.TplName = "repo/issue/view_content/reactions"

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
	return openProjects, closedProjects
}

// GetActionIssue will return the issue which is used in the context.
func GetActionIssue(ctx *context.Context) *issues_model.Issue {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
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
	issue, err := issues_model.GetIssueWithAttrsByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
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

	rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository)
	content, err := markdown.RenderString(rctx, issue.Content)
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
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
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

	switch ctx.PathParam("action") {
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
		ctx.NotFound(fmt.Sprintf("Unknown action %s", ctx.PathParam("action")), nil)
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
			err = issues_model.UpdateCommentAttachments(ctx, content, files)
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

// handleMentionableAssigneesAndTeams gets all teams that current user can mention, and fills the assignee users to the context data
func handleMentionableAssigneesAndTeams(ctx *context.Context, assignees []*user_model.User) {
	// TODO: need to figure out how many places this is really used, and rename it to "MentionableAssignees"
	// at the moment it is used on the issue list page, for the markdown editor mention
	ctx.Data["Assignees"] = assignees

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
