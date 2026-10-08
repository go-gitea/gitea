// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"strings"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/cvss"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/reqctx"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	advisory_service "gitea.dev/services/advisory"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

func advisoryViewer(ctx *context.APIContext) advisory_model.Viewer {
	return advisory_model.Viewer{Doer: ctx.Doer, IsRepoAdmin: ctx.Repo.Permission.IsAdmin(), PublicOnly: ctx.PublicOnly}
}

type securityAdvisoryContextKey struct{}

type loadedSecurityAdvisory struct {
	advisory *advisory_model.Advisory
	perms    advisory_model.Permissions
}

// LoadSecurityAdvisory responds with 404 if the doer cannot see the advisory, like for advisories that don't exist
func LoadSecurityAdvisory(ctx *context.APIContext) {
	a, perms, err := advisoryViewer(ctx).GetAdvisory(ctx, ctx.Repo.Repository, ctx.PathParam("identifier"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.SetContextValue(securityAdvisoryContextKey{}, &loadedSecurityAdvisory{advisory: a, perms: perms})
}

func getSecurityAdvisory(ctx *context.APIContext) (*advisory_model.Advisory, advisory_model.Permissions) {
	loaded := reqctx.MustContextValue[*loadedSecurityAdvisory](ctx, securityAdvisoryContextKey{})
	return loaded.advisory, loaded.perms
}

func MustManageSecurityAdvisories(ctx *context.APIContext) {
	if !advisoryViewer(ctx).CanManage() {
		ctx.APIError(http.StatusForbidden, "only repository admins can create security advisories")
	}
}

// MustSeeSecurityAdvisoryDiscussion responds with 404 like for advisories the doer cannot see at all
func MustSeeSecurityAdvisoryDiscussion(ctx *context.APIContext) {
	if _, perms := getSecurityAdvisory(ctx); !perms.CanSeeDiscussion {
		ctx.APIErrorNotFound()
	}
}

// apiAdvisoryError responds with 422 for validation errors like GitHub does
func apiAdvisoryError(ctx *context.APIContext, err error) {
	if errors.Is(err, util.ErrInvalidArgument) {
		ctx.APIError(http.StatusUnprocessableEntity, err.Error())
		return
	}
	ctx.APIErrorAuto(err)
}

func toAPIAdvisories(ctx *context.APIContext, list advisory_model.List) ([]*api.RepositoryAdvisory, error) {
	withPrivateDetails, err := advisoryViewer(ctx).LoadForDisplay(ctx, list)
	if err != nil {
		return nil, err
	}
	res := make([]*api.RepositoryAdvisory, 0, len(list))
	for _, a := range list {
		if !withPrivateDetails.Contains(a.ID) {
			res = append(res, convert.ToAPIRepositoryAdvisory(ctx, a, ctx.Doer))
			continue
		}
		apiAdvisory, err := convert.ToAPIRepositoryAdvisoryWithPrivateDetails(ctx, a, ctx.Doer)
		if err != nil {
			return nil, err
		}
		res = append(res, apiAdvisory)
	}
	return res, nil
}

func respondAdvisory(ctx *context.APIContext, a *advisory_model.Advisory, status int) {
	res, err := toAPIAdvisories(ctx, advisory_model.List{a})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(status, res[0])
}

func toAdvisoryVulnerabilities(vulns []*api.RepositoryAdvisoryVulnerability) []*advisory_model.Vulnerability {
	res := make([]*advisory_model.Vulnerability, 0, len(vulns))
	for _, v := range vulns {
		if v == nil {
			continue
		}
		vuln := &advisory_model.Vulnerability{
			VulnerableVersionRange: v.VulnerableVersionRange,
			PatchedVersions:        v.PatchedVersions,
			VulnerableFunctions:    v.VulnerableFunctions,
		}
		if v.Package != nil {
			vuln.Ecosystem, vuln.PackageName = v.Package.Ecosystem, v.Package.Name
		}
		res = append(res, vuln)
	}
	return res
}

func toAdvisoryCredits(ctx *context.APIContext, credits []*api.RepositoryAdvisoryCredit) ([]*advisory_model.Credit, error) {
	logins, types := make([]string, 0, len(credits)), make([]string, 0, len(credits))
	for _, c := range credits {
		if c != nil {
			logins, types = append(logins, c.Login), append(types, c.Type)
		}
	}
	return advisory_service.CreditsByLogins(ctx, logins, types)
}

// cvssVectorOptions sets the vector of its CVSS version, an empty vector removes both versions,
// unsupported vectors are passed on to be rejected by the service
func cvssVectorOptions(vector string) (v3, v4 optional.Option[string]) {
	vector = strings.TrimSpace(vector)
	switch {
	case vector == "":
		return optional.Some(""), optional.Some("")
	case cvss.DetectVersion(vector) == cvss.Version40:
		return optional.None[string](), optional.Some(vector)
	default:
		return optional.Some(vector), optional.None[string]()
	}
}

// optionalList tells an omitted JSON list from an empty one
func optionalList[T any](list []T) optional.Option[[]T] {
	if list == nil {
		return optional.None[[]T]()
	}
	return optional.Some(list)
}

// ListSecurityAdvisories lists the security advisories of a repository
func ListSecurityAdvisories(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/security-advisories repository repoListSecurityAdvisories
	// ---
	// summary: List the security advisories of a repository the user can see
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
	// - name: state
	//   in: query
	//   description: filter by state
	//   type: string
	//   enum: [triage, draft, published, closed, withdrawn]
	// - name: q
	//   in: query
	//   description: search the summary, description, identifier, CVE ID and package names
	//   type: string
	// - name: severity
	//   in: query
	//   description: filter by severity
	//   type: string
	//   enum: [low, medium, high, critical]
	// - name: labels
	//   in: query
	//   description: comma separated list of label names, all must be set
	//   type: string
	// - name: ecosystem
	//   in: query
	//   description: filter by the ecosystem of an affected package
	//   type: string
	// - name: cwe
	//   in: query
	//   description: filter by CWE ID
	//   type: string
	// - name: close_reason
	//   in: query
	//   description: filter closed advisories by the close reason
	//   type: string
	//   enum: [invalid, duplicate]
	// - name: sort
	//   in: query
	//   description: the property to sort by
	//   type: string
	//   enum: [created, updated, published]
	//   default: created
	// - name: direction
	//   in: query
	//   description: the sort direction
	//   type: string
	//   enum: [asc, desc]
	//   default: desc
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
	//     "$ref": "#/responses/RepositoryAdvisoryList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	opts, err := advisory_service.ParseListFilters(advisory_service.ListFilters{
		State:       ctx.FormString("state"),
		Keyword:     ctx.FormTrim("q"),
		Severity:    ctx.FormString("severity"),
		Ecosystem:   ctx.FormTrim("ecosystem"),
		CweID:       ctx.FormTrim("cwe"),
		CloseReason: ctx.FormString("close_reason"),
		SortBy:      ctx.FormString("sort"),
		Direction:   ctx.FormString("direction"),
	})
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	if names := ctx.FormTrim("labels"); names != "" {
		labelIDs, allFound, err := advisory_service.LabelIDsByNames(ctx, ctx.Repo.Repository, strings.Split(names, ","))
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		if !allFound {
			ctx.SetTotalCountHeader(0)
			ctx.JSON(http.StatusOK, []*api.RepositoryAdvisory{})
			return
		}
		opts.LabelIDs = labelIDs
	}
	opts.ListOptions = utils.GetListOptions(ctx)
	opts.RepoID = ctx.Repo.Repository.ID
	opts.Viewer = advisoryViewer(ctx)

	advisories, total, err := db.FindAndCount[advisory_model.Advisory](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	for _, a := range advisories {
		a.Repo = ctx.Repo.Repository
	}
	res, err := toAPIAdvisories(ctx, advisories)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.SetLinkHeader(total, opts.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, res)
}

// GetSecurityAdvisory gets a security advisory of a repository
func GetSecurityAdvisory(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/security-advisories/{identifier} repository repoGetSecurityAdvisory
	// ---
	// summary: Get a security advisory of a repository
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
	// - name: identifier
	//   in: path
	//   description: identifier of the advisory
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryAdvisory"
	//   "404":
	//     "$ref": "#/responses/notFound"
	a, _ := getSecurityAdvisory(ctx)
	respondAdvisory(ctx, a, http.StatusOK)
}

// CreateSecurityAdvisory creates a draft security advisory
func CreateSecurityAdvisory(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/security-advisories repository repoCreateSecurityAdvisory
	// ---
	// summary: Create a draft security advisory, requires repository admin permission
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
	//     "$ref": "#/definitions/CreateRepositoryAdvisoryOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/RepositoryAdvisory"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	form := web.GetForm[*api.CreateRepositoryAdvisoryOption](ctx)
	credits, err := toAdvisoryCredits(ctx, form.Credits)
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	v3, v4 := cvssVectorOptions(form.CVSSVectorString)
	a, err := advisory_service.CreateAdvisory(ctx, ctx.Doer, ctx.Repo.Repository, &advisory_service.ContentOptions{
		Summary:         form.Summary,
		Description:     form.Description,
		CveID:           form.CveID,
		Severity:        form.Severity,
		CvssV3Vector:    v3.Value(),
		CvssV4Vector:    v4.Value(),
		CweIDs:          form.CweIDs,
		Vulnerabilities: toAdvisoryVulnerabilities(form.Vulnerabilities),
		Credits:         credits,
		LabelIDs:        form.Labels,
	})
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	respondAdvisory(ctx, a, http.StatusCreated)
}

// CreatePrivateVulnerabilityReport reports a vulnerability privately
func CreatePrivateVulnerabilityReport(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/security-advisories/reports repository repoCreatePrivateVulnerabilityReport
	// ---
	// summary: Report a vulnerability privately to the maintainers of a repository
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
	//     "$ref": "#/definitions/CreatePrivateVulnerabilityReportOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/RepositoryAdvisory"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	form := web.GetForm[*api.CreatePrivateVulnerabilityReportOption](ctx)
	v3, v4 := cvssVectorOptions(form.CVSSVectorString)
	a, err := advisory_service.ReportVulnerability(ctx, ctx.Doer, ctx.Repo.Repository, &advisory_service.ContentOptions{
		Summary:         form.Summary,
		Description:     form.Description,
		Severity:        form.Severity,
		CvssV3Vector:    v3.Value(),
		CvssV4Vector:    v4.Value(),
		CweIDs:          form.CweIDs,
		Vulnerabilities: toAdvisoryVulnerabilities(form.Vulnerabilities),
	})
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	respondAdvisory(ctx, a, http.StatusCreated)
}

// EditSecurityAdvisory updates a security advisory
func EditSecurityAdvisory(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/security-advisories/{identifier} repository repoEditSecurityAdvisory
	// ---
	// summary: Update a security advisory
	// description: Repository admins can change everything. The reporter and collaborators can only change the content of non-published advisories.
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
	// - name: identifier
	//   in: path
	//   description: identifier of the advisory
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditRepositoryAdvisoryOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryAdvisory"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	form := web.GetForm[*api.EditRepositoryAdvisoryOption](ctx)
	a, perms := getSecurityAdvisory(ctx)
	if err := a.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	opts := advisory_service.EditOptions{
		Summary:           optional.FromPtr(form.Summary),
		Description:       optional.FromPtr(form.Description),
		Severity:          optional.FromPtr(form.Severity),
		CweIDs:            optionalList(form.CweIDs),
		CveID:             optional.FromPtr(form.CveID),
		LabelIDs:          optionalList(form.Labels),
		CollaboratorUsers: optionalList(form.CollaboratingUsers),
		CollaboratorTeams: optionalList(form.CollaboratingTeams),
		State:             optional.FromPtr(form.State),
		CloseReason:       optional.FromPtr(form.CloseReason),
		DuplicateOf:       optional.FromPtr(form.DuplicateOf),
	}
	if form.CVSSVectorString != nil {
		opts.CvssV3Vector, opts.CvssV4Vector = cvssVectorOptions(*form.CVSSVectorString)
	}
	if form.Vulnerabilities != nil {
		opts.Vulnerabilities = optional.Some(toAdvisoryVulnerabilities(form.Vulnerabilities))
	}
	if form.Credits != nil {
		credits, err := toAdvisoryCredits(ctx, form.Credits)
		if err != nil {
			apiAdvisoryError(ctx, err)
			return
		}
		opts.Credits = optional.Some(credits)
	}
	if err := advisory_service.EditAdvisory(ctx, ctx.Doer, a, perms, opts); err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	respondAdvisory(ctx, a, http.StatusOK)
}

// GetPrivateVulnerabilityReporting checks whether private vulnerability reporting is enabled
func GetPrivateVulnerabilityReporting(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/private-vulnerability-reporting repository repoGetPrivateVulnerabilityReporting
	// ---
	// summary: Check whether private vulnerability reporting is enabled for a repository
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
	// responses:
	//   "200":
	//     "$ref": "#/responses/PrivateVulnerabilityReporting"
	//   "404":
	//     "$ref": "#/responses/notFound"
	ctx.JSON(http.StatusOK, &api.PrivateVulnerabilityReporting{Enabled: advisory_service.IsPrivateReportingEnabled(ctx, ctx.Repo.Repository)})
}

// EnablePrivateVulnerabilityReporting enables private vulnerability reporting
func EnablePrivateVulnerabilityReporting(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/private-vulnerability-reporting repository repoEnablePrivateVulnerabilityReporting
	// ---
	// summary: Enable private vulnerability reporting for a repository
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
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	setPrivateVulnerabilityReporting(ctx, true)
}

// DisablePrivateVulnerabilityReporting disables private vulnerability reporting
func DisablePrivateVulnerabilityReporting(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/private-vulnerability-reporting repository repoDisablePrivateVulnerabilityReporting
	// ---
	// summary: Disable private vulnerability reporting for a repository
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
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	setPrivateVulnerabilityReporting(ctx, false)
}

func setPrivateVulnerabilityReporting(ctx *context.APIContext, enabled bool) {
	if err := advisory_service.SetPrivateReporting(ctx, ctx.Doer, ctx.Repo.Repository, enabled); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func getAdvisoryComment(ctx *context.APIContext) *advisory_model.Comment {
	a, _ := getSecurityAdvisory(ctx)
	c, err := advisory_model.GetCommentByID(ctx, a.ID, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return nil
	}
	if _, c.Poster, err = user_model.GetPossibleUserByID(ctx, c.PosterID); err != nil {
		ctx.APIErrorAuto(err)
		return nil
	}
	return c
}

// ListSecurityAdvisoryComments lists the comments of the private discussion of an advisory
func ListSecurityAdvisoryComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/security-advisories/{identifier}/comments repository repoListSecurityAdvisoryComments
	// ---
	// summary: List the comments of the private discussion of a security advisory
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
	// - name: identifier
	//   in: path
	//   description: identifier of the advisory
	//   type: string
	//   required: true
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
	//     "$ref": "#/responses/RepositoryAdvisoryCommentList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	a, _ := getSecurityAdvisory(ctx)
	listOptions := utils.GetListOptions(ctx)
	comments, total, err := db.FindAndCount[advisory_model.Comment](ctx, advisory_model.FindCommentsOptions{ListOptions: listOptions, AdvisoryID: a.ID})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if err := advisory_model.CommentList(comments).LoadPosters(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	res := make([]*api.RepositoryAdvisoryComment, 0, len(comments))
	for _, c := range comments {
		res = append(res, convert.ToAPIRepositoryAdvisoryComment(ctx, a, c, ctx.Doer))
	}
	ctx.SetLinkHeader(total, listOptions.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, res)
}

// CreateSecurityAdvisoryComment adds a comment to the private discussion of an advisory
func CreateSecurityAdvisoryComment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/security-advisories/{identifier}/comments repository repoCreateSecurityAdvisoryComment
	// ---
	// summary: Add a comment to the private discussion of a security advisory
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
	// - name: identifier
	//   in: path
	//   description: identifier of the advisory
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/RepositoryAdvisoryCommentOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/RepositoryAdvisoryComment"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	form := web.GetForm[*api.RepositoryAdvisoryCommentOption](ctx)
	a, _ := getSecurityAdvisory(ctx)
	c, err := advisory_service.CreateComment(ctx, ctx.Doer, a, form.Body)
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToAPIRepositoryAdvisoryComment(ctx, a, c, ctx.Doer))
}

// EditSecurityAdvisoryComment edits a comment of the private discussion of an advisory
func EditSecurityAdvisoryComment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/security-advisories/{identifier}/comments/{id} repository repoEditSecurityAdvisoryComment
	// ---
	// summary: Edit an own comment of the private discussion of a security advisory
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
	// - name: identifier
	//   in: path
	//   description: identifier of the advisory
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the comment
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/RepositoryAdvisoryCommentOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryAdvisoryComment"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	form := web.GetForm[*api.RepositoryAdvisoryCommentOption](ctx)
	a, _ := getSecurityAdvisory(ctx)
	c := getAdvisoryComment(ctx)
	if ctx.Written() {
		return
	}
	if err := advisory_service.EditComment(ctx, ctx.Doer, a, c, form.Body); err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIRepositoryAdvisoryComment(ctx, a, c, ctx.Doer))
}

// DeleteSecurityAdvisoryComment deletes a comment of the private discussion of an advisory
func DeleteSecurityAdvisoryComment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/security-advisories/{identifier}/comments/{id} repository repoDeleteSecurityAdvisoryComment
	// ---
	// summary: Delete a comment of the private discussion of a security advisory, by its poster or a repository admin
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
	// - name: identifier
	//   in: path
	//   description: identifier of the advisory
	//   type: string
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the comment
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	_, perms := getSecurityAdvisory(ctx)
	c := getAdvisoryComment(ctx)
	if ctx.Written() {
		return
	}
	if err := advisory_service.DeleteComment(ctx, ctx.Doer, perms, c); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
