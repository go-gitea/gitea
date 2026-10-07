// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/db"
	"gitea.dev/models/organization"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/cvss"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web"
	"gitea.dev/routers/api/v1/utils"
	advisory_service "gitea.dev/services/advisory"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

func advisoryViewer(ctx *context.APIContext) advisory_model.Viewer {
	return advisory_model.NewViewer(ctx.Doer, ctx.Repo.Permission, ctx.PublicOnly)
}

// apiAdvisoryError responds with 422 for validation errors like GitHub does
func apiAdvisoryError(ctx *context.APIContext, err error) {
	if errors.Is(err, util.ErrInvalidArgument) {
		ctx.APIError(http.StatusUnprocessableEntity, err.Error())
		return
	}
	ctx.APIErrorAuto(err)
}

func toAPIAdvisories(ctx *context.APIContext, viewer advisory_model.Viewer, list advisory_model.List) ([]*api.RepositoryAdvisory, error) {
	if err := list.LoadAttributes(ctx); err != nil {
		return nil, err
	}
	if err := viewer.HideUnviewableOriginals(ctx, list); err != nil {
		return nil, err
	}
	fullIDs, err := viewer.DiscussionIDs(ctx, list)
	if err != nil {
		return nil, err
	}
	full := container.FilterSlice(list, func(a *advisory_model.Advisory) (*advisory_model.Advisory, bool) { return a, fullIDs.Contains(a.ID) })
	if err := advisory_model.List(full).LoadCollaborators(ctx); err != nil {
		return nil, err
	}
	res := make([]*api.RepositoryAdvisory, 0, len(list))
	for _, a := range list {
		apiAdvisory, err := convert.ToAPIRepositoryAdvisory(ctx, a, ctx.Doer, fullIDs.Contains(a.ID))
		if err != nil {
			return nil, err
		}
		res = append(res, apiAdvisory)
	}
	return res, nil
}

func respondAdvisory(ctx *context.APIContext, viewer advisory_model.Viewer, a *advisory_model.Advisory, status int) {
	res, err := toAPIAdvisories(ctx, viewer, advisory_model.List{a})
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.JSON(status, res[0])
}

// getVisibleAdvisory loads the advisory of the path, it responds with 404 if the doer cannot see it
func getVisibleAdvisory(ctx *context.APIContext, viewer advisory_model.Viewer) (*advisory_model.Advisory, advisory_model.Permissions) {
	a, perms, err := viewer.GetAdvisory(ctx, ctx.Repo.Repository, ctx.PathParam("identifier"))
	if err != nil {
		ctx.APIErrorAuto(err)
	}
	return a, perms
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

// labelIDsByNames prefers repository labels over organization labels of the same name, an unknown name matches nothing
func labelIDsByNames(ctx *context.APIContext, names []string) ([]int64, error) {
	labels, err := advisory_service.RepoAndOrgLabels(ctx, ctx.Repo.Repository)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]int64, len(labels))
	for _, l := range slices.Backward(labels) {
		byName[l.Name] = l.ID
	}
	ids := make([]int64, 0, len(names))
	for _, name := range names {
		id, ok := byName[strings.TrimSpace(name)]
		if !ok {
			return nil, nil
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// setCVSSVector replaces the vector of the same CVSS version, unsupported vectors are passed on to be rejected by the service
func setCVSSVector(opts *advisory_service.ContentOptions, vector string) {
	vector = strings.TrimSpace(vector)
	if cvss.DetectVersion(vector) == cvss.Version40 {
		opts.CvssV4Vector = vector
	} else {
		opts.CvssV3Vector = vector
	}
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
	listOptions := utils.GetListOptions(ctx)
	viewer := advisoryViewer(ctx)
	stateName, severity, closeReason, direction := ctx.FormString("state"), ctx.FormString("severity"), ctx.FormString("close_reason"), ctx.FormString("direction")
	opts := advisory_model.FindAdvisoriesOptions{
		ListOptions: listOptions,
		RepoID:      ctx.Repo.Repository.ID,
		Viewer:      viewer,
		Keyword:     ctx.FormTrim("q"),
		Severity:    advisory_model.ParseSeverity(severity),
		Ecosystem:   ctx.FormTrim("ecosystem"),
		CweID:       strings.ToUpper(ctx.FormTrim("cwe")),
		CloseReason: advisory_model.ParseCloseReason(closeReason),
		SortBy:      ctx.FormString("sort"),
		Ascending:   direction == "asc",
	}
	state := advisory_model.ParseState(stateName)
	for name, invalid := range map[string]bool{
		"state":        stateName != "" && state == 0,
		"severity":     severity != "" && opts.Severity == advisory_model.SeverityUnknown,
		"close_reason": closeReason != "" && opts.CloseReason == advisory_model.CloseReasonNone,
		"sort":         !slices.Contains([]string{"", "created", "updated", "published"}, opts.SortBy),
		"direction":    !slices.Contains([]string{"", "asc", "desc"}, direction),
		"ecosystem":    opts.Ecosystem != "" && !slices.Contains(advisory_model.Ecosystems, opts.Ecosystem),
		"cwe":          opts.CweID != "" && !advisory_service.IsValidCweID(opts.CweID), // also keeps LIKE wildcards out
	} {
		if invalid {
			ctx.APIError(http.StatusUnprocessableEntity, "invalid "+name)
			return
		}
	}
	if state != 0 {
		opts.States = []advisory_model.State{state}
	}
	if names := ctx.FormTrim("labels"); names != "" {
		var err error
		if opts.LabelIDs, err = labelIDsByNames(ctx, strings.Split(names, ",")); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		if len(opts.LabelIDs) == 0 {
			ctx.SetTotalCountHeader(0)
			ctx.JSON(http.StatusOK, []*api.RepositoryAdvisory{})
			return
		}
	}

	advisories, total, err := db.FindAndCount[advisory_model.Advisory](ctx, opts)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	for _, a := range advisories {
		a.Repo = ctx.Repo.Repository
	}
	res, err := toAPIAdvisories(ctx, viewer, advisories)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	ctx.SetLinkHeader(total, listOptions.PageSize)
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
	viewer := advisoryViewer(ctx)
	a, _ := getVisibleAdvisory(ctx, viewer)
	if ctx.Written() {
		return
	}
	respondAdvisory(ctx, viewer, a, http.StatusOK)
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
	viewer := advisoryViewer(ctx)
	if !viewer.CanManage() {
		ctx.APIError(http.StatusForbidden, "only repository admins can create security advisories")
		return
	}
	credits, err := toAdvisoryCredits(ctx, form.Credits)
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	opts := &advisory_service.ContentOptions{
		Summary:         form.Summary,
		Description:     form.Description,
		CveID:           form.CveID,
		Severity:        form.Severity,
		CweIDs:          form.CweIDs,
		Vulnerabilities: toAdvisoryVulnerabilities(form.Vulnerabilities),
		Credits:         credits,
		LabelIDs:        form.Labels,
	}
	setCVSSVector(opts, form.CVSSVectorString)
	a, err := advisory_service.CreateAdvisory(ctx, ctx.Doer, ctx.Repo.Repository, opts)
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	respondAdvisory(ctx, viewer, a, http.StatusCreated)
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
	opts := &advisory_service.ContentOptions{
		Summary:         form.Summary,
		Description:     form.Description,
		Severity:        form.Severity,
		CweIDs:          form.CweIDs,
		Vulnerabilities: toAdvisoryVulnerabilities(form.Vulnerabilities),
	}
	setCVSSVector(opts, form.CVSSVectorString)
	a, err := advisory_service.ReportVulnerability(ctx, ctx.Doer, ctx.Repo.Repository, opts)
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	respondAdvisory(ctx, advisoryViewer(ctx), a, http.StatusCreated)
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
	viewer := advisoryViewer(ctx)
	a, perms := getVisibleAdvisory(ctx, viewer)
	if ctx.Written() {
		return
	}
	if err := a.LoadAttributes(ctx); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	editContent := form.Summary != nil || form.Description != nil || form.Vulnerabilities != nil ||
		form.CweIDs != nil || form.Severity != nil || form.CVSSVectorString != nil
	manageContent := form.CveID != nil || form.Credits != nil || form.Labels != nil
	setCollaborators := form.CollaboratingUsers != nil || form.CollaboratingTeams != nil
	manageOther := form.State != nil || form.CloseReason != nil || form.DuplicateOf != nil || setCollaborators
	if (manageContent || manageOther) && !viewer.CanManage() {
		ctx.APIError(http.StatusForbidden, "only repository admins can change the CVE ID, credits, labels, state and collaborators")
		return
	}
	if editContent && !perms.CanEdit {
		ctx.APIError(http.StatusForbidden, "you cannot edit this advisory")
		return
	}

	// everything is validated before the first change is saved
	stateOpts, err := stateOptionsFromForm(ctx, a, form)
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	users, teams, err := resolveCollaborators(ctx, a, form)
	if err != nil {
		apiAdvisoryError(ctx, err)
		return
	}
	if setCollaborators {
		if err := advisory_service.ValidateNewCollaborators(ctx, a, users, teams); err != nil {
			apiAdvisoryError(ctx, err)
			return
		}
	}

	if editContent || manageContent {
		opts := advisory_service.ContentFromAdvisory(a)
		if form.Summary != nil {
			opts.Summary = *form.Summary
		}
		if form.Description != nil {
			opts.Description = *form.Description
		}
		if form.CveID != nil {
			opts.CveID = *form.CveID
		}
		if form.Vulnerabilities != nil {
			opts.Vulnerabilities = toAdvisoryVulnerabilities(form.Vulnerabilities)
		}
		if form.CweIDs != nil {
			opts.CweIDs = form.CweIDs
		}
		if form.Credits != nil {
			if opts.Credits, err = toAdvisoryCredits(ctx, form.Credits); err != nil {
				apiAdvisoryError(ctx, err)
				return
			}
		}
		if form.Labels != nil {
			opts.LabelIDs = form.Labels
		}
		if form.CVSSVectorString != nil {
			if *form.CVSSVectorString == "" {
				opts.CvssV3Vector, opts.CvssV4Vector = "", ""
			} else {
				opts.Severity = ""
				setCVSSVector(opts, *form.CVSSVectorString)
			}
		}
		if form.Severity != nil {
			opts.Severity = *form.Severity
		}
		if err := advisory_service.UpdateAdvisory(ctx, ctx.Doer, a, opts, viewer.CanManage()); err != nil {
			apiAdvisoryError(ctx, err)
			return
		}
	}
	if setCollaborators {
		if err := advisory_service.SetCollaborators(ctx, ctx.Doer, a, users, teams); err != nil {
			apiAdvisoryError(ctx, err)
			return
		}
	}
	if stateOpts != nil {
		if err := advisory_service.ChangeState(ctx, ctx.Doer, a, *stateOpts); err != nil {
			apiAdvisoryError(ctx, err)
			return
		}
	}
	respondAdvisory(ctx, viewer, a, http.StatusOK)
}

// stateOptionsFromForm returns nil if the state doesn't change
func stateOptionsFromForm(ctx *context.APIContext, a *advisory_model.Advisory, form *api.EditRepositoryAdvisoryOption) (*advisory_service.StateOptions, error) {
	if form.State == nil || advisory_model.ParseState(*form.State) == a.State {
		if form.CloseReason != nil || form.DuplicateOf != nil {
			return nil, util.NewInvalidArgumentErrorf("the close reason can only be set when closing the advisory")
		}
		return nil, nil //nolint:nilnil // the state doesn't change
	}
	var closeReason, duplicateOf string
	if form.CloseReason != nil {
		closeReason = *form.CloseReason
	}
	if form.DuplicateOf != nil {
		duplicateOf = *form.DuplicateOf
	}
	opts, err := advisory_service.NewStateOptions(ctx, a, *form.State, closeReason, duplicateOf)
	return &opts, err
}

// resolveCollaborators keeps the current users or teams if only the other list is changed
func resolveCollaborators(ctx *context.APIContext, a *advisory_model.Advisory, form *api.EditRepositoryAdvisoryOption) ([]*user_model.User, []*organization.Team, error) {
	if form.CollaboratingUsers == nil && form.CollaboratingTeams == nil {
		return nil, nil, nil
	}
	if err := a.LoadCollaborators(ctx); err != nil {
		return nil, nil, err
	}
	users, teams := a.CollaboratorUsers, a.CollaboratorTeams
	if form.CollaboratingUsers != nil {
		users = make([]*user_model.User, 0, len(form.CollaboratingUsers))
		for _, login := range form.CollaboratingUsers {
			u, err := user_model.GetUserByName(ctx, login)
			if user_model.IsErrUserNotExist(err) {
				return nil, nil, util.NewInvalidArgumentErrorf("user %q does not exist", login)
			} else if err != nil {
				return nil, nil, err
			}
			users = append(users, u)
		}
	}
	if form.CollaboratingTeams != nil {
		teams = make([]*organization.Team, 0, len(form.CollaboratingTeams))
		for _, name := range form.CollaboratingTeams {
			t, err := organization.GetTeam(ctx, ctx.Repo.Repository.OwnerID, name)
			if organization.IsErrTeamNotExist(err) {
				return nil, nil, util.NewInvalidArgumentErrorf("team %q does not exist", name)
			} else if err != nil {
				return nil, nil, err
			}
			teams = append(teams, t)
		}
	}
	return users, teams, nil
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

// getAdvisoryDiscussion loads the advisory of the path, it responds with 404 if the doer cannot see its discussion
func getAdvisoryDiscussion(ctx *context.APIContext) *advisory_model.Advisory {
	a, perms := getVisibleAdvisory(ctx, advisoryViewer(ctx))
	if ctx.Written() {
		return nil
	}
	if !perms.CanSeeDiscussion {
		ctx.APIErrorNotFound()
		return nil
	}
	return a
}

func getAdvisoryComment(ctx *context.APIContext) (*advisory_model.Advisory, *advisory_model.Comment) {
	a := getAdvisoryDiscussion(ctx)
	if ctx.Written() {
		return nil, nil
	}
	c, err := advisory_model.GetCommentByID(ctx, a.ID, ctx.PathParamInt64("id"))
	if err == nil {
		_, c.Poster, err = user_model.GetPossibleUserByID(ctx, c.PosterID)
	}
	if err != nil {
		ctx.APIErrorAuto(err)
		return nil, nil
	}
	return a, c
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
	a := getAdvisoryDiscussion(ctx)
	if ctx.Written() {
		return
	}
	listOptions := utils.GetListOptions(ctx)
	comments, total, err := db.FindAndCount[advisory_model.Comment](ctx, advisory_model.FindCommentsOptions{ListOptions: listOptions, AdvisoryID: a.ID})
	if err == nil {
		err = advisory_model.CommentList(comments).LoadPosters(ctx)
	}
	if err != nil {
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
	a := getAdvisoryDiscussion(ctx)
	if ctx.Written() {
		return
	}
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
	a, c := getAdvisoryComment(ctx)
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
	_, c := getAdvisoryComment(ctx)
	if ctx.Written() {
		return
	}
	if err := advisory_service.DeleteComment(ctx, ctx.Doer, c, advisoryViewer(ctx).CanManage()); err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
