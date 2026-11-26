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
	"gitea.dev/modules/optional"
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
	return advisory_service.RequestViewer(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.Permission, false)
}

func PrepareSecurityAdvisories(ctx *context.Context) {
	ctx.Data["PageIsSecurityAdvisories"] = true
	ctx.Data["CanManageSecurityAdvisories"] = securityAdvisoryViewer(ctx).CanManage() && !ctx.Repo.Repository.IsArchived
	ctx.Data["PrivateVulnerabilityReportingEnabled"] = ctx.IsSigned && advisory_service.IsPrivateReportingEnabledForUser(ctx, ctx.Repo.Repository, ctx.Doer)
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
		ctx.ServerErrorAuto(err)
		return
	}
	state := ctx.FormTrim("state")
	if !slices.Contains([]string{"triage", "draft", "published", "closed"}, state) {
		state = util.Iif(viewer.CanManage(), "triage", "published")
	}
	if state == "published" {
		opts.States = []advisory_model.State{advisory_model.StatePublished, advisory_model.StateWithdrawn}
		opts.SortBy = "published"
	} else {
		opts.States = []advisory_model.State{advisory_model.ParseState(state)}
	}
	advisories, err := db.Find[advisory_model.Advisory](ctx, opts)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	for _, advisory := range advisories {
		advisory.Repo = ctx.Repo.Repository
	}
	discussionIDs, err := viewer.DiscussionIDs(ctx, advisories)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	withDiscussion := slices.DeleteFunc(slices.Clone(advisories), func(advisory *advisory_model.Advisory) bool { return !discussionIDs.Contains(advisory.ID) })
	if err := advisory_model.List(withDiscussion).LoadLabels(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	if err := advisory_model.List(advisories).LoadUsers(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	var total int64
	for _, tabState := range opts.States {
		total += counts[tabState]
	}

	isFiltered := opts.Keyword != "" || opts.Severity != advisory_model.SeverityUnknown || len(opts.LabelIDs) > 0 || opts.CloseReason != advisory_model.CloseReasonNone
	var visibleTotal int64
	for _, count := range counts {
		visibleTotal += count
	}

	ctx.Data["Advisories"] = advisories
	ctx.Data["DiscussionAdvisoryIDs"] = discussionIDs
	ctx.Data["HasAnyAdvisory"] = isFiltered || visibleTotal > 0
	ctx.Data["State"] = state
	ctx.Data["Keyword"] = opts.Keyword
	ctx.Data["SelectedSeverity"] = opts.Severity
	ctx.Data["CloseReason"] = opts.CloseReason.String()
	ctx.Data["Severities"] = advisory_model.Severities
	published := securityAdvisoryStateTab{"published", advisory_model.StatePublished.Icon(), counts[advisory_model.StatePublished] + counts[advisory_model.StateWithdrawn]}
	tabs := []securityAdvisoryStateTab{published}
	if counts[advisory_model.StateTriage]+counts[advisory_model.StateDraft]+counts[advisory_model.StateClosed] > 0 || viewer.CanManage() {
		tabs = []securityAdvisoryStateTab{
			{"triage", advisory_model.StateTriage.Icon(), counts[advisory_model.StateTriage]},
			{"draft", advisory_model.StateDraft.Icon(), counts[advisory_model.StateDraft]},
			published,
			{"closed", advisory_model.StateClosed.Icon(), counts[advisory_model.StateClosed]},
		}
	}
	ctx.Data["StateTabs"] = tabs
	ctx.Data["Page"] = context.NewPagerBuilder(ctx).TotalCount(total).PerPageLimit(setting.UI.IssuePagingNum).CurPage(opts.Page).Build()
	ctx.HTML(http.StatusOK, tplSecurityAdvisories)
}

type securityAdvisoryFormPage struct {
	TitleKey, DescriptionKey, SubmitKey string
	CancelLink                          string
	IsNew, IsReport                     bool
}

type securityAdvisoryCalculator struct {
	Version          cvss.Version
	SpecLink, Vector string
	Metrics          []cvss.Metric
	Selected         bool
}

func renderSecurityAdvisoryForm(ctx *context.Context, page securityAdvisoryFormPage, content *advisory_service.ContentOptions) {
	newVulnerability := &advisory_model.Vulnerability{}
	if len(content.Vulnerabilities) == 0 {
		content.Vulnerabilities = []*advisory_model.Vulnerability{newVulnerability}
	}
	ctx.Data["Title"] = ctx.Tr(page.TitleKey)
	ctx.Data["DescriptionKey"] = page.DescriptionKey
	ctx.Data["SubmitKey"] = page.SubmitKey
	ctx.Data["CancelLink"] = page.CancelLink
	ctx.Data["Content"] = content
	ctx.Data["NewVulnerability"] = newVulnerability
	ctx.Data["NewCredit"] = &advisory_model.Credit{Type: "finder", User: &user_model.User{}}
	ctx.Data["CweIDsText"] = strings.Join(content.CweIDs, ", ")
	ctx.Data["Ecosystems"] = advisory_model.Ecosystems
	ctx.Data["Severities"] = advisory_model.Severities
	ctx.Data["CreditTypes"] = advisory_model.CreditTypes
	calculators := []securityAdvisoryCalculator{
		{Version: cvss.Version40, SpecLink: "https://www.first.org/cvss/v4.0/specification-document"},
		{Version: cvss.Version31, SpecLink: "https://www.first.org/cvss/v3.1/specification-document"},
	}
	severity := content.Severity
	version := cvss.DetectVersion(content.CvssVector)
	if version != "" {
		severity = "cvss:" + string(version)
	}
	for i := range calculators {
		calculator := &calculators[i]
		calculator.Metrics = cvss.BaseMetrics(calculator.Version)
		if calculator.Version == version {
			calculator.Selected = true
			calculator.Vector = content.CvssVector
		}
	}
	ctx.Data["FormSeverity"] = severity
	ctx.Data["CVSSCalculators"] = calculators
	ctx.Data["CVSSPreviewLink"] = ctx.Repo.RepoLink + "/security/advisories/cvss"
	ctx.Data["CanManageContent"] = !page.IsReport && securityAdvisoryViewer(ctx).CanManage()
	if page.IsNew {
		labels, err := advisory_service.RepoAndOrgLabels(ctx, ctx.Repo.Repository)
		if err != nil {
			ctx.ServerErrorAuto(err)
			return
		}
		ctx.Data["Labels"] = labels
	}
	ctx.HTML(http.StatusOK, tplSecurityAdvisoryForm)
}

func formValueAt(values []string, i int) string {
	if i < len(values) {
		return values[i]
	}
	return ""
}

func contentFromForm(ctx *context.Context) *advisory_service.ContentOptions {
	form := context.GetFetchActionForm[*forms.SecurityAdvisoryForm](ctx)
	if ctx.Written() {
		return nil
	}
	content := &advisory_service.ContentOptions{
		ContentVersion: form.ContentVersion,
		Summary:        form.Summary,
		Description:    form.Content,
		CveID:          form.CveID,
		CweIDs:         strings.FieldsFunc(form.CweIDs, func(char rune) bool { return char == ',' || char == ' ' }),
	}
	if strings.HasPrefix(form.Severity, "cvss:") {
		content.CvssVector = form.CvssVector
	} else {
		content.Severity = form.Severity
	}
	labelIDs, err := base.StringsToInt64s(strings.Split(form.LabelIDs, ","))
	if err != nil {
		jsonSecurityAdvisoryError(ctx, util.NewInvalidArgumentErrorf("invalid label"))
		return nil
	}
	content.LabelIDs = labelIDs
	for i, ecosystem := range form.Ecosystem {
		vulnerability := &advisory_model.Vulnerability{
			Ecosystem:              ecosystem,
			PackageName:            formValueAt(form.PackageName, i),
			VulnerableVersionRange: formValueAt(form.VulnerableVersionRange, i),
			PatchedVersions:        formValueAt(form.PatchedVersions, i),
			VulnerableFunctions:    util.SplitTrimSpace(formValueAt(form.VulnerableFunctions, i), "\n"),
		}
		if vulnerability.Ecosystem != "" || strings.TrimSpace(vulnerability.PackageName+vulnerability.VulnerableVersionRange+vulnerability.PatchedVersions) != "" || len(vulnerability.VulnerableFunctions) > 0 {
			content.Vulnerabilities = append(content.Vulnerabilities, vulnerability)
		}
	}
	if content.Credits, err = advisory_service.CreditsByLogins(ctx, form.CreditUser, form.CreditType); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return nil
	}
	return content
}

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
		IsNew:          true,
	}, &advisory_service.ContentOptions{})
}

func NewSecurityAdvisoryPost(ctx *context.Context) {
	content := contentFromForm(ctx)
	if ctx.Written() {
		return
	}
	advisory, err := advisory_service.CreateAdvisory(ctx, ctx.Doer, ctx.Repo.Repository, content)
	if err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.JSONRedirect(advisory.Link())
}

func MustAllowPrivateVulnerabilityReporting(ctx *context.Context) {
	if !advisory_service.IsPrivateReportingEnabledForUser(ctx, ctx.Repo.Repository, ctx.Doer) {
		ctx.NotFound()
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
	content := contentFromForm(ctx)
	if ctx.Written() {
		return
	}
	advisory, err := advisory_service.ReportVulnerability(ctx, ctx.Doer, ctx.Repo.Repository, content)
	if err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("repo.security_advisories.report_success"))
	ctx.JSONRedirect(advisory.Link())
}

func SecurityAdvisoryCVSSPreview(ctx *context.Context) {
	result, err := cvss.Parse(ctx.FormString("vector"))
	if err != nil {
		ctx.JSON(http.StatusOK, map[string]any{"error": ctx.Locale.TrString("repo.security_advisories.cvss.invalid")})
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"score":    result.Score,
		"severity": ctx.Locale.TrString(advisory_model.ParseSeverity(result.Rating).LocaleKey()),
	})
}

func LoadSecurityAdvisory(ctx *context.Context) {
	advisory, perms, err := securityAdvisoryViewer(ctx).GetAdvisory(ctx, ctx.Repo.Repository, ctx.PathParam("identifier"))
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	perms.CanEdit = perms.CanEdit && !ctx.Repo.Repository.IsArchived
	advisory_service.SetRequestAdvisory(ctx, advisory, perms)
	ctx.Data["Advisory"] = advisory
	ctx.Data["AdvisoryPerms"] = perms
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
	if _, perms := advisory_service.RequestAdvisory(ctx); !perms.CanEdit {
		ctx.HTTPError(http.StatusForbidden)
	}
}

func MustSeeSecurityAdvisoryDiscussion(ctx *context.Context) {
	if _, perms := advisory_service.RequestAdvisory(ctx); !perms.CanSeeDiscussion {
		ctx.HTTPError(http.StatusForbidden)
	}
}

type securityAdvisoryCollaborator struct {
	User       *user_model.User
	Team       *organization.Team
	ReadOnly   bool
	RemoveLink string
}

func ViewSecurityAdvisory(ctx *context.Context) {
	advisory, perms := advisory_service.RequestAdvisory(ctx)
	if _, err := securityAdvisoryViewer(ctx).LoadForDisplay(ctx, advisory_model.List{advisory}); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Data["Title"] = advisory.Summary
	var err error
	if ctx.Data["RenderedDescription"], err = renderSecurityAdvisoryMarkdown(ctx, "description", advisory.Description); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	if perms.CanSeeDiscussion {
		if advisory.ReportDescription != "" && advisory.ReportDescription != advisory.Description {
			if ctx.Data["RenderedReportDescription"], err = renderSecurityAdvisoryMarkdown(ctx, "report", advisory.ReportDescription); err != nil {
				ctx.ServerErrorAuto(err)
				return
			}
		}
		comments, err := db.Find[advisory_model.Comment](ctx, advisory_model.FindCommentsOptions{AdvisoryID: advisory.ID, IncludeConfidential: perms.CanSeeConfidential})
		if err != nil {
			ctx.ServerErrorAuto(err)
			return
		}
		if err := advisory_model.LoadCommentPosters(ctx, comments); err != nil {
			ctx.ServerErrorAuto(err)
			return
		}
		for _, comment := range comments {
			if comment.RenderedContent, err = renderSecurityAdvisoryMarkdown(ctx, strconv.FormatInt(comment.ID, 10), comment.Content); err != nil {
				ctx.ServerErrorAuto(err)
				return
			}
		}
		ctx.Data["Comments"] = comments

		collaborators := make([]securityAdvisoryCollaborator, 0, len(advisory.CollaboratorUsers)+len(advisory.ReadOnlyUsers)+len(advisory.CollaboratorTeams))
		removeLink := advisory.Link() + "/collaborators/delete"
		for _, user := range advisory.CollaboratorUsers {
			collaborators = append(collaborators, securityAdvisoryCollaborator{User: user, RemoveLink: fmt.Sprintf("%s?user_id=%d", removeLink, user.ID)})
		}
		for _, user := range advisory.ReadOnlyUsers {
			collaborators = append(collaborators, securityAdvisoryCollaborator{User: user, ReadOnly: true, RemoveLink: fmt.Sprintf("%s?user_id=%d", removeLink, user.ID)})
		}
		for _, team := range advisory.CollaboratorTeams {
			collaborators = append(collaborators, securityAdvisoryCollaborator{Team: team, RemoveLink: fmt.Sprintf("%s?team_id=%d", removeLink, team.ID)})
		}
		ctx.Data["Collaborators"] = collaborators
	}
	switch original := advisory.DuplicateOf; {
	case original != nil && original.IsMetadataOnly:
		ctx.Data["StatusText"] = ctx.Tr("repo.security_advisories.closed_as_duplicate_of_metadata", original.Identifier, ctx.Tr("repo.security_advisories.state."+original.State.String()), ctx.Tr(original.Severity.LocaleKey()), templates.TimeSince(original.CreatedUnix))
	case original != nil:
		ctx.Data["StatusText"] = ctx.Tr("repo.security_advisories.closed_as_duplicate_of", original.Link(), original.Identifier)
	case advisory.CloseReason != advisory_model.CloseReasonNone:
		ctx.Data["StatusText"] = ctx.Tr("repo.security_advisories.closed_as." + advisory.CloseReason.String())
	case advisory.Publisher != nil:
		ctx.Data["StatusText"] = ctx.Tr("repo.security_advisories.published_by", templates.TimeSince(advisory.PublishedUnix), advisory.Publisher.HomeLink(), advisory.Publisher.GetDisplayName())
	case perms.CanSeeDiscussion:
		key := util.Iif(advisory.IsReport, "repo.security_advisories.reported_by", "repo.issues.opened_by")
		ctx.Data["StatusText"] = ctx.Tr(key, templates.TimeSince(advisory.CreatedUnix), advisory.Reporter.HomeLink(), advisory.Reporter.GetDisplayName())
	}
	ctx.Data["DescriptionAuthor"] = util.Iif(perms.CanSeeDiscussion, advisory.Reporter, advisory.Publisher)
	if version := advisory.CvssVersion(); version != "" {
		values := map[string]string{}
		for part := range strings.SplitSeq(strings.TrimPrefix(advisory.CvssVector, "CVSS:"+string(version)+"/"), "/") {
			key, value, _ := strings.Cut(part, ":")
			values[key] = value
		}
		var metrics [][2]string
		for _, metric := range cvss.BaseMetrics(version) {
			for _, value := range metric.Values {
				if value.Key == values[metric.Key] {
					metrics = append(metrics, [2]string{metric.Name, value.Name})
				}
			}
		}
		ctx.Data["CVSSMetrics"] = metrics
	}
	ctx.Data["VisibleCredits"] = advisory.VisibleCredits(ctx, ctx.Doer)
	if perms.CanSeeDiscussion {
		labelsData := &issueSidebarLabelsData{}
		labelsData.retrieve(ctx, ctx.Repo.Repository)
		if ctx.Written() {
			return
		}
		labelsData.SetSelectedLabels(advisory.Labels)
		ctx.Data["LabelsData"] = labelsData
	}
	if perms.CanManage && perms.CanEdit {
		switch advisory.State {
		case advisory_model.StateTriage:
			ctx.Data["ActionSection"] = pullMergeBoxSection{SectionTitle: ctx.Tr("repo.security_advisories.action.accept_title"), SectionDetails: []template.HTML{ctx.Tr("repo.security_advisories.action.accept_desc")}}
		case advisory_model.StateDraft:
			ctx.Data["ActionSection"] = pullMergeBoxSection{SectionTitle: ctx.Tr("repo.security_advisories.action.published"), SectionDetails: []template.HTML{ctx.Tr("repo.security_advisories.action.publish_desc")}}
		}
		if advisory.State.CanTransitionTo(advisory_model.StateClosed) {
			candidates, err := advisory_model.FindDuplicateCandidates(ctx, advisory, securityAdvisoryViewer(ctx))
			if err != nil {
				ctx.ServerErrorAuto(err)
				return
			}
			ctx.Data["DuplicateCandidates"] = candidates
			ctx.Data["CanCloseSecurityAdvisory"] = true
		}
	}
	ctx.HTML(http.StatusOK, tplSecurityAdvisoryView)
}

func EditSecurityAdvisory(ctx *context.Context) {
	advisory, _ := advisory_service.RequestAdvisory(ctx)
	if err := advisory.LoadAttributes(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	renderSecurityAdvisoryForm(ctx, securityAdvisoryFormPage{
		TitleKey:       "repo.security_advisories.edit",
		DescriptionKey: "repo.security_advisories.form_desc",
		SubmitKey:      "save",
		CancelLink:     advisory.Link(),
	}, advisory_service.ContentFromAdvisory(advisory))
}

func EditSecurityAdvisoryPost(ctx *context.Context) {
	advisory, perms := advisory_service.RequestAdvisory(ctx)
	content := contentFromForm(ctx)
	if ctx.Written() {
		return
	}
	if err := advisory.LoadAttributes(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	opts := advisory_service.EditOptions{
		Summary:         optional.Some(content.Summary),
		Description:     optional.Some(content.Description),
		Severity:        optional.Some(content.Severity),
		CvssVector:      optional.Some(content.CvssVector),
		CweIDs:          optional.Some(content.CweIDs),
		Vulnerabilities: optional.Some(content.Vulnerabilities),
		ContentVersion:  optional.Some(content.ContentVersion),
	}
	if perms.CanManage {
		opts.CveID = optional.Some(content.CveID)
		opts.Credits = optional.Some(content.Credits)
	}
	if err := advisory_service.EditAdvisory(ctx, ctx.Doer, advisory, perms, opts); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.JSONRedirect(advisory.Link())
}

func UpdateSecurityAdvisoryLabels(ctx *context.Context) {
	advisory, perms := advisory_service.RequestAdvisory(ctx)
	ids, err := base.StringsToInt64s(strings.Split(ctx.FormString("id"), ","))
	if err != nil {
		ctx.JSONErrorAuto(util.NewInvalidArgumentErrorf("invalid label"))
		return
	}
	if err := advisory.LoadAttributes(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	if err := advisory_service.EditAdvisory(ctx, ctx.Doer, advisory, perms, advisory_service.EditOptions{LabelIDs: optional.Some(ids)}); err != nil {
		ctx.JSONErrorAuto(err)
		return
	}
	ctx.JSONOK()
}

func ChangeSecurityAdvisoryState(ctx *context.Context) {
	advisory, _ := advisory_service.RequestAdvisory(ctx)
	closeReason := ctx.FormString("close_reason")
	duplicateOf := util.Iif(closeReason == advisory_model.CloseReasonDuplicate.String(), ctx.FormTrim("duplicate_of"), "")
	if err := advisory_service.ChangeState(ctx, ctx.Doer, advisory, ctx.FormString("state"), closeReason, duplicateOf, duplicateOf != "" && ctx.FormBool("share_original")); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.JSONRedirect(advisory.Link())
}

func DeleteSecurityAdvisory(ctx *context.Context) {
	advisory, _ := advisory_service.RequestAdvisory(ctx)
	if err := advisory_service.DeleteAdvisory(ctx, ctx.Doer, advisory); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.Flash.Success(ctx.Tr("repo.security_advisories.deletion_success"))
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/security/advisories")
}

func AddSecurityAdvisoryCollaborator(ctx *context.Context) {
	advisory, _ := advisory_service.RequestAdvisory(ctx)
	err := advisory_service.AddCollaborator(ctx, ctx.Doer, advisory, ctx.FormTrim("collaborator"), ctx.FormString("type") == "team")
	switch {
	case err == nil:
		ctx.JSONRedirect(advisory.Link())
	case errors.Is(err, util.ErrNotExist):
		ctx.JSONError(ctx.Tr("repo.security_advisories.collaborator.not_exist"))
	case errors.Is(err, user_model.ErrBlockedUser):
		ctx.JSONError(ctx.Tr("repo.security_advisories.collaborator.blocked"))
	case errors.Is(err, util.ErrInvalidArgument):
		ctx.JSONError(ctx.Tr("repo.security_advisories.collaborator.no_access"))
	default:
		ctx.ServerErrorAuto(err)
	}
}

func RemoveSecurityAdvisoryCollaborator(ctx *context.Context) {
	advisory, _ := advisory_service.RequestAdvisory(ctx)
	if err := advisory_service.RemoveCollaborator(ctx, ctx.Doer, advisory, ctx.FormInt64("user_id"), ctx.FormInt64("team_id")); err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.JSONRedirect(advisory.Link())
}

func NewSecurityAdvisoryComment(ctx *context.Context) {
	advisory, perms := advisory_service.RequestAdvisory(ctx)
	form := context.GetFetchActionForm[*forms.SecurityAdvisoryCommentForm](ctx)
	if ctx.Written() {
		return
	}
	comment, err := advisory_service.CreateComment(ctx, ctx.Doer, advisory, perms, form.Content, form.IsConfidential)
	if err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.JSONRedirect(fmt.Sprintf("%s#advisory-comment-%d", advisory.Link(), comment.ID))
}

func getSecurityAdvisoryComment(ctx *context.Context, advisoryID int64, includeConfidential bool) *advisory_model.Comment {
	comment, err := advisory_model.GetCommentByID(ctx, advisoryID, ctx.PathParamInt64("id"), includeConfidential)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return nil
	}
	return comment
}

func EditSecurityAdvisoryComment(ctx *context.Context) {
	advisory, perms := advisory_service.RequestAdvisory(ctx)
	comment := getSecurityAdvisoryComment(ctx, advisory.ID, perms.CanSeeConfidential)
	if comment == nil {
		return
	}
	if err := advisory_service.EditComment(ctx, ctx.Doer, advisory, comment, ctx.FormString("content")); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	rendered, err := renderSecurityAdvisoryMarkdown(ctx, strconv.FormatInt(comment.ID, 10), comment.Content)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{
		"content":        rendered,
		"contentVersion": 0,
		"attachments":    "",
	})
}

func DeleteSecurityAdvisoryComment(ctx *context.Context) {
	advisory, perms := advisory_service.RequestAdvisory(ctx)
	comment := getSecurityAdvisoryComment(ctx, advisory.ID, perms.CanSeeConfidential)
	if comment == nil {
		return
	}
	if err := advisory_service.DeleteComment(ctx, ctx.Doer, perms, comment); err != nil {
		jsonSecurityAdvisoryError(ctx, err)
		return
	}
	ctx.JSONOK()
}
