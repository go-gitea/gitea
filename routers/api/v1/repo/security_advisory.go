// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"strings"
	"time"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/optional"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	advisory_service "gitea.dev/services/advisory"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

func advisoryViewer(ctx *context.APIContext) advisory_model.Viewer {
	return advisory_service.RequestViewer(ctx, ctx.Doer, ctx.Repo.Repository, ctx.Repo.Permission, user_model.IsPublicOnlyDoer(ctx.Doer))
}

func LoadSecurityAdvisory(ctx *context.APIContext) {
	advisory, perms, err := advisoryViewer(ctx).GetAdvisory(ctx, ctx.Repo.Repository, ctx.PathParam("gtsa_id"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	advisory_service.SetRequestAdvisory(ctx, advisory, perms)
}

// MustSeeSecurityAdvisoryDiscussion responds with 404 like for advisories the doer cannot see at all
func MustSeeSecurityAdvisoryDiscussion(ctx *context.APIContext) {
	if _, perms := advisory_service.RequestAdvisory(ctx); !perms.CanSeeDiscussion {
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

func toAPIAdvisories(ctx *context.APIContext, list advisory_model.List, viewer advisory_model.Viewer) ([]*api.RepositoryAdvisory, error) {
	withPrivateDetails, err := viewer.LoadForDisplay(ctx, list)
	if err != nil {
		return nil, err
	}
	result := make([]*api.RepositoryAdvisory, 0, len(list))
	for _, advisory := range list {
		converted := convert.ToAPIRepositoryAdvisory(ctx, advisory, viewer.Doer)
		if withPrivateDetails.Contains(advisory.ID) {
			if err := convert.AddAPIRepositoryAdvisoryPrivateDetails(ctx, converted, advisory, viewer.Doer, viewer.CanManage()); err != nil {
				return nil, err
			}
		}
		result = append(result, converted)
	}
	return result, nil
}

func respondAdvisory(ctx *context.APIContext, advisory *advisory_model.Advisory, status int) {
	result, err := toAPIAdvisories(ctx, advisory_model.List{advisory}, advisoryViewer(ctx))
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(status, result[0])
}

// optionalList tells an omitted JSON list from an empty one
func optionalList[T any](list []T) optional.Option[[]T] {
	return util.Iif(list == nil, optional.None[[]T](), optional.Some(list))
}

func toAdvisoryVulnerabilities(vulnerabilities []*api.RepositoryAdvisoryVulnerability) []*advisory_model.Vulnerability {
	if vulnerabilities == nil {
		return nil
	}
	result := make([]*advisory_model.Vulnerability, 0, len(vulnerabilities))
	for _, vulnerability := range vulnerabilities {
		if vulnerability == nil {
			continue
		}
		converted := &advisory_model.Vulnerability{
			VulnerableVersionRange: optional.FromPtr(vulnerability.VulnerableVersionRange).Value(),
			PatchedVersions:        optional.FromPtr(vulnerability.PatchedVersions).Value(),
			VulnerableFunctions:    vulnerability.VulnerableFunctions,
		}
		if vulnerability.Package != nil {
			converted.Ecosystem, converted.PackageName = vulnerability.Package.Ecosystem, optional.FromPtr(vulnerability.Package.Name).Value()
		}
		result = append(result, converted)
	}
	return result
}

func toAdvisoryCredits(ctx *context.APIContext, credits []*api.RepositoryAdvisoryCredit) ([]*advisory_model.Credit, error) {
	if credits == nil {
		return nil, nil
	}
	logins, types := make([]string, 0, len(credits)), make([]string, 0, len(credits))
	for _, credit := range credits {
		if credit != nil {
			logins, types = append(logins, credit.Login), append(types, credit.Type)
		}
	}
	return advisory_service.CreditsByLogins(ctx, logins, types)
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
	//   description: only advisories in this state (triage, draft, published, closed, withdrawn)
	//   type: string
	// - name: q
	//   in: query
	//   description: search in the summary, description, identifier, CVE ID and package names
	//   type: string
	// - name: severity
	//   in: query
	//   description: only advisories with this severity (low, medium, high, critical)
	//   type: string
	// - name: ecosystem
	//   in: query
	//   description: only advisories affecting a package of this ecosystem
	//   type: string
	// - name: cwe
	//   in: query
	//   description: only advisories with this CWE ID
	//   type: string
	// - name: close_reason
	//   in: query
	//   description: only advisories closed for this reason (invalid, duplicate)
	//   type: string
	// - name: labels
	//   in: query
	//   description: comma separated list of label names, only applied to advisories whose private details the user can see
	//   type: string
	// - name: sort
	//   in: query
	//   description: sort by created (default), updated or published
	//   type: string
	// - name: direction
	//   in: query
	//   description: sort direction, asc or desc (default)
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
	//     "$ref": "#/responses/RepositoryAdvisoryList"
	//   default:
	//     "$ref": "#/responses/error"
	ListAdvisories(ctx, ctx.Repo.Repository, 0, advisoryViewer(ctx))
}

func ListAdvisories(ctx *context.APIContext, repository *repo_model.Repository, orgID int64, viewer advisory_model.Viewer) {
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
	opts.ListOptions = utils.GetListOptions(ctx)
	opts.OrgID = orgID
	opts.Viewer = viewer
	if repository != nil {
		opts.RepoID = repository.ID
	}
	if names := ctx.FormTrim("labels"); names != "" && repository != nil {
		labelIDs, allFound, err := advisory_service.LabelIDsByNames(ctx, repository, strings.Split(names, ","))
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

	advisories, total, err := db.FindAndCount[advisory_model.Advisory](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	for _, advisory := range advisories {
		advisory.Repo = repository
	}
	result, err := toAPIAdvisories(ctx, advisories, viewer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.SetLinkHeader(total, opts.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, result)
}

// GetSecurityAdvisory gets a security advisory of a repository
func GetSecurityAdvisory(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/security-advisories/{gtsa_id} repository repoGetSecurityAdvisory
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
	// - name: gtsa_id
	//   in: path
	//   description: identifier of the advisory
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryAdvisory"
	//   default:
	//     "$ref": "#/responses/error"
	advisory, _ := advisory_service.RequestAdvisory(ctx)
	respondAdvisory(ctx, advisory, http.StatusOK)
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
	//   default:
	//     "$ref": "#/responses/error"
	if !advisoryViewer(ctx).CanManage() {
		ctx.APIError(http.StatusForbidden, "only repository admins can create security advisories")
		return
	}
	form := web.GetForm[*api.CreateRepositoryAdvisoryOption](ctx)
	if form.StartPrivateFork {
		ctx.APIError(http.StatusUnprocessableEntity, "temporary private forks are not supported")
		return
	}
	credits, err := toAdvisoryCredits(ctx, form.Credits)
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	advisory, err := advisory_service.CreateAdvisory(ctx, ctx.Doer, ctx.Repo.Repository, &advisory_service.ContentOptions{
		Summary:         form.Summary,
		Description:     form.Description,
		CveID:           form.CveID,
		Severity:        form.Severity,
		CvssVector:      form.CVSSVectorString,
		CweIDs:          form.CweIDs,
		Vulnerabilities: toAdvisoryVulnerabilities(form.Vulnerabilities),
		Credits:         credits,
		LabelIDs:        form.Labels,
	})
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	respondAdvisory(ctx, advisory, http.StatusCreated)
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
	//   default:
	//     "$ref": "#/responses/error"
	form := web.GetForm[*api.CreatePrivateVulnerabilityReportOption](ctx)
	if form.StartPrivateFork {
		ctx.APIError(http.StatusUnprocessableEntity, "temporary private forks are not supported")
		return
	}
	advisory, err := advisory_service.ReportVulnerability(ctx, ctx.Doer, ctx.Repo.Repository, &advisory_service.ContentOptions{
		Summary:         form.Summary,
		Description:     form.Description,
		Severity:        form.Severity,
		CvssVector:      form.CVSSVectorString,
		CweIDs:          form.CweIDs,
		Vulnerabilities: toAdvisoryVulnerabilities(form.Vulnerabilities),
	})
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	respondAdvisory(ctx, advisory, http.StatusCreated)
}

// EditSecurityAdvisory updates a security advisory
func EditSecurityAdvisory(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/security-advisories/{gtsa_id} repository repoEditSecurityAdvisory
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
	// - name: gtsa_id
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
	//   default:
	//     "$ref": "#/responses/error"
	form := web.GetForm[*api.EditRepositoryAdvisoryOption](ctx)
	advisory, perms := advisory_service.RequestAdvisory(ctx)
	if err := advisory.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	credits, err := toAdvisoryCredits(ctx, form.Credits)
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	opts := advisory_service.EditOptions{
		Summary:           optional.FromPtr(form.Summary),
		Description:       optional.FromPtr(form.Description),
		Severity:          optional.FromPtr(form.Severity),
		CvssVector:        optional.FromPtr(form.CVSSVectorString),
		CweIDs:            optionalList(form.CweIDs),
		Vulnerabilities:   optionalList(toAdvisoryVulnerabilities(form.Vulnerabilities)),
		CveID:             optional.FromPtr(form.CveID),
		Credits:           optionalList(credits),
		LabelIDs:          optionalList(form.Labels),
		CollaboratorUsers: optionalList(form.CollaboratingUsers),
		CollaboratorTeams: optionalList(form.CollaboratingTeams),
		State:             optional.FromPtr(form.State),
		CloseReason:       optional.FromPtr(form.CloseReason),
		DuplicateOf:       optional.FromPtr(form.DuplicateOf),
		ShareOriginal:     optional.FromPtr(form.ShareOriginal),
	}
	if err := advisory_service.EditAdvisory(ctx, ctx.Doer, advisory, perms, opts); err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	respondAdvisory(ctx, advisory, http.StatusOK)
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
	//   default:
	//     "$ref": "#/responses/error"
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
	//   default:
	//     "$ref": "#/responses/error"
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
	//   default:
	//     "$ref": "#/responses/error"
	setPrivateVulnerabilityReporting(ctx, false)
}

func setPrivateVulnerabilityReporting(ctx *context.APIContext, enabled bool) {
	if err := advisory_service.SetPrivateReporting(ctx, ctx.Doer, ctx.Repo.Repository, enabled); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func getAdvisoryComment(ctx *context.APIContext, advisoryID int64, includeConfidential bool) *advisory_model.Comment {
	comment, err := advisory_model.GetCommentByID(ctx, advisoryID, ctx.PathParamInt64("comment_id"), includeConfidential)
	if err != nil {
		ctx.APIErrorAuto(err)
		return nil
	}
	if _, comment.Poster, err = user_model.GetPossibleUserByID(ctx, comment.PosterID); err != nil {
		ctx.APIErrorAuto(err)
		return nil
	}
	return comment
}

// ListSecurityAdvisoryComments lists the comments of the private discussion of an advisory
func ListSecurityAdvisoryComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/security-advisories/{gtsa_id}/comments repository repoListSecurityAdvisoryComments
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
	// - name: gtsa_id
	//   in: path
	//   description: identifier of the advisory
	//   type: string
	//   required: true
	// - name: since
	//   in: query
	//   description: only return comments updated after this time
	//   type: string
	//   format: date-time
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryAdvisoryCommentList"
	//   default:
	//     "$ref": "#/responses/error"
	advisory, perms := advisory_service.RequestAdvisory(ctx)
	opts := advisory_model.FindCommentsOptions{ListOptions: utils.GetListOptions(ctx), AdvisoryID: advisory.ID, IncludeConfidential: perms.CanSeeConfidential}
	if since := ctx.FormTrim("since"); since != "" {
		var err error
		opts.UpdatedAfter, err = time.Parse(time.RFC3339, since)
		if err != nil {
			ctx.APIError(http.StatusUnprocessableEntity, "invalid 'since' format, expected ISO 8601 (RFC3339)")
			return
		}
	}
	comments, total, err := db.FindAndCount[advisory_model.Comment](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if err := advisory_model.LoadCommentPosters(ctx, comments); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	result := make([]*api.RepositoryAdvisoryComment, 0, len(comments))
	for _, comment := range comments {
		result = append(result, convert.ToAPIRepositoryAdvisoryComment(ctx, advisory, comment, ctx.Doer))
	}
	ctx.SetLinkHeader(total, opts.PageSize)
	ctx.SetTotalCountHeader(total)
	ctx.JSON(http.StatusOK, result)
}

func GetSecurityAdvisoryComment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/security-advisories/{gtsa_id}/comments/{comment_id} repository repoGetSecurityAdvisoryComment
	// ---
	// summary: Get a security advisory comment
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
	// - name: gtsa_id
	//   in: path
	//   description: identifier of the advisory
	//   type: string
	//   required: true
	// - name: comment_id
	//   in: path
	//   description: id of the comment
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryAdvisoryComment"
	//   default:
	//     "$ref": "#/responses/error"
	advisory, perms := advisory_service.RequestAdvisory(ctx)
	if comment := getAdvisoryComment(ctx, advisory.ID, perms.CanSeeConfidential); comment != nil {
		ctx.JSON(http.StatusOK, convert.ToAPIRepositoryAdvisoryComment(ctx, advisory, comment, ctx.Doer))
	}
}

// CreateSecurityAdvisoryComment adds a comment to the private discussion of an advisory
func CreateSecurityAdvisoryComment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/security-advisories/{gtsa_id}/comments repository repoCreateSecurityAdvisoryComment
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
	// - name: gtsa_id
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
	//   default:
	//     "$ref": "#/responses/error"
	form := web.GetForm[*api.RepositoryAdvisoryCommentOption](ctx)
	advisory, perms := advisory_service.RequestAdvisory(ctx)
	comment, err := advisory_service.CreateComment(ctx, ctx.Doer, advisory, perms, form.Body, form.IsConfidential)
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, convert.ToAPIRepositoryAdvisoryComment(ctx, advisory, comment, ctx.Doer))
}

// EditSecurityAdvisoryComment edits a comment of the private discussion of an advisory
func EditSecurityAdvisoryComment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/security-advisories/{gtsa_id}/comments/{comment_id} repository repoEditSecurityAdvisoryComment
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
	// - name: gtsa_id
	//   in: path
	//   description: identifier of the advisory
	//   type: string
	//   required: true
	// - name: comment_id
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
	//   default:
	//     "$ref": "#/responses/error"
	form := web.GetForm[*api.RepositoryAdvisoryCommentOption](ctx)
	advisory, perms := advisory_service.RequestAdvisory(ctx)
	comment := getAdvisoryComment(ctx, advisory.ID, perms.CanSeeConfidential)
	if comment == nil {
		return
	}
	if err := advisory_service.EditComment(ctx, ctx.Doer, advisory, comment, form.Body); err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIRepositoryAdvisoryComment(ctx, advisory, comment, ctx.Doer))
}

// DeleteSecurityAdvisoryComment deletes a comment of the private discussion of an advisory
func DeleteSecurityAdvisoryComment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/security-advisories/{gtsa_id}/comments/{comment_id} repository repoDeleteSecurityAdvisoryComment
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
	// - name: gtsa_id
	//   in: path
	//   description: identifier of the advisory
	//   type: string
	//   required: true
	// - name: comment_id
	//   in: path
	//   description: id of the comment
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   default:
	//     "$ref": "#/responses/error"
	advisory, perms := advisory_service.RequestAdvisory(ctx)
	comment := getAdvisoryComment(ctx, advisory.ID, perms.CanSeeConfidential)
	if comment == nil {
		return
	}
	if err := advisory_service.DeleteComment(ctx, ctx.Doer, perms, comment); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
