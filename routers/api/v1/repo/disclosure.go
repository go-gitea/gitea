// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"strings"

	disclosure_model "gitea.dev/models/disclosure"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
	disclosure_service "gitea.dev/services/disclosure"
)

func parseListDisclosuresOptions(ctx *context.APIContext) disclosure_model.ListDisclosuresOptions {
	opts := disclosure_model.ListDisclosuresOptions{
		ListOptions: utils.GetListOptions(ctx),
		Keyword:     ctx.FormTrim("q"),
		Version:     ctx.FormTrim("version"),
		OrderBy:     ctx.FormTrim("sort"),
	}
	for s := range strings.SplitSeq(ctx.FormTrim("severity"), ",") {
		if severity, err := disclosure_model.ParseSeverity(strings.TrimSpace(s)); err == nil {
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

// canSeeDraftDisclosures reports whether the current user may see draft disclosures (maintainers only)
func canSeeDraftDisclosures(ctx *context.APIContext) bool {
	return ctx.IsSigned && ctx.Repo.Permission.IsAdmin()
}

// ListDisclosures lists the security disclosures of a repository
func ListDisclosures(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/security/disclosures repository repoListSecurityDisclosures
	// ---
	// summary: List a repository's security disclosures
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: q
	//   in: query
	//   description: keyword to search in title, summary and description
	//   type: string
	// - name: severity
	//   in: query
	//   description: comma-separated list of severities to filter by (informative, low, moderate, high, critical)
	//   type: string
	// - name: status
	//   in: query
	//   description: filter by status (draft, published, withdrawn); drafts are only visible to maintainers
	//   type: string
	// - name: version
	//   in: query
	//   description: only include disclosures whose affected versions match this version (semver or exact)
	//   type: string
	// - name: sort
	//   in: query
	//   description: sort order (newest, oldest, severity, updated)
	//   type: string
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/SecurityDisclosureList"
	//   default:
	//     "$ref": "#/responses/error"

	opts := parseListDisclosuresOptions(ctx)
	opts.RepoID = ctx.Repo.Repository.ID
	opts.IncludeDrafts = canSeeDraftDisclosures(ctx) // drafts are only visible to maintainers
	list, err := disclosure_model.ListDisclosures(ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	count, err := disclosure_model.CountDisclosures(ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	result := make([]*structs.SecurityDisclosure, len(list))
	for i, d := range list {
		result[i] = convert.ToSecurityDisclosure(ctx, ctx.Repo.Repository, d)
	}
	ctx.SetLinkHeader(count, opts.PageSize)
	ctx.SetTotalCountHeader(count)
	ctx.JSON(http.StatusOK, result)
}

// GetDisclosure gets a single security disclosure of a repository
func GetDisclosure(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/security/disclosures/{id} repository repoGetSecurityDisclosure
	// ---
	// summary: Get a single security disclosure
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the disclosure to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/SecurityDisclosure"
	//   default:
	//     "$ref": "#/responses/error"

	d, ok := getAPIDisclosure(ctx)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, convert.ToSecurityDisclosure(ctx, ctx.Repo.Repository, d))
}

// CreateDisclosure creates a new security disclosure (maintainers only)
func CreateDisclosure(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/security/disclosures repository repoCreateSecurityDisclosure
	// ---
	// summary: Create a new security disclosure (maintainers only)
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateSecurityDisclosureOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/SecurityDisclosure"
	//   default:
	//     "$ref": "#/responses/error"

	form := web.GetForm[*structs.CreateSecurityDisclosureOption](ctx)
	input := &disclosure_service.Input{
		Title:            form.Title,
		Summary:          form.Summary,
		Content:          form.Description,
		Severity:         disclosure_model.Severity(string(form.Severity)),
		CVSSVector:       form.CVSSVector,
		CweIds:           form.CweIds,
		CveID:            form.CveID,
		References:       form.References,
		AffectedVersions: form.AffectedVersions,
		FixedVersions:    form.FixedVersions,
	}
	if form.Publish {
		input.Status = disclosure_model.StatusPublished
	}
	d, err := disclosure_service.CreateDisclosure(ctx, ctx.Repo.Repository, ctx.Doer, input)
	if err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err.Error())
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToSecurityDisclosure(ctx, ctx.Repo.Repository, d))
}

// EditDisclosure edits a security disclosure (maintainers only)
func EditDisclosure(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/security/disclosures/{id} repository repoEditSecurityDisclosure
	// ---
	// summary: Edit a security disclosure (maintainers only)
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the disclosure to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditSecurityDisclosureOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/SecurityDisclosure"
	//   default:
	//     "$ref": "#/responses/error"

	d, ok := getAPIDisclosure(ctx)
	if !ok {
		return
	}
	form := web.GetForm[*structs.EditSecurityDisclosureOption](ctx)
	input := &disclosure_service.Input{
		Title:            util.IfZero(form.Title, d.Title),
		Summary:          util.IfZero(form.Summary, d.Summary),
		Content:          util.IfZero(form.Description, d.Content),
		Severity:         disclosure_model.Severity(string(util.IfZero(form.Severity, structs.SecurityDisclosureSeverity(d.Severity)))),
		CVSSVector:       util.IfZero(form.CVSSVector, d.CVSSVector),
		CweIds:           orKeep(form.CweIds, d.CweIds),
		CveID:            util.IfZero(form.CveID, d.CveID),
		References:       orKeep(form.References, d.References),
		AffectedVersions: orKeep(form.AffectedVersions, d.AffectedVersions()),
		FixedVersions:    orKeep(form.FixedVersions, d.FixedVersions()),
		Status:           disclosure_model.Status(util.IfZero(form.Status, string(d.Status))),
		WithdrawReason:   util.IfZero(form.WithdrawReason, d.WithdrawReason),
	}
	if err := disclosure_service.UpdateDisclosure(ctx, d, ctx.Doer, input); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err.Error())
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	if err := d.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToSecurityDisclosure(ctx, ctx.Repo.Repository, d))
}

// DeleteDisclosure deletes a security disclosure (maintainers only)
func DeleteDisclosure(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/security/disclosures/{id} repository repoDeleteSecurityDisclosure
	// ---
	// summary: Delete a security disclosure (maintainers only, drafts only)
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the disclosure to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   default:
	//     "$ref": "#/responses/error"

	d, ok := getAPIDisclosure(ctx)
	if !ok {
		return
	}
	if err := disclosure_service.DeleteDisclosure(ctx, ctx.Repo.Repository.ID, d.ID); err != nil {
		if errors.Is(err, util.ErrInvalidArgument) {
			ctx.APIError(http.StatusBadRequest, err.Error())
			return
		}
		ctx.APIErrorInternal(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// orKeep returns the new value if it is not nil, otherwise the current one (so [] clears, null keeps)
func orKeep[T any](got, cur []T) []T {
	if got == nil {
		return cur
	}
	return got
}

// getAPIDisclosure loads the disclosure by path ID, hiding drafts from non-maintainers
func getAPIDisclosure(ctx *context.APIContext) (*disclosure_model.Disclosure, bool) {
	d, err := disclosure_model.GetDisclosureByRepoAndID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		if disclosure_model.IsErrDisclosureNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil, false
	}
	if d.IsDraft() && !canSeeDraftDisclosures(ctx) {
		ctx.APIErrorNotFound()
		return nil, false
	}
	if err := d.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return nil, false
	}
	return d, true
}
