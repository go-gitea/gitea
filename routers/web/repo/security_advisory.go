// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"strings"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	"gitea.dev/models/renderhelper"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/base"
	"gitea.dev/modules/cvss"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/routers/web/shared/issue"
	advisory_service "gitea.dev/services/advisory"
	"gitea.dev/services/context"
	"gitea.dev/services/forms"
)

const (
	tplSecurityAdvisories   templates.TplName = "repo/security_advisory/list"
	tplSecurityAdvisoryView templates.TplName = "repo/security_advisory/view"
	tplSecurityAdvisoryForm templates.TplName = "repo/security_advisory/form"
)

func securityAdvisoryViewer(ctx *context.Context) advisory_model.Viewer {
	return advisory_model.NewViewer(ctx.Doer, ctx.Repo.Permission, false)
}

// PrepareSecurityAdvisories runs after the unit reader check, which also rejects admins if the unit is disabled
func PrepareSecurityAdvisories(ctx *context.Context) {
	ctx.Data["PageIsSecurityAdvisories"] = true
	ctx.Data["CanManageSecurityAdvisories"] = securityAdvisoryViewer(ctx).CanManage() && !ctx.Repo.Repository.IsArchived
	ctx.Data["PrivateVulnerabilityReportingEnabled"] = ctx.IsSigned && ctx.Doer.IsIndividual() && advisory_service.IsPrivateReportingEnabled(ctx, ctx.Repo.Repository)
}

func SecurityAdvisories(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.security_advisories")
	viewer := securityAdvisoryViewer(ctx)
	labels := issue.PrepareFilterIssueLabels(ctx, ctx.Repo.Repository.ID, ctx.Repo.Repository.Owner)
	if ctx.Written() {
		return
	}

	opts := advisory_model.FindAdvisoriesOptions{
		ListOptions: db.ListOptions{Page: max(ctx.FormInt("page"), 1), PageSize: setting.UI.IssuePagingNum},
		RepoID:      ctx.Repo.Repository.ID,
		Viewer:      viewer,
		Keyword:     ctx.FormTrim("q"),
		Severity:    advisory_model.ParseSeverity(ctx.FormString("severity")),
		LabelIDs:    labels.SelectedLabelIDs,
		CloseReason: advisory_model.ParseCloseReason(ctx.FormString("close_reason")),
	}
	counts, err := advisory_model.CountAdvisoriesByState(ctx, opts)
	if err != nil {
		ctx.ServerError("CountAdvisoriesByState", err)
		return
	}
	state := ctx.FormTrim("state")
	switch state {
	case "triage", "draft", "closed":
		opts.States = []advisory_model.State{advisory_model.ParseState(state)}
	default:
		state = "published"
		opts.States = []advisory_model.State{advisory_model.StatePublished, advisory_model.StateWithdrawn}
		opts.SortBy = "published"
	}
	advisories, err := db.Find[advisory_model.Advisory](ctx, opts)
	if err != nil {
		ctx.ServerError("FindAdvisories", err)
		return
	}
	for _, a := range advisories {
		a.Repo = ctx.Repo.Repository
	}
	if err := advisory_model.List(advisories).LoadLabels(ctx); err != nil {
		ctx.ServerError("LoadLabels", err)
		return
	}
	var total int64
	for _, s := range opts.States {
		total += counts[s]
	}

	ctx.Data["Advisories"] = advisories
	ctx.Data["State"] = state
	ctx.Data["Keyword"] = opts.Keyword
	ctx.Data["SelectedSeverity"] = opts.Severity
	ctx.Data["Severities"] = advisory_model.Severities
	type stateTab struct {
		Name, Icon string
		Count      int64
	}
	tabs := []stateTab{{"published", "octicon-shield", counts[advisory_model.StatePublished] + counts[advisory_model.StateWithdrawn]}}
	if private := counts[advisory_model.StateTriage] + counts[advisory_model.StateDraft] + counts[advisory_model.StateClosed]; private > 0 || viewer.CanManage() {
		tabs = append(tabs,
			stateTab{"triage", "octicon-report", counts[advisory_model.StateTriage]},
			stateTab{"draft", "octicon-pencil", counts[advisory_model.StateDraft]},
			stateTab{"closed", "octicon-x-circle", counts[advisory_model.StateClosed]},
		)
	}
	ctx.Data["StateTabs"] = tabs
	ctx.Data["Page"] = context.NewPagerBuilder(ctx).TotalCount(total).PerPageLimit(setting.UI.IssuePagingNum).CurPage(opts.Page).Build()
	ctx.HTML(http.StatusOK, tplSecurityAdvisories)
}

func renderSecurityAdvisoryForm(ctx *context.Context, title string, isReport bool, content *advisory_service.ContentOptions) {
	if len(content.Vulnerabilities) == 0 {
		content.Vulnerabilities = []*advisory_model.Vulnerability{{Ecosystem: "other"}}
	}
	ctx.Data["Title"] = ctx.Tr(title)
	ctx.Data["IsReport"] = isReport
	ctx.Data["Content"] = content
	ctx.Data["CweIDsText"] = strings.Join(content.CweIDs, ", ")
	ctx.Data["Ecosystems"] = advisory_model.Ecosystems
	ctx.Data["Severities"] = advisory_model.Severities
	ctx.Data["CreditTypes"] = advisory_model.CreditTypes
	ctx.Data["CVSSMetricsV31"] = cvss.BaseMetrics(cvss.Version31)
	ctx.Data["CVSSMetricsV40"] = cvss.BaseMetrics(cvss.Version40)
	ctx.Data["CVSSPreviewLink"] = ctx.Repo.RepoLink + "/security/advisories/cvss"
	canManageContent := !isReport && securityAdvisoryViewer(ctx).CanManage()
	ctx.Data["CanManageContent"] = canManageContent
	if canManageContent {
		labels, err := advisory_service.RepoAndOrgLabels(ctx, ctx.Repo.Repository)
		if err != nil {
			ctx.ServerError("RepoAndOrgLabels", err)
			return
		}
		ctx.Data["Labels"] = labels
		ctx.Data["LabelIDsText"] = strings.Join(base.Int64sToStrings(content.LabelIDs), ",")
	}
	ctx.HTML(http.StatusOK, tplSecurityAdvisoryForm)
}

func formValueAt(values []string, i int) string {
	if i < len(values) {
		return values[i]
	}
	return ""
}

func contentFromForm(ctx *context.Context, form *forms.SecurityAdvisoryForm) (*advisory_service.ContentOptions, error) {
	content := &advisory_service.ContentOptions{
		ContentVersion: form.ContentVersion,
		Summary:        form.Summary,
		Description:    form.Content,
		CveID:          form.CveID,
		CvssV3Vector:   form.CvssV3Vector,
		CvssV4Vector:   form.CvssV4Vector,
		CweIDs:         strings.FieldsFunc(form.CweIDs, func(r rune) bool { return r == ',' || r == ' ' }),
	}
	labelIDs, err := base.StringsToInt64s(strings.Split(form.LabelIDs, ","))
	if err != nil {
		return nil, util.NewInvalidArgumentErrorf("invalid label")
	}
	content.LabelIDs = labelIDs
	if strings.TrimSpace(content.CvssV3Vector+content.CvssV4Vector) == "" {
		content.Severity = form.Severity
	}
	for i, ecosystem := range form.Ecosystem {
		v := &advisory_model.Vulnerability{
			Ecosystem:              ecosystem,
			PackageName:            formValueAt(form.PackageName, i),
			VulnerableVersionRange: formValueAt(form.VulnerableVersionRange, i),
			PatchedVersions:        formValueAt(form.PatchedVersions, i),
		}
		for f := range strings.SplitSeq(formValueAt(form.VulnerableFunctions, i), "\n") {
			if f = strings.TrimSpace(f); f != "" {
				v.VulnerableFunctions = append(v.VulnerableFunctions, f)
			}
		}
		if strings.TrimSpace(v.PackageName+v.VulnerableVersionRange+v.PatchedVersions) != "" || len(v.VulnerableFunctions) > 0 {
			content.Vulnerabilities = append(content.Vulnerabilities, v)
		}
	}
	content.Credits, err = advisory_service.CreditsByLogins(ctx, form.CreditUser, form.CreditType)
	return content, err
}

// jsonSecurityAdvisoryError responds to fetch-action requests, validation errors keep the form as it is
func jsonSecurityAdvisoryError(ctx *context.Context, err error) {
	if errors.Is(err, user_model.ErrBlockedUser) {
		ctx.JSONError(ctx.Tr("repo.security_advisories.blocked"))
		return
	}
	ctx.JSONErrorAuto(err)
}

// submitSecurityAdvisoryForm validates the posted form and saves it, then redirects to the advisory
func submitSecurityAdvisoryForm(ctx *context.Context, save func(*advisory_service.ContentOptions) (*advisory_model.Advisory, error)) {
	form := context.GetFetchActionForm[*forms.SecurityAdvisoryForm](ctx)
	if ctx.Written() {
		return
	}
	content, err := contentFromForm(ctx, form)
	if err == nil {
		var a *advisory_model.Advisory
		if a, err = save(content); err == nil {
			ctx.JSONRedirect(a.Link())
			return
		}
	}
	jsonSecurityAdvisoryError(ctx, err)
}

func NewSecurityAdvisory(ctx *context.Context) {
	renderSecurityAdvisoryForm(ctx, "repo.security_advisories.new", false, &advisory_service.ContentOptions{})
}

func NewSecurityAdvisoryPost(ctx *context.Context) {
	submitSecurityAdvisoryForm(ctx, func(content *advisory_service.ContentOptions) (*advisory_model.Advisory, error) {
		return advisory_service.CreateAdvisory(ctx, ctx.Doer, ctx.Repo.Repository, content)
	})
}

func MustAllowPrivateVulnerabilityReporting(ctx *context.Context) {
	if !ctx.Doer.IsIndividual() || !advisory_service.IsPrivateReportingEnabled(ctx, ctx.Repo.Repository) {
		ctx.NotFound(nil)
	}
}

func ReportVulnerability(ctx *context.Context) {
	renderSecurityAdvisoryForm(ctx, "repo.security_advisories.report", true, &advisory_service.ContentOptions{})
}

func ReportVulnerabilityPost(ctx *context.Context) {
	submitSecurityAdvisoryForm(ctx, func(content *advisory_service.ContentOptions) (*advisory_model.Advisory, error) {
		a, err := advisory_service.ReportVulnerability(ctx, ctx.Doer, ctx.Repo.Repository, content)
		if err == nil {
			ctx.Flash.Success(ctx.Tr("repo.security_advisories.report_success"))
		}
		return a, err
	})
}

// SecurityAdvisoryCVSSPreview validates a vector and returns its score for the calculator
func SecurityAdvisoryCVSSPreview(ctx *context.Context) {
	r, err := cvss.Parse(ctx.FormString("vector"))
	if err != nil {
		ctx.JSON(http.StatusOK, map[string]any{"error": ctx.Locale.TrString("repo.security_advisories.cvss.invalid")})
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"score":    r.Score,
		"severity": ctx.Locale.TrString("repo.security_advisories.severity." + advisory_service.SeverityFromRating(r.Rating).String()),
	})
}

type securityAdvisoryState struct {
	advisory_model.Permissions
	Advisory *advisory_model.Advisory
}

// LoadSecurityAdvisory responds with 404 if the doer cannot see the advisory, like for advisories that don't exist
func LoadSecurityAdvisory(ctx *context.Context) {
	a, perms, err := securityAdvisoryViewer(ctx).GetAdvisory(ctx, ctx.Repo.Repository, ctx.PathParam("identifier"))
	if err != nil {
		ctx.ServerError("GetAdvisory", err)
		return
	}
	perms.CanEdit = perms.CanEdit && !ctx.Repo.Repository.IsArchived
	ctx.Data["AdvisoryState"] = &securityAdvisoryState{Permissions: perms, Advisory: a}
}

// getSecurityAdvisoryState returns the state set by LoadSecurityAdvisory which runs before all handlers using it
func getSecurityAdvisoryState(ctx *context.Context) *securityAdvisoryState {
	state, _ := ctx.Data["AdvisoryState"].(*securityAdvisoryState)
	return state
}

// getLoadedSecurityAdvisory returns the advisory with the attributes that only the view and edit pages need
func getLoadedSecurityAdvisory(ctx *context.Context) *advisory_model.Advisory {
	a := getSecurityAdvisoryState(ctx).Advisory
	if err := a.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return nil
	}
	return a
}

func renderSecurityAdvisoryMarkdown(ctx *context.Context, footnoteID, content string) (template.HTML, error) {
	rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{FootnoteContextID: footnoteID})
	return markdown.RenderString(rctx, content)
}

func MustManageSecurityAdvisory(ctx *context.Context) {
	if !securityAdvisoryViewer(ctx).CanManage() {
		ctx.HTTPError(http.StatusForbidden)
	}
}

func MustEditSecurityAdvisory(ctx *context.Context) {
	if !getSecurityAdvisoryState(ctx).CanEdit {
		ctx.HTTPError(http.StatusForbidden)
	}
}

func MustSeeSecurityAdvisoryDiscussion(ctx *context.Context) {
	if !getSecurityAdvisoryState(ctx).CanSeeDiscussion {
		ctx.HTTPError(http.StatusForbidden)
	}
}

func ViewSecurityAdvisory(ctx *context.Context) {
	state := getSecurityAdvisoryState(ctx)
	a := getLoadedSecurityAdvisory(ctx)
	if ctx.Written() {
		return
	}
	viewer := securityAdvisoryViewer(ctx)
	if err := viewer.HideUnviewableOriginals(ctx, advisory_model.List{a}); err != nil {
		ctx.ServerError("HideUnviewableOriginals", err)
		return
	}
	ctx.Data["Title"] = a.Summary
	var err error
	if ctx.Data["RenderedDescription"], err = renderSecurityAdvisoryMarkdown(ctx, "description", a.Description); err != nil {
		ctx.ServerError("RenderString", err)
		return
	}
	if state.CanSeeDiscussion {
		comments, err := db.Find[advisory_model.Comment](ctx, advisory_model.FindCommentsOptions{AdvisoryID: a.ID})
		if err != nil {
			ctx.ServerError("FindComments", err)
			return
		}
		if err := advisory_model.CommentList(comments).LoadPosters(ctx); err != nil {
			ctx.ServerError("LoadPosters", err)
			return
		}
		for _, c := range comments {
			if c.RenderedContent, err = renderSecurityAdvisoryMarkdown(ctx, strconv.FormatInt(c.ID, 10), c.Content); err != nil {
				ctx.ServerError("RenderString", err)
				return
			}
		}
		if err := a.LoadCollaborators(ctx); err != nil {
			ctx.ServerError("LoadCollaborators", err)
			return
		}
		ctx.Data["Comments"] = comments
	}
	var targets []advisory_model.State
	for _, s := range []advisory_model.State{advisory_model.StateDraft, advisory_model.StatePublished, advisory_model.StateWithdrawn} {
		if a.State.CanTransitionTo(s) {
			targets = append(targets, s)
		}
	}
	ctx.Data["StateTargets"] = targets
	if a.State.CanTransitionTo(advisory_model.StateClosed) && viewer.CanManage() {
		others, err := db.Find[advisory_model.Advisory](ctx, advisory_model.FindAdvisoriesOptions{
			ListOptions: db.ListOptions{PageSize: 100}, // the most recent ones, older originals can be chosen by the API
			RepoID:      a.RepoID,
			Viewer:      viewer,
		})
		if err != nil {
			ctx.ServerError("FindAdvisories", err)
			return
		}
		ctx.Data["DuplicateCandidates"] = slices.DeleteFunc(others, func(o *advisory_model.Advisory) bool { return o.ID == a.ID })
		ctx.Data["CanCloseSecurityAdvisory"] = true
	}
	ctx.HTML(http.StatusOK, tplSecurityAdvisoryView)
}

func EditSecurityAdvisory(ctx *context.Context) {
	a := getLoadedSecurityAdvisory(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Advisory"] = a
	renderSecurityAdvisoryForm(ctx, "repo.security_advisories.edit", false, advisory_service.ContentFromAdvisory(a))
}

func EditSecurityAdvisoryPost(ctx *context.Context) {
	a := getLoadedSecurityAdvisory(ctx)
	if ctx.Written() {
		return
	}
	submitSecurityAdvisoryForm(ctx, func(content *advisory_service.ContentOptions) (*advisory_model.Advisory, error) {
		return a, advisory_service.UpdateAdvisory(ctx, ctx.Doer, a, content, securityAdvisoryViewer(ctx).CanManage())
	})
}

func ChangeSecurityAdvisoryState(ctx *context.Context) {
	a := getSecurityAdvisoryState(ctx).Advisory
	closeReason := ctx.FormString("close_reason")
	// the modal keeps a chosen original when switching to another reason
	duplicateOf := util.Iif(closeReason == advisory_model.CloseReasonDuplicate.String(), ctx.FormTrim("duplicate_of"), "")
	opts, err := advisory_service.NewStateOptions(ctx, a, ctx.FormString("state"), closeReason, duplicateOf)
	if err == nil {
		err = advisory_service.ChangeState(ctx, ctx.Doer, a, opts)
	}
	if err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.JSONRedirect(a.Link())
}

func DeleteSecurityAdvisory(ctx *context.Context) {
	if err := advisory_service.DeleteAdvisory(ctx, ctx.Doer, getSecurityAdvisoryState(ctx).Advisory); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("repo.security_advisories.deletion_success"))
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/security/advisories")
}

func AddSecurityAdvisoryCollaborator(ctx *context.Context) {
	a := getSecurityAdvisoryState(ctx).Advisory
	name := ctx.FormTrim("collaborator")
	var user *user_model.User
	var team *organization.Team
	var err error
	if ctx.FormString("type") == "team" {
		team, err = organization.GetTeam(ctx, ctx.Repo.Repository.OwnerID, name)
	} else {
		user, err = user_model.GetUserByName(ctx, name)
	}
	if err == nil {
		err = advisory_service.AddCollaborator(ctx, ctx.Doer, a, user, team)
	}
	switch {
	case err == nil:
		ctx.JSONRedirect(a.Link())
	case errors.Is(err, util.ErrNotExist):
		ctx.JSONError(ctx.Tr("repo.security_advisories.collaborator.not_exist"))
	case errors.Is(err, user_model.ErrBlockedUser):
		ctx.JSONError(ctx.Tr("repo.security_advisories.collaborator.blocked"))
	case errors.Is(err, util.ErrInvalidArgument):
		ctx.JSONError(ctx.Tr("repo.security_advisories.collaborator.no_access"))
	default:
		ctx.ServerError("AddCollaborator", err)
	}
}

func RemoveSecurityAdvisoryCollaborator(ctx *context.Context) {
	a := getSecurityAdvisoryState(ctx).Advisory
	if err := a.LoadCollaborators(ctx); err != nil {
		ctx.ServerError("LoadCollaborators", err)
		return
	}
	// only the current collaborators can be removed, which also limits the teams to the repository owner
	userID, teamID := ctx.FormInt64("user_id"), ctx.FormInt64("team_id")
	for _, u := range a.CollaboratorUsers {
		if u.ID == userID {
			if err := advisory_service.RemoveCollaborator(ctx, ctx.Doer, a, u, nil); err != nil {
				ctx.ServerError("RemoveCollaborator", err)
				return
			}
		}
	}
	for _, t := range a.CollaboratorTeams {
		if t.ID == teamID {
			if err := advisory_service.RemoveCollaborator(ctx, ctx.Doer, a, nil, t); err != nil {
				ctx.ServerError("RemoveCollaborator", err)
				return
			}
		}
	}
	ctx.JSONRedirect(a.Link())
}

func NewSecurityAdvisoryComment(ctx *context.Context) {
	a := getSecurityAdvisoryState(ctx).Advisory
	form := context.GetFetchActionForm[*forms.SecurityAdvisoryCommentForm](ctx)
	if ctx.Written() {
		return
	}
	c, err := advisory_service.CreateComment(ctx, ctx.Doer, a, form.Content)
	if err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.JSONRedirect(fmt.Sprintf("%s#advisory-comment-%d", a.Link(), c.ID))
}

func getSecurityAdvisoryComment(ctx *context.Context) *advisory_model.Comment {
	c, err := advisory_model.GetCommentByID(ctx, getSecurityAdvisoryState(ctx).Advisory.ID, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.ServerError("GetCommentByID", err)
		return nil
	}
	return c
}

// EditSecurityAdvisoryComment responds to the in-place editor of the issue comments, only the poster can edit
func EditSecurityAdvisoryComment(ctx *context.Context) {
	c := getSecurityAdvisoryComment(ctx)
	if ctx.Written() {
		return
	}
	if err := advisory_service.EditComment(ctx, ctx.Doer, getSecurityAdvisoryState(ctx).Advisory, c, ctx.FormString("content")); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	rendered, err := renderSecurityAdvisoryMarkdown(ctx, strconv.FormatInt(c.ID, 10), c.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{"content": rendered, "contentVersion": 0, "attachments": ""})
}

func DeleteSecurityAdvisoryComment(ctx *context.Context) {
	c := getSecurityAdvisoryComment(ctx)
	if ctx.Written() {
		return
	}
	if err := advisory_service.DeleteComment(ctx, ctx.Doer, c, securityAdvisoryViewer(ctx).CanManage()); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.JSONOK()
}
