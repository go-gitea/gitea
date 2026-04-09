// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	stdctx "context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	perm_model "code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	project_model "code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/htmlutil"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
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

	tplPullMergeBox templates.TplName = "repo/issue/view_content/pull_merge_box"
	tplReactions    templates.TplName = "repo/issue/view_content/reactions"

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
		ctx.NotFound(nil)
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
		ctx.NotFound(nil)
		return
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
		ctx.NotFound(nil)
	}
}

func getActionIssues(ctx *context.Context) issues_model.IssueList {
	commaSeparatedIssueIDs := ctx.FormString("issue_ids")
	if len(commaSeparatedIssueIDs) == 0 {
		return nil
	}
	issueIDs := make([]int64, 0, 10)
	for stringIssueID := range strings.SplitSeq(commaSeparatedIssueIDs, ",") {
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
			ctx.NotFound(errors.New("some issue's RepoID is incorrect"))
			return nil
		}
		if issue.IsPull && !prUnitEnabled || !issue.IsPull && !issueUnitEnabled {
			ctx.NotFound(nil)
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
			ctx.HTTPError(http.StatusNotFound)
		} else {
			ctx.HTTPError(http.StatusInternalServerError, "GetIssueByIndex", err.Error())
		}
		return
	}

	if issue.IsPull {
		// Need to check if Pulls are enabled and we can read Pulls
		if !ctx.Repo.Repository.CanEnablePulls() || !ctx.Repo.CanRead(unit.TypePullRequests) {
			ctx.HTTPError(http.StatusNotFound)
			return
		}
	} else {
		// Need to check if Issues are enabled and we can read Issues
		if !ctx.Repo.CanRead(unit.TypeIssues) {
			ctx.HTTPError(http.StatusNotFound)
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
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	title := ctx.FormTrim("title")
	if len(title) == 0 {
		ctx.HTTPError(http.StatusNoContent)
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
		ctx.HTTPError(http.StatusForbidden)
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
		ctx.HTTPError(http.StatusForbidden)
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

	rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{
		FootnoteContextID: "0",
	})
	content, err := markdown.RenderString(rctx, issue.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"content":        commentContentHTML(ctx, content),
		"contentVersion": issue.ContentVersion,
		"attachments":    attachmentsHTML(ctx, issue.Attachments, issue.Content),
	})
}

// UpdateIssueDeadline updates an issue deadline
func UpdateIssueDeadline(ctx *context.Context) {
	issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("index"))
	if err != nil {
		if issues_model.IsErrIssueNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.HTTPError(http.StatusInternalServerError, "GetIssueByIndex", err.Error())
		}
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.HTTPError(http.StatusForbidden, "", "Not repo writer")
		return
	}

	deadlineUnix, _ := common.ParseDeadlineDateToEndOfDay(ctx.FormString("deadline"))
	if err := issues_model.UpdateIssueDeadline(ctx, issue, deadlineUnix, ctx.Doer); err != nil {
		ctx.HTTPError(http.StatusInternalServerError, "UpdateIssueDeadline", err.Error())
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
		if milestoneID > 0 {
			var err error
			issue.Milestone, err = issues_model.GetMilestoneByRepoID(ctx, ctx.Repo.Repository.ID, milestoneID)
			if err != nil {
				ctx.ServerError("GetMilestoneByRepoID", err)
				return
			}
		} else {
			issue.Milestone = nil
		}
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

		ctx.HTTPError(http.StatusForbidden)
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
		ctx.NotFound(nil)
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

func commentContentHTML(ctx *context.Context, content template.HTML) template.HTML {
	if strings.TrimSpace(string(content)) == "" {
		return htmlutil.HTMLFormat(`<span class="no-content">%s</span>`, ctx.Tr("repo.issues.no_content"))
	}
	return content
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

// copyLabelsToIssue copies labels from origLabels to issue in targetRepo.
// If a label belongs to an org and targetRepo is in the same org, the label is reused directly.
// Otherwise, finds or creates a matching label in targetRepo.
func copyLabelsToIssue(ctx stdctx.Context, issue *issues_model.Issue, origLabels []*issues_model.Label, targetRepo *repo_model.Repository) {
	for _, origLabel := range origLabels {
		var labelID int64
		if origLabel.BelongsToOrg() && origLabel.OrgID == targetRepo.OwnerID {
			// Same org label: reuse directly
			labelID = origLabel.ID
		} else {
			// Repo label or different org: find by name in target repo
			found, err := issues_model.GetLabelInRepoByName(ctx, targetRepo.ID, origLabel.Name)
			if err == nil {
				labelID = found.ID
			} else {
				// Not found: create it
				newLabel := &issues_model.Label{
					RepoID:      targetRepo.ID,
					Name:        origLabel.Name,
					Exclusive:   origLabel.Exclusive,
					Description: origLabel.Description,
					Color:       origLabel.Color,
				}
				if err := issues_model.NewLabel(ctx, newLabel); err != nil {
					continue
				}
				labelID = newLabel.ID
			}
		}
		_, _ = db.GetEngine(ctx).Insert(&issues_model.IssueLabel{IssueID: issue.ID, LabelID: labelID})
	}
}

// MoveIssueToRepo moves an issue to another repository.
func MoveIssueToRepo(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if issue.IsPull {
		ctx.Flash.Error("Cannot move a pull request to another repository.")
		ctx.JSONRedirect(issue.Link())
		return
	}

	newRepoFullName := ctx.FormString("new_repo")
	if newRepoFullName == "" {
		ctx.Flash.Error("Target repository is required.")
		ctx.JSONRedirect(issue.Link())
		return
	}

	parts := strings.SplitN(newRepoFullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		ctx.Flash.Error("Target repository must be in 'owner/repo' format.")
		ctx.JSONRedirect(issue.Link())
		return
	}

	newRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, parts[0], parts[1])
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.Flash.Error("Target repository not found.")
			ctx.JSONRedirect(issue.Link())
			return
		}
		ctx.ServerError("GetRepositoryByOwnerAndName", err)
		return
	}

	isSameRepo := newRepo.ID == ctx.Repo.Repository.ID

	// Same repo: nothing to move, just redirect
	if isSameRepo {
		ctx.Flash.Info("Issue is already in this repository.")
		ctx.JSONRedirect(issue.Link())
		return
	}

	// Check permission: user must be able to write issues in the target repo
	canWrite, err := access_model.HasAccessUnit(ctx, ctx.Doer, newRepo, unit.TypeIssues, perm_model.AccessModeWrite)
	if err != nil {
		ctx.ServerError("HasAccessUnit", err)
		return
	}
	if !canWrite {
		ctx.Flash.Error("You don't have permission to create issues in the target repository.")
		ctx.JSONRedirect(issue.Link())
		return
	}

	// Collect original labels, milestone, and project info before moving (for copy options)
	var origLabels []*issues_model.Label
	var origMilestoneName string

	origLabels, err = issues_model.GetLabelsByIssueID(ctx, issue.ID)
	if err != nil {
		ctx.ServerError("GetLabelsByIssueID", err)
		return
	}

	var origMilestoneIsOrg bool
	var origMilestoneOrgID int64
	if issue.MilestoneID > 0 {
		origMilestone, err := issues_model.GetMilestoneByRepoID(ctx, issue.RepoID, issue.MilestoneID)
		if err == nil {
			origMilestoneName = origMilestone.Name
			origMilestoneIsOrg = origMilestone.BelongsToOrg()
			origMilestoneOrgID = origMilestone.OrgID
		} else if issues_model.IsErrMilestoneNotExist(err) {
			// Could be an org milestone not found by repo query, try direct lookup
			var m issues_model.Milestone
			if has, err2 := db.GetEngine(ctx).ID(issue.MilestoneID).Get(&m); err2 == nil && has {
				origMilestoneName = m.Name
				origMilestoneIsOrg = m.BelongsToOrg()
				origMilestoneOrgID = m.OrgID
			}
		}
	}

	var origProject *project_model.Project
	if err := issue.LoadProject(ctx); err == nil && issue.Project != nil {
		origProject = issue.Project
	}

	oldRepoID := issue.RepoID
	oldIndex := issue.Index
	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		ctx.ServerError("TxContext", err)
		return
	}
	defer committer.Close()

	// Get next issue index for the new repo
	newIndex, err := db.GetNextResourceIndex(dbCtx, "issue_index", newRepo.ID)
	if err != nil {
		ctx.ServerError("GetNextResourceIndex", err)
		return
	}

	// Update issue's RepoID and Index
	issue.RepoID = newRepo.ID
	issue.Index = newIndex
	if _, err := db.GetEngine(dbCtx).ID(issue.ID).Cols("repo_id", "`index`").Update(issue); err != nil {
		ctx.ServerError("UpdateIssue", err)
		return
	}

	// Record redirect so the old URL still works
	if err := issues_model.CreateIssueRedirect(dbCtx, oldRepoID, oldIndex, issue.ID); err != nil {
		ctx.ServerError("CreateIssueRedirect", err)
		return
	}

	// Clear labels (they belong to the old repo)
	if _, err := db.GetEngine(dbCtx).Where("issue_id = ?", issue.ID).Delete(&issues_model.IssueLabel{}); err != nil {
		ctx.ServerError("ClearIssueLabels", err)
		return
	}

	// Clear milestone (keep org milestones if same org)
	if issue.MilestoneID > 0 {
		keepMilestone := origMilestoneIsOrg && origMilestoneOrgID == newRepo.OwnerID
		if !keepMilestone {
			if _, err := db.GetEngine(dbCtx).ID(issue.ID).Cols("milestone_id").Update(&issues_model.Issue{MilestoneID: 0}); err != nil {
				ctx.ServerError("ClearMilestone", err)
				return
			}
		}
	}

	// Clear project
	if _, err := db.GetEngine(dbCtx).Where("issue_id = ?", issue.ID).Delete(&project_model.ProjectIssue{}); err != nil {
		ctx.ServerError("ClearProject", err)
		return
	}

	// Decrement old repo issue count
	if err := issues_model.DecrRepoIssueNumbers(dbCtx, oldRepoID, issue.IsPull, true, issue.IsClosed); err != nil {
		ctx.ServerError("DecrRepoIssueNumbers", err)
		return
	}

	// Increment new repo issue count
	if err := issues_model.IncrRepoIssueNumbers(dbCtx, newRepo.ID, issue.IsPull, true); err != nil {
		ctx.ServerError("IncrRepoIssueNumbers", err)
		return
	}
	if issue.IsClosed {
		if err := issues_model.IncrRepoIssueNumbers(dbCtx, newRepo.ID, issue.IsPull, false); err != nil {
			ctx.ServerError("IncrRepoIssueNumbers", err)
			return
		}
	}

	if err := committer.Commit(); err != nil {
		ctx.ServerError("Commit", err)
		return
	}

	// Record move activity on the moved issue
	oldIssueRef := fmt.Sprintf("%s#%d", ctx.Repo.Repository.FullName(), oldIndex)
	_, _ = issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Type:    issues_model.CommentTypeMoveIssueFrom,
		Doer:    ctx.Doer,
		Repo:    newRepo,
		Issue:   issue,
		Content: oldIssueRef,
	})

	// Update search index for the moved issue
	issue_indexer.UpdateIssueIndexer(ctx, issue.ID)

	// Remove subscriptions for users who can't access the new (private) repo
	if newRepo.IsPrivate {
		if err := issues_model.RemoveIssueWatchersWithoutRepoAccess(ctx, issue.ID, newRepo.ID, newRepo.OwnerID); err != nil {
			log.Error("RemoveIssueWatchersWithoutRepoAccess issue[%d]: %v", issue.ID, err)
		}
	}

	// Best-effort copy after commit (using ctx, not dbCtx)
	// Copy labels to new repo
	if ctx.FormBool("copy_labels") && len(origLabels) > 0 {
		copyLabelsToIssue(ctx, issue, origLabels, newRepo)
	}

	// Copy milestone to new repo (skip if org milestone was already kept)
	keepMilestone := origMilestoneIsOrg && origMilestoneOrgID == newRepo.OwnerID
	if ctx.FormBool("copy_milestone") && origMilestoneName != "" && !keepMilestone {
		newMilestone, err := issues_model.GetMilestoneByRepoIDANDName(ctx, newRepo.ID, origMilestoneName)
		if err != nil {
			// Milestone not found in target repo, create it
			newMilestone = &issues_model.Milestone{
				RepoID: newRepo.ID,
				Name:   origMilestoneName,
			}
			if err := issues_model.NewMilestone(ctx, newMilestone); err != nil {
				newMilestone = nil // best-effort: skip on error
			}
		}
		if newMilestone != nil {
			if _, err := db.GetEngine(ctx).ID(issue.ID).Cols("milestone_id").Update(&issues_model.Issue{MilestoneID: newMilestone.ID}); err != nil {
				// best-effort: ignore
				_ = err
			}
		}
	}

	// Copy project to new repo
	if ctx.FormBool("copy_project") && origProject != nil {
		var targetProjectID int64
		if origProject.Type == project_model.TypeOrganization && origProject.OwnerID == newRepo.OwnerID {
			// Same org project: reuse as-is
			targetProjectID = origProject.ID
		} else {
			// Repo project or different org: find by name or create
			projects, err := db.Find[project_model.Project](ctx, project_model.SearchOptions{
				RepoID:   newRepo.ID,
				IsClosed: optional.Some(false),
				Type:     project_model.TypeRepository,
			})
			if err == nil {
				for _, p := range projects {
					if p.Title == origProject.Title {
						targetProjectID = p.ID
						break
					}
				}
			}
			if targetProjectID == 0 {
				newProject := &project_model.Project{
					Title:     origProject.Title,
					RepoID:    newRepo.ID,
					CreatorID: ctx.Doer.ID,
					Type:      project_model.TypeRepository,
				}
				if err := project_model.NewProject(ctx, newProject); err == nil {
					targetProjectID = newProject.ID
				}
			}
		}
		if targetProjectID > 0 {
			targetProject, err := project_model.GetProjectByID(ctx, targetProjectID)
			if err == nil {
				defaultColumn, err := targetProject.MustDefaultColumn(ctx)
				if err == nil {
					if _, err := db.GetEngine(ctx).Insert(&project_model.ProjectIssue{
						IssueID:         issue.ID,
						ProjectID:       targetProjectID,
						ProjectColumnID: defaultColumn.ID,
					}); err != nil {
						// best-effort: ignore
						_ = err
					}
				}
			}
		}
	}

	log.Info("Issue [%d] moved from repo [%d] to repo [%d] by user [%d]", issue.ID, ctx.Repo.Repository.ID, newRepo.ID, ctx.Doer.ID)

	ctx.Flash.Success("Issue moved successfully.")
	ctx.JSONRedirect(fmt.Sprintf("%s/issues/%d", newRepo.Link(), issue.Index))
}

// CopyIssueToRepo copies an issue to another repository (without deleting the original)
func CopyIssueToRepo(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if issue.IsPull {
		ctx.Flash.Error("Cannot copy a pull request to another repository.")
		ctx.JSONRedirect(issue.Link())
		return
	}

	newRepoFullName := ctx.FormString("new_repo")
	if newRepoFullName == "" {
		ctx.Flash.Error("Target repository is required.")
		ctx.JSONRedirect(issue.Link())
		return
	}

	parts := strings.SplitN(newRepoFullName, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		ctx.Flash.Error("Target repository must be in 'owner/repo' format.")
		ctx.JSONRedirect(issue.Link())
		return
	}

	newRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, parts[0], parts[1])
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			ctx.Flash.Error("Target repository not found.")
			ctx.JSONRedirect(issue.Link())
			return
		}
		ctx.ServerError("GetRepositoryByOwnerAndName", err)
		return
	}

	// Check permission: user must be able to write issues in the target repo
	canWrite, err := access_model.HasAccessUnit(ctx, ctx.Doer, newRepo, unit.TypeIssues, perm_model.AccessModeWrite)
	if err != nil {
		ctx.ServerError("HasAccessUnit", err)
		return
	}
	if !canWrite {
		ctx.Flash.Error("You don't have permission to create issues in the target repository.")
		ctx.JSONRedirect(issue.Link())
		return
	}

	dbCtx, committer, err := db.TxContext(ctx)
	if err != nil {
		ctx.ServerError("TxContext", err)
		return
	}
	defer committer.Close()

	// Get next issue index for the new repo
	newIndex, err := db.GetNextResourceIndex(dbCtx, "issue_index", newRepo.ID)
	if err != nil {
		ctx.ServerError("GetNextResourceIndex", err)
		return
	}

	// Create a new issue in the target repo
	newIssue := &issues_model.Issue{
		RepoID:   newRepo.ID,
		Index:    newIndex,
		PosterID: issue.PosterID,
		Title:    issue.Title,
		Content:  issue.Content,
	}
	if _, err := db.GetEngine(dbCtx).Insert(newIssue); err != nil {
		ctx.ServerError("InsertIssue", err)
		return
	}

	// Increment new repo issue count
	if err := issues_model.IncrRepoIssueNumbers(dbCtx, newRepo.ID, false, true); err != nil {
		ctx.ServerError("IncrRepoIssueNumbers", err)
		return
	}

	if err := committer.Commit(); err != nil {
		ctx.ServerError("Commit", err)
		return
	}

	// Set dependency between original and copied issue (best-effort)
	depType := ctx.FormString("dep_type")
	if depType == "original_blocks_copy" {
		// Original blocks copy: copy (newIssue) depends on original (issue)
		_ = issues_model.CreateIssueDependency(ctx, ctx.Doer, newIssue, issue)
	} else if depType == "copy_blocks_original" {
		// Copy blocks original: original (issue) depends on copy (newIssue)
		_ = issues_model.CreateIssueDependency(ctx, ctx.Doer, issue, newIssue)
	}

	// Best-effort: copy labels
	if ctx.FormBool("copy_labels") {
		origLabels, err := issues_model.GetLabelsByIssueID(ctx, issue.ID)
		if err == nil && len(origLabels) > 0 {
			copyLabelsToIssue(ctx, newIssue, origLabels, newRepo)
		}
	}

	// Best-effort: copy milestone
	if ctx.FormBool("copy_milestone") && issue.MilestoneID > 0 {
		origMilestone, err := issues_model.GetMilestoneByRepoID(ctx, issue.RepoID, issue.MilestoneID)
		if err == nil {
			newMilestone, err := issues_model.GetMilestoneByRepoIDANDName(ctx, newRepo.ID, origMilestone.Name)
			if err != nil {
				newMilestone = &issues_model.Milestone{
					RepoID: newRepo.ID,
					Name:   origMilestone.Name,
				}
				if err := issues_model.NewMilestone(ctx, newMilestone); err != nil {
					newMilestone = nil
				}
			}
			if newMilestone != nil {
				_, _ = db.GetEngine(ctx).ID(newIssue.ID).Cols("milestone_id").Update(&issues_model.Issue{MilestoneID: newMilestone.ID})
			}
		}
	}

	// Best-effort: copy project
	if ctx.FormBool("copy_project") {
		if err := issue.LoadProject(ctx); err == nil && issue.Project != nil {
			origProj := issue.Project
			var targetProjectID int64
			if origProj.Type == project_model.TypeOrganization && origProj.OwnerID == newRepo.OwnerID {
				// Same org project: reuse as-is
				targetProjectID = origProj.ID
			} else {
				// Repo project or different org: find by name or create
				projects, err := db.Find[project_model.Project](ctx, project_model.SearchOptions{
					RepoID:   newRepo.ID,
					IsClosed: optional.Some(false),
					Type:     project_model.TypeRepository,
				})
				if err == nil {
					for _, p := range projects {
						if p.Title == origProj.Title {
							targetProjectID = p.ID
							break
						}
					}
				}
				if targetProjectID == 0 {
					newProject := &project_model.Project{
						Title:     origProj.Title,
						RepoID:    newRepo.ID,
						CreatorID: ctx.Doer.ID,
						Type:      project_model.TypeRepository,
					}
					if err := project_model.NewProject(ctx, newProject); err == nil {
						targetProjectID = newProject.ID
					}
				}
			}
			if targetProjectID > 0 {
				targetProject, err := project_model.GetProjectByID(ctx, targetProjectID)
				if err == nil {
					defaultColumn, err := targetProject.MustDefaultColumn(ctx)
					if err == nil {
						_, _ = db.GetEngine(ctx).Insert(&project_model.ProjectIssue{
							IssueID:         newIssue.ID,
							ProjectID:       targetProjectID,
							ProjectColumnID: defaultColumn.ID,
						})
					}
				}
			}
		}
	}

	// Record copy activity on both the original and copied issues
	newIssueRef := fmt.Sprintf("%s#%d", newRepo.FullName(), newIssue.Index)
	_, _ = issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Type:    issues_model.CommentTypeCopyIssueTo,
		Doer:    ctx.Doer,
		Repo:    ctx.Repo.Repository,
		Issue:   issue,
		Content: newIssueRef,
	})
	origIssueRef := fmt.Sprintf("%s#%d", ctx.Repo.Repository.FullName(), issue.Index)
	_, _ = issues_model.CreateComment(ctx, &issues_model.CreateCommentOptions{
		Type:    issues_model.CommentTypeCopyIssueFrom,
		Doer:    ctx.Doer,
		Repo:    newRepo,
		Issue:   newIssue,
		Content: origIssueRef,
	})

	log.Info("Issue [%d] copied to repo [%d] as issue [%d] by user [%d]", issue.ID, newRepo.ID, newIssue.ID, ctx.Doer.ID)

	ctx.Flash.Success("Issue copied successfully.")
	ctx.JSONRedirect(fmt.Sprintf("%s/issues/%d", newRepo.Link(), newIssue.Index))
}

// ListAccessibleReposForIssueMove returns a JSON list of repos where the user can write issues
func ListAccessibleReposForIssueMove(ctx *context.Context) {
	q := ctx.FormString("q")

	repos, _, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 20,
		},
		Actor:              ctx.Doer,
		Keyword:            q,
		Collaborate:        optional.Some(false),
		IncludeDescription: false,
		Archived:           optional.Some(false),
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}

	// Also search collaborative repos
	collabRepos, _, err := repo_model.SearchRepository(ctx, repo_model.SearchRepoOptions{
		ListOptions: db.ListOptions{
			Page:     1,
			PageSize: 20,
		},
		Actor:              ctx.Doer,
		Keyword:            q,
		Collaborate:        optional.Some(true),
		UnitType:           unit.TypeIssues,
		IncludeDescription: false,
		Archived:           optional.Some(false),
	})
	if err != nil {
		ctx.ServerError("SearchRepository", err)
		return
	}

	// Merge and deduplicate
	seen := make(map[int64]bool)
	type repoInfo struct {
		FullName string `json:"full_name"`
		ID       int64  `json:"id"`
	}
	var result []repoInfo

	for _, r := range append(repos, collabRepos...) {
		if seen[r.ID] {
			continue
		}
		// Check write permission for issues
		canWrite, err := access_model.HasAccessUnit(ctx, ctx.Doer, r, unit.TypeIssues, perm_model.AccessModeWrite)
		if err != nil {
			continue
		}
		if !canWrite {
			continue
		}
		seen[r.ID] = true
		result = append(result, repoInfo{
			FullName: r.FullName(),
			ID:       r.ID,
		})
	}

	ctx.JSON(http.StatusOK, result)
}
