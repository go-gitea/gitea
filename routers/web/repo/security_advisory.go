// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
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
	"gitea.dev/modules/optional"
	"gitea.dev/modules/reqctx"
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
	return advisory_model.Viewer{Doer: ctx.Doer, IsRepoAdmin: ctx.Repo.Permission.IsAdmin()}
}

func PrepareSecurityAdvisories(ctx *context.Context) {
	ctx.Data["PageIsSecurityAdvisories"] = true
	ctx.Data["CanManageSecurityAdvisories"] = securityAdvisoryViewer(ctx).CanManage() && !ctx.Repo.Repository.IsArchived
	ctx.Data["PrivateVulnerabilityReportingEnabled"] = ctx.IsSigned && ctx.Doer.IsIndividual() && advisory_service.IsPrivateReportingEnabled(ctx, ctx.Repo.Repository)
}

type securityAdvisoryStateTab struct {
	Name, Icon string
	Count      int64
}

func SecurityAdvisories(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.security_advisories")
	viewer := securityAdvisoryViewer(ctx)
	labels := issue.PrepareFilterIssueLabels(ctx, ctx.Repo.Repository.ID, ctx.Repo.Repository.Owner)
	if ctx.Written() {
		return
	}

	opts, err := advisory_service.ParseListFilters(advisory_service.ListFilters{
		Keyword:     ctx.FormTrim("q"),
		Severity:    ctx.FormString("severity"),
		LabelIDs:    labels.SelectedLabelIDs,
		CloseReason: ctx.FormString("close_reason"),
	})
	if err != nil {
		ctx.HTTPError(http.StatusBadRequest, err.Error())
		return
	}
	opts.ListOptions = db.ListOptions{Page: max(ctx.FormInt("page"), 1), PageSize: setting.UI.IssuePagingNum}
	opts.RepoID = ctx.Repo.Repository.ID
	opts.Viewer = viewer
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
	// the published tab also lists the withdrawn advisories, so it has a neutral icon
	tabs := []securityAdvisoryStateTab{{"published", "octicon-shield", counts[advisory_model.StatePublished] + counts[advisory_model.StateWithdrawn]}}
	if private := counts[advisory_model.StateTriage] + counts[advisory_model.StateDraft] + counts[advisory_model.StateClosed]; private > 0 || viewer.CanManage() {
		for _, s := range []advisory_model.State{advisory_model.StateTriage, advisory_model.StateDraft, advisory_model.StateClosed} {
			tabs = append(tabs, securityAdvisoryStateTab{s.String(), s.Icon(), counts[s]})
		}
	}
	ctx.Data["StateTabs"] = tabs
	ctx.Data["Page"] = context.NewPagerBuilder(ctx).TotalCount(total).PerPageLimit(setting.UI.IssuePagingNum).CurPage(opts.Page).Build()
	ctx.HTML(http.StatusOK, tplSecurityAdvisories)
}

type securityAdvisoryFormPage struct {
	TitleKey, DescriptionKey, SubmitKey string
	CancelLink                          string
	IsReport                            bool
}

func newSecurityAdvisoryVulnerability() *advisory_model.Vulnerability {
	return &advisory_model.Vulnerability{Ecosystem: "other"}
}

func renderSecurityAdvisoryForm(ctx *context.Context, page securityAdvisoryFormPage, content *advisory_service.ContentOptions) {
	if len(content.Vulnerabilities) == 0 {
		content.Vulnerabilities = []*advisory_model.Vulnerability{newSecurityAdvisoryVulnerability()}
	}
	ctx.Data["Title"] = ctx.Tr(page.TitleKey)
	ctx.Data["DescriptionKey"] = page.DescriptionKey
	ctx.Data["SubmitKey"] = page.SubmitKey
	ctx.Data["CancelLink"] = page.CancelLink
	ctx.Data["Content"] = content
	ctx.Data["NewVulnerability"] = newSecurityAdvisoryVulnerability()
	ctx.Data["NewCredit"] = &advisory_model.Credit{Type: "finder", User: &user_model.User{}} // no user chosen yet
	ctx.Data["CweIDsText"] = strings.Join(content.CweIDs, ", ")
	ctx.Data["Ecosystems"] = advisory_model.Ecosystems
	ctx.Data["Severities"] = advisory_model.Severities
	ctx.Data["CreditTypes"] = advisory_model.CreditTypes
	ctx.Data["CVSSMetricsV31"] = cvss.BaseMetrics(cvss.Version31)
	ctx.Data["CVSSMetricsV40"] = cvss.BaseMetrics(cvss.Version40)
	ctx.Data["CVSSPreviewLink"] = ctx.Repo.RepoLink + "/security/advisories/cvss"
	canManageContent := !page.IsReport && securityAdvisoryViewer(ctx).CanManage()
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
		content.Severity = form.Severity // the form always submits the manual severity, which only applies without vectors
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
		ctx.JSONError(ctx.Tr("repo.action.blocked_user"))
		return
	}
	ctx.JSONErrorAuto(err)
}

func NewSecurityAdvisory(ctx *context.Context) {
	renderSecurityAdvisoryForm(ctx, securityAdvisoryFormPage{
		TitleKey:       "repo.security_advisories.new",
		DescriptionKey: "repo.security_advisories.form_desc",
		SubmitKey:      "repo.security_advisories.create",
		CancelLink:     ctx.Repo.RepoLink + "/security/advisories",
	}, &advisory_service.ContentOptions{})
}

func NewSecurityAdvisoryPost(ctx *context.Context) {
	form := context.GetFetchActionForm[*forms.SecurityAdvisoryForm](ctx)
	if ctx.Written() {
		return
	}
	content, err := contentFromForm(ctx, form)
	if err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	a, err := advisory_service.CreateAdvisory(ctx, ctx.Doer, ctx.Repo.Repository, content)
	if err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.JSONRedirect(a.Link())
}

func MustAllowPrivateVulnerabilityReporting(ctx *context.Context) {
	if !ctx.Doer.IsIndividual() || !advisory_service.IsPrivateReportingEnabled(ctx, ctx.Repo.Repository) {
		ctx.NotFound(nil)
	}
}

func ReportVulnerability(ctx *context.Context) {
	renderSecurityAdvisoryForm(ctx, securityAdvisoryFormPage{
		TitleKey:       "repo.security_advisories.report",
		DescriptionKey: "repo.security_advisories.report_desc",
		SubmitKey:      "repo.security_advisories.report_submit",
		CancelLink:     ctx.Repo.RepoLink + "/security/advisories",
		IsReport:       true,
	}, &advisory_service.ContentOptions{})
}

func ReportVulnerabilityPost(ctx *context.Context) {
	form := context.GetFetchActionForm[*forms.SecurityAdvisoryForm](ctx)
	if ctx.Written() {
		return
	}
	content, err := contentFromForm(ctx, form)
	if err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	a, err := advisory_service.ReportVulnerability(ctx, ctx.Doer, ctx.Repo.Repository, content)
	if err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("repo.security_advisories.report_success"))
	ctx.JSONRedirect(a.Link())
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
		"severity": ctx.Locale.TrString(advisory_service.SeverityFromRating(r.Rating).LocaleKey()),
	})
}

type securityAdvisoryContextKey struct{}

type loadedSecurityAdvisory struct {
	advisory *advisory_model.Advisory
	perms    advisory_model.Permissions
}

// LoadSecurityAdvisory responds with 404 if the doer cannot see the advisory, like for advisories that don't exist
func LoadSecurityAdvisory(ctx *context.Context) {
	a, perms, err := securityAdvisoryViewer(ctx).GetAdvisory(ctx, ctx.Repo.Repository, ctx.PathParam("identifier"))
	if err != nil {
		ctx.ServerError("GetAdvisory", err)
		return
	}
	perms.CanEdit = perms.CanEdit && !ctx.Repo.Repository.IsArchived
	ctx.SetContextValue(securityAdvisoryContextKey{}, &loadedSecurityAdvisory{advisory: a, perms: perms})
	ctx.Data["Advisory"] = a
	ctx.Data["AdvisoryPerms"] = perms
}

func getSecurityAdvisory(ctx *context.Context) (*advisory_model.Advisory, advisory_model.Permissions) {
	loaded := reqctx.MustContextValue[*loadedSecurityAdvisory](ctx, securityAdvisoryContextKey{})
	return loaded.advisory, loaded.perms
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
	if _, perms := getSecurityAdvisory(ctx); !perms.CanEdit {
		ctx.HTTPError(http.StatusForbidden)
	}
}

func MustSeeSecurityAdvisoryDiscussion(ctx *context.Context) {
	if _, perms := getSecurityAdvisory(ctx); !perms.CanSeeDiscussion {
		ctx.HTTPError(http.StatusForbidden)
	}
}

func ViewSecurityAdvisory(ctx *context.Context) {
	a, perms := getSecurityAdvisory(ctx)
	if _, err := securityAdvisoryViewer(ctx).LoadForDisplay(ctx, advisory_model.List{a}); err != nil {
		ctx.ServerError("LoadForDisplay", err)
		return
	}
	ctx.Data["Title"] = a.Summary
	var err error
	if ctx.Data["RenderedDescription"], err = renderSecurityAdvisoryMarkdown(ctx, "description", a.Description); err != nil {
		ctx.ServerError("RenderString", err)
		return
	}
	if perms.CanSeeDiscussion {
		prepareSecurityAdvisoryComments(ctx, a)
		if ctx.Written() {
			return
		}
	}
	ctx.Data["StatusText"] = securityAdvisoryStatusText(ctx, a, perms)
	ctx.Data["DescriptionAuthor"] = util.Iif(perms.CanSeeDiscussion, a.Reporter, a.Publisher)
	ctx.Data["CVSSScores"] = securityAdvisoryCVSSScores(a)
	ctx.Data["Collaborators"] = securityAdvisoryCollaborators(a)
	ctx.Data["CollaboratorForms"] = securityAdvisoryCollaboratorForms(ctx)
	ctx.Data["StateActions"] = securityAdvisoryStateActions(a.State)
	if perms.CanManage && a.State.CanTransitionTo(advisory_model.StateClosed) {
		prepareSecurityAdvisoryCloseForm(ctx, a)
		if ctx.Written() {
			return
		}
	}
	ctx.HTML(http.StatusOK, tplSecurityAdvisoryView)
}

// securityAdvisoryStatusText is shown next to the state, only the participants see the reporter
func securityAdvisoryStatusText(ctx *context.Context, a *advisory_model.Advisory, perms advisory_model.Permissions) template.HTML {
	switch {
	case a.DuplicateOf != nil:
		return ctx.Tr("repo.security_advisories.closed_as_duplicate_of", a.DuplicateOf.Link(), a.DuplicateOf.Identifier)
	case a.CloseReason != advisory_model.CloseReasonNone:
		return ctx.Tr("repo.security_advisories.closed_as." + a.CloseReason.String())
	case a.Publisher != nil:
		return ctx.Tr("repo.security_advisories.published_by", templates.TimeSince(a.PublishedUnix), a.Publisher.HomeLink(), a.Publisher.GetDisplayName())
	case perms.CanSeeDiscussion:
		key := util.Iif(a.IsReport, "repo.security_advisories.reported_by", "repo.issues.opened_by")
		return ctx.Tr(key, templates.TimeSince(a.CreatedUnix), a.Reporter.HomeLink(), a.Reporter.GetDisplayName())
	}
	return ""
}

type securityAdvisoryCVSSScore struct {
	Version cvss.Version
	Score   float64
	Vector  string
}

func securityAdvisoryCVSSScores(a *advisory_model.Advisory) (scores []securityAdvisoryCVSSScore) {
	if a.CvssV4Vector != "" {
		scores = append(scores, securityAdvisoryCVSSScore{cvss.Version40, a.CvssV4Score(), a.CvssV4Vector})
	}
	if a.CvssV3Vector != "" {
		scores = append(scores, securityAdvisoryCVSSScore{cvss.Version31, a.CvssV3Score(), a.CvssV3Vector})
	}
	return scores
}

// securityAdvisoryCollaborator is either a user or a team
type securityAdvisoryCollaborator struct {
	User       *user_model.User
	Team       *organization.Team
	RemoveLink string
}

func securityAdvisoryCollaborators(a *advisory_model.Advisory) []securityAdvisoryCollaborator {
	collaborators := make([]securityAdvisoryCollaborator, 0, len(a.CollaboratorUsers)+len(a.CollaboratorTeams))
	for _, u := range a.CollaboratorUsers {
		collaborators = append(collaborators, securityAdvisoryCollaborator{User: u, RemoveLink: fmt.Sprintf("%s/collaborators/delete?user_id=%d", a.Link(), u.ID)})
	}
	for _, t := range a.CollaboratorTeams {
		collaborators = append(collaborators, securityAdvisoryCollaborator{Team: t, RemoveLink: fmt.Sprintf("%s/collaborators/delete?team_id=%d", a.Link(), t.ID)})
	}
	return collaborators
}

type securityAdvisoryCollaboratorForm struct {
	Type, Icon, SearchInit, OrgName string
	PlaceholderKey, SubmitKey       string
}

func securityAdvisoryCollaboratorForms(ctx *context.Context) []securityAdvisoryCollaboratorForm {
	forms := []securityAdvisoryCollaboratorForm{{
		Type:           "user",
		Icon:           "octicon-person-add",
		SearchInit:     "initSecurityAdvisoryUserSearch",
		PlaceholderKey: "search.user_kind",
		SubmitKey:      "repo.settings.add_collaborator",
	}}
	if owner := ctx.Repo.Repository.Owner; owner.IsOrganization() {
		forms = append(forms, securityAdvisoryCollaboratorForm{
			Type:           "team",
			Icon:           "octicon-people",
			SearchInit:     "initSecurityAdvisoryTeamSearch",
			OrgName:        owner.Name,
			PlaceholderKey: "search.team_kind",
			SubmitKey:      "repo.settings.add_team",
		})
	}
	return forms
}

type securityAdvisoryStateAction struct {
	State                             advisory_model.State
	ButtonClass, LabelKey, ConfirmKey string
}

// securityAdvisoryStateActions are the state buttons, closing has its own form for the reason
func securityAdvisoryStateActions(current advisory_model.State) []securityAdvisoryStateAction {
	var actions []securityAdvisoryStateAction
	for _, target := range current.Transitions() {
		action := securityAdvisoryStateAction{State: target, LabelKey: "repo.security_advisories.action." + target.String()}
		switch target {
		case advisory_model.StatePublished:
			action.ButtonClass = "primary"
			action.ConfirmKey = "repo.security_advisories.action.published_confirm"
		case advisory_model.StateWithdrawn:
			action.ButtonClass = "red"
			action.ConfirmKey = "repo.security_advisories.action.withdrawn_confirm"
		case advisory_model.StateDraft:
			if current == advisory_model.StateClosed {
				action.LabelKey = "repo.security_advisories.action.reopen"
			}
		}
		actions = append(actions, action)
	}
	return actions
}

func prepareSecurityAdvisoryComments(ctx *context.Context, a *advisory_model.Advisory) {
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
	ctx.Data["Comments"] = comments
}

func prepareSecurityAdvisoryCloseForm(ctx *context.Context, a *advisory_model.Advisory) {
	candidates, err := db.Find[advisory_model.Advisory](ctx, advisory_model.FindAdvisoriesOptions{
		ListOptions: db.ListOptions{PageSize: 100}, // the most recent ones, older originals can be chosen by the API
		RepoID:      a.RepoID,
		ExcludeID:   a.ID,
		Viewer:      securityAdvisoryViewer(ctx),
	})
	if err != nil {
		ctx.ServerError("FindAdvisories", err)
		return
	}
	ctx.Data["DuplicateCandidates"] = candidates
	ctx.Data["CanCloseSecurityAdvisory"] = true
}

func EditSecurityAdvisory(ctx *context.Context) {
	a, _ := getSecurityAdvisory(ctx)
	if err := a.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}
	renderSecurityAdvisoryForm(ctx, securityAdvisoryFormPage{
		TitleKey:       "repo.security_advisories.edit",
		DescriptionKey: "repo.security_advisories.form_desc",
		SubmitKey:      "save",
		CancelLink:     a.Link(),
	}, advisory_service.ContentFromAdvisory(a))
}

func EditSecurityAdvisoryPost(ctx *context.Context) {
	a, perms := getSecurityAdvisory(ctx)
	form := context.GetFetchActionForm[*forms.SecurityAdvisoryForm](ctx)
	if ctx.Written() {
		return
	}
	if err := a.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}
	content, err := contentFromForm(ctx, form)
	if err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	opts := advisory_service.EditOptions{
		Summary:         optional.Some(content.Summary),
		Description:     optional.Some(content.Description),
		Severity:        optional.Some(content.Severity),
		CvssV3Vector:    optional.Some(content.CvssV3Vector),
		CvssV4Vector:    optional.Some(content.CvssV4Vector),
		CweIDs:          optional.Some(content.CweIDs),
		Vulnerabilities: optional.Some(content.Vulnerabilities),
		ContentVersion:  optional.Some(content.ContentVersion),
	}
	if perms.CanManage { // the other participants don't get these fields in the form
		opts.CveID = optional.Some(content.CveID)
		opts.Credits = optional.Some(content.Credits)
		opts.LabelIDs = optional.Some(content.LabelIDs)
	}
	if err := advisory_service.EditAdvisory(ctx, ctx.Doer, a, perms, opts); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.JSONRedirect(a.Link())
}

func ChangeSecurityAdvisoryState(ctx *context.Context) {
	a, _ := getSecurityAdvisory(ctx)
	closeReason := ctx.FormString("close_reason")
	// the modal keeps a chosen original when switching to another reason
	duplicateOf := util.Iif(closeReason == advisory_model.CloseReasonDuplicate.String(), ctx.FormTrim("duplicate_of"), "")
	opts, err := advisory_service.NewStateOptions(ctx, a, ctx.FormString("state"), closeReason, duplicateOf)
	if err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	if err := advisory_service.ChangeState(ctx, ctx.Doer, a, opts); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.JSONRedirect(a.Link())
}

func DeleteSecurityAdvisory(ctx *context.Context) {
	a, _ := getSecurityAdvisory(ctx)
	if err := advisory_service.DeleteAdvisory(ctx, ctx.Doer, a); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("repo.security_advisories.deletion_success"))
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/security/advisories")
}

func AddSecurityAdvisoryCollaborator(ctx *context.Context) {
	a, _ := getSecurityAdvisory(ctx)
	err := advisory_service.AddCollaborator(ctx, ctx.Doer, a, ctx.FormTrim("collaborator"), ctx.FormString("type") == "team")
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
	a, _ := getSecurityAdvisory(ctx)
	if err := advisory_service.RemoveCollaborator(ctx, ctx.Doer, a, ctx.FormInt64("user_id"), ctx.FormInt64("team_id")); err != nil {
		ctx.ServerError("RemoveCollaborator", err)
		return
	}
	ctx.JSONRedirect(a.Link())
}

func NewSecurityAdvisoryComment(ctx *context.Context) {
	a, _ := getSecurityAdvisory(ctx)
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
	a, _ := getSecurityAdvisory(ctx)
	c, err := advisory_model.GetCommentByID(ctx, a.ID, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.ServerError("GetCommentByID", err)
		return nil
	}
	return c
}

// EditSecurityAdvisoryComment responds to the in-place editor of the issue comments, only the poster can edit
func EditSecurityAdvisoryComment(ctx *context.Context) {
	a, _ := getSecurityAdvisory(ctx)
	c := getSecurityAdvisoryComment(ctx)
	if ctx.Written() {
		return
	}
	if err := advisory_service.EditComment(ctx, ctx.Doer, a, c, ctx.FormString("content")); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	rendered, err := renderSecurityAdvisoryMarkdown(ctx, strconv.FormatInt(c.ID, 10), c.Content)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"content":        rendered,
		"contentVersion": 0,  // advisory comments have no concurrent edit detection
		"attachments":    "", // advisory comments have no attachments
	})
}

func DeleteSecurityAdvisoryComment(ctx *context.Context) {
	_, perms := getSecurityAdvisory(ctx)
	c := getSecurityAdvisoryComment(ctx)
	if ctx.Written() {
		return
	}
	if err := advisory_service.DeleteComment(ctx, ctx.Doer, perms, c); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.JSONOK()
}
