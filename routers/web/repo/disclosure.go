// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"gitea.dev/models/db"
	disclosure_model "gitea.dev/models/disclosure"
	"gitea.dev/models/renderhelper"
	"gitea.dev/modules/markup/markdown"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	disclosure_service "gitea.dev/services/disclosure"
	"gitea.dev/services/forms"
)

const (
	tplSecurityHome   templates.TplName = "repo/security/home"
	tplDisclosureList templates.TplName = "repo/security/disclosures/list"
	tplDisclosureView templates.TplName = "repo/security/disclosures/view"
	tplDisclosureNew  templates.TplName = "repo/security/disclosures/new"
	tplDisclosureEdit templates.TplName = "repo/security/disclosures/edit"
)

// AllSeverities lists the severities for the form dropdown
var AllSeverities = []disclosure_model.Severity{
	disclosure_model.SeverityCritical,
	disclosure_model.SeverityHigh,
	disclosure_model.SeverityModerate,
	disclosure_model.SeverityLow,
	disclosure_model.SeverityInformative,
}

func prepareDisclosurePage(ctx *context.Context) {
	ctx.Data["PageIsSecurity"] = true
	ctx.Data["Title"] = ctx.Tr("repo.security.disclosures")
	ctx.Data["AllSeverities"] = AllSeverities
	// only maintainers may manage disclosures
	ctx.Data["CanManageDisclosures"] = ctx.Repo.Permission.IsAdmin() && !ctx.Repo.Repository.IsArchived
}

// prepareDisclosureFormData sets the shared data for the new/edit form template
func prepareDisclosureFormData(ctx *context.Context, isNew bool, action, cancelLink, submitText string) {
	ctx.Data["IsNew"] = isNew
	ctx.Data["FormAction"] = action
	ctx.Data["CancelLink"] = cancelLink
	ctx.Data["SubmitText"] = submitText
}

// SecurityHome renders the landing page of the repository security section
func SecurityHome(ctx *context.Context) {
	ctx.Data["PageIsSecurity"] = true
	ctx.Data["Title"] = ctx.Tr("repo.security")
	ctx.HTML(http.StatusOK, tplSecurityHome)
}

func disclosureListOptions(ctx *context.Context) disclosure_model.ListDisclosuresOptions {
	listOptions := db.ListOptions{
		Page:     ctx.FormInt("page"),
		PageSize: ctx.FormInt("limit"),
	}
	if listOptions.PageSize == 0 {
		listOptions.PageSize = setting.API.DefaultPagingNum
	}
	if listOptions.PageSize > setting.API.MaxResponseItems {
		listOptions.PageSize = setting.API.MaxResponseItems
	}
	opts := disclosure_model.ListDisclosuresOptions{
		ListOptions: listOptions,
		Keyword:     ctx.FormTrim("q"),
		Version:     ctx.FormTrim("version"),
		OrderBy:     ctx.FormTrim("sort"),
	}
	for _, s := range ctx.FormStrings("severity") {
		if severity, err := disclosure_model.ParseSeverity(s); err == nil {
			opts.Severities = append(opts.Severities, severity)
		}
	}
	if s := ctx.FormTrim("status"); s != "" {
		if status, err := disclosure_model.ParseStatus(s); err == nil {
			opts.Status = status
		}
	}
	return opts
}

// Disclosures renders the list of security disclosures of the repo
func Disclosures(ctx *context.Context) {
	prepareDisclosurePage(ctx)
	ctx.Data["PageIsSecurityDisclosures"] = true

	opts := disclosureListOptions(ctx)
	// drafts are only visible to maintainers
	list, total, err := disclosure_service.ListDisclosures(ctx, ctx.Repo.Repository, ctx.Repo.Permission.IsAdmin(), opts)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}

	severityFilterSet := make(map[string]bool, len(opts.Severities))
	for _, s := range opts.Severities {
		severityFilterSet[string(s)] = true
	}
	ctx.Data["Disclosures"] = list
	ctx.Data["TotalCount"] = total
	ctx.Data["Query"] = opts.Keyword
	ctx.Data["VersionFilter"] = opts.Version
	ctx.Data["StatusFilter"] = opts.Status
	ctx.Data["SeverityFilterSet"] = severityFilterSet
	ctx.Data["SortOrder"] = opts.OrderBy
	pager := context.NewPagerBuilder(ctx).TotalCount(total).PerPageLimit(opts.PageSize).CurPage(opts.Page).Build()
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplDisclosureList)
}

// getVisibleDisclosure loads a disclosure by ID, hiding drafts from non-maintainers
func getVisibleDisclosure(ctx *context.Context) *disclosure_model.Disclosure {
	d, err := disclosure_model.GetDisclosureByRepoAndID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if disclosure_model.IsErrDisclosureNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.ServerErrorAuto(err)
		}
		return nil
	}
	if d.IsDraft() && !ctx.Repo.Permission.IsAdmin() {
		ctx.NotFound()
		return nil
	}
	if err := d.LoadAttributes(ctx); err != nil {
		ctx.ServerErrorAuto(err)
		return nil
	}
	return d
}

// ViewDisclosure renders a single disclosure
func ViewDisclosure(ctx *context.Context) {
	prepareDisclosurePage(ctx)

	d := getVisibleDisclosure(ctx)
	if d == nil {
		return
	}
	rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{
		FootnoteContextID: "disclosure-" + strconv.FormatInt(d.ID, 10),
	})
	content, err := markdown.RenderString(rctx, d.Content)
	if err != nil {
		ctx.ServerErrorAuto(err)
		return
	}
	d.RenderedContent = content
	ctx.Data["Disclosure"] = d
	ctx.Data["Title"] = d.Title
	ctx.HTML(http.StatusOK, tplDisclosureView)
}

func disclosureFormToInput(formTitle, formSummary, formContent, formSeverity, formCVSS, formCweIds, formCveID, formReferences, formAffected, formFixed string) *disclosure_service.Input {
	return &disclosure_service.Input{
		Title:            formTitle,
		Summary:          formSummary,
		Content:          formContent,
		Severity:         disclosure_model.Severity(formSeverity),
		CVSSVector:       formCVSS,
		CweIds:           disclosure_service.SplitCommaSeparated(formCweIds),
		CveID:            formCveID,
		References:       disclosure_service.SplitLines(formReferences),
		AffectedVersions: disclosure_service.SplitLines(formAffected),
		FixedVersions:    disclosure_service.SplitLines(formFixed),
	}
}

// NewDisclosure renders the form for a new disclosure
func NewDisclosure(ctx *context.Context) {
	prepareDisclosurePage(ctx)
	ctx.Data["PageIsSecurityNew"] = true
	ctx.Data["Title"] = ctx.Tr("repo.security.new_disclosure")
	prepareDisclosureFormData(ctx, true, ctx.Repo.RepoLink+"/security/disclosures/new", ctx.Repo.RepoLink+"/security/disclosures", ctx.Locale.TrString("repo.security.create"))
	ctx.HTML(http.StatusOK, tplDisclosureNew)
}

// NewDisclosurePost handles the creation of a disclosure
func NewDisclosurePost(ctx *context.Context) {
	prepareDisclosurePage(ctx)
	ctx.Data["PageIsSecurityNew"] = true
	ctx.Data["Title"] = ctx.Tr("repo.security.new_disclosure")
	prepareDisclosureFormData(ctx, true, ctx.Repo.RepoLink+"/security/disclosures/new", ctx.Repo.RepoLink+"/security/disclosures", ctx.Locale.TrString("repo.security.create"))

	form := web.GetForm[*forms.NewDisclosureForm](ctx)
	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplDisclosureNew)
		return
	}
	input := disclosureFormToInput(form.Title, form.Summary, form.Content, form.Severity, form.CVSSVector, form.CweIds, form.CveID, form.References, form.AffectedVersions, form.FixedVersions)
	if form.Publish {
		input.Status = disclosure_model.StatusPublished
	}
	d, err := disclosure_service.CreateDisclosure(ctx, ctx.Repo.Repository, ctx.Doer, input)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Flash.Error(err.Error(), true)
			ctx.HTML(http.StatusOK, tplDisclosureNew)
			return
		}
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Flash.Success(ctx.Tr("repo.security.create_success", d.Identifier()))
	ctx.Redirect(ctx.Repo.RepoLink + "/security/disclosures/" + strconv.FormatInt(d.ID, 10))
}

// EditDisclosure renders the edit form of a disclosure
func EditDisclosure(ctx *context.Context) {
	prepareDisclosurePage(ctx)
	ctx.Data["PageIsSecurityEdit"] = true

	d := getVisibleDisclosure(ctx)
	if d == nil {
		return
	}
	ctx.Data["Disclosure"] = d
	ctx.Data["Title"] = ctx.Tr("repo.security.edit_disclosure", d.Identifier())
	link := ctx.Repo.RepoLink + "/security/disclosures/" + strconv.FormatInt(d.ID, 10)
	prepareDisclosureFormData(ctx, false, link+"/edit", link, ctx.Locale.TrString("repo.security.save"))
	// prefill the form fields, POST binding overwrites them on error re-render
	ctx.Data["title"] = d.Title
	ctx.Data["summary"] = d.Summary
	ctx.Data["content"] = d.Content
	ctx.Data["severity"] = string(d.Severity)
	ctx.Data["cvss_vector"] = d.CVSSVector
	ctx.Data["cwe_ids"] = strings.Join(d.CweIds, ", ")
	ctx.Data["cve_id"] = d.CveID
	ctx.Data["references"] = strings.Join(d.References, "\n")
	ctx.Data["affected_versions"] = strings.Join(d.AffectedVersions(), "\n")
	ctx.Data["fixed_versions"] = strings.Join(d.FixedVersions(), "\n")
	ctx.Data["status"] = string(d.Status)
	ctx.HTML(http.StatusOK, tplDisclosureEdit)
}

// EditDisclosurePost handles saving edits of a disclosure
func EditDisclosurePost(ctx *context.Context) {
	prepareDisclosurePage(ctx)
	ctx.Data["PageIsSecurityEdit"] = true

	d := getVisibleDisclosure(ctx)
	if d == nil {
		return
	}
	ctx.Data["Disclosure"] = d
	ctx.Data["Title"] = ctx.Tr("repo.security.edit_disclosure", d.Identifier())
	link := ctx.Repo.RepoLink + "/security/disclosures/" + strconv.FormatInt(d.ID, 10)
	prepareDisclosureFormData(ctx, false, link+"/edit", link, ctx.Locale.TrString("repo.security.save"))

	form := web.GetForm[*forms.EditDisclosureForm](ctx)
	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplDisclosureEdit)
		return
	}
	input := disclosureFormToInput(form.Title, form.Summary, form.Content, form.Severity, form.CVSSVector, form.CweIds, form.CveID, form.References, form.AffectedVersions, form.FixedVersions)
	input.Status = disclosure_model.Status(form.Status)
	if err := disclosure_service.UpdateDisclosure(ctx, d, ctx.Doer, input); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Flash.Error(err.Error(), true)
			ctx.HTML(http.StatusOK, tplDisclosureEdit)
			return
		}
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Flash.Success(ctx.Tr("repo.security.edit_success", d.Identifier()))
	ctx.Redirect(ctx.Repo.RepoLink + "/security/disclosures/" + strconv.FormatInt(d.ID, 10))
}

// ChangeDisclosureStatusPost publishes or withdraws a disclosure
func ChangeDisclosureStatusPost(ctx *context.Context) {
	d := getVisibleDisclosure(ctx)
	if d == nil {
		return
	}
	var status disclosure_model.Status
	switch ctx.PathParam("action") {
	case "publish":
		status = disclosure_model.StatusPublished
	case "withdraw":
		status = disclosure_model.StatusWithdrawn
	default:
		ctx.NotFound()
		return
	}
	input := &disclosure_service.Input{
		Title:            d.Title,
		Summary:          d.Summary,
		Content:          d.Content,
		Severity:         d.Severity,
		CVSSVector:       d.CVSSVector,
		CweIds:           d.CweIds,
		CveID:            d.CveID,
		References:       d.References,
		AffectedVersions: d.AffectedVersions(),
		FixedVersions:    d.FixedVersions(),
		Status:           status,
		WithdrawReason:   ctx.FormTrim("reason"),
	}
	if err := disclosure_service.UpdateDisclosure(ctx, d, ctx.Doer, input); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Flash.Error(err.Error(), true)
			ctx.Redirect(ctx.Repo.RepoLink + "/security/disclosures/" + strconv.FormatInt(d.ID, 10))
			return
		}
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/security/disclosures/" + strconv.FormatInt(d.ID, 10))
}

// DeleteDisclosurePost deletes a disclosure
func DeleteDisclosurePost(ctx *context.Context) {
	d := getVisibleDisclosure(ctx)
	if d == nil {
		return
	}
	if err := disclosure_service.DeleteDisclosure(ctx, ctx.Repo.Repository.ID, d.ID); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.Flash.Error(err.Error(), true)
			ctx.Redirect(ctx.Repo.RepoLink + "/security/disclosures/" + strconv.FormatInt(d.ID, 10))
			return
		}
		ctx.ServerErrorAuto(err)
		return
	}
	ctx.Flash.Success(ctx.Tr("repo.security.delete_success", d.Identifier()))
	ctx.Redirect(ctx.Repo.RepoLink + "/security/disclosures")
}
