// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"fmt"

	advisory_model "gitea.dev/models/advisory"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

// ToAPIRepositoryAdvisory converts the public parts of an advisory with loaded attributes
func ToAPIRepositoryAdvisory(ctx context.Context, a *advisory_model.Advisory, doer *user_model.User) *api.RepositoryAdvisory {
	res := &api.RepositoryAdvisory{
		Identifier:      a.Identifier,
		CveID:           a.CveID,
		URL:             a.Repo.APIURL() + "/security-advisories/" + a.Identifier,
		HTMLURL:         a.HTMLURL(),
		Summary:         a.Summary,
		Description:     a.Description,
		State:           a.State.String(),
		CreatedAt:       a.CreatedUnix.AsTime(),
		UpdatedAt:       a.UpdatedUnix.AsTime(),
		PublishedAt:     util.Iif(a.PublishedUnix == 0, nil, a.PublishedUnix.AsTimePtr()),
		ClosedAt:        util.Iif(a.ClosedUnix == 0, nil, a.ClosedUnix.AsTimePtr()),
		WithdrawnAt:     util.Iif(a.WithdrawnUnix == 0, nil, a.WithdrawnUnix.AsTimePtr()),
		Identifiers:     []*api.RepositoryAdvisoryIdentifier{},
		Vulnerabilities: make([]*api.RepositoryAdvisoryVulnerability, 0, len(a.Vulnerabilities)),
		CVSSSeverities:  &api.RepositoryAdvisoryCVSSSeverities{},
		CweIDs:          util.SliceNilAsEmpty(a.CweIDs),
		Credits:         make([]*api.RepositoryAdvisoryCredit, 0, len(a.Credits)),
		CreditsDetailed: make([]*api.RepositoryAdvisoryCreditDetailed, 0, len(a.Credits)),
		Labels:          ToLabelList(a.Labels, a.Repo, a.Repo.Owner),
		CloseReason:     a.CloseReason.String(),
	}
	if a.DuplicateOf != nil {
		res.DuplicateOf = a.DuplicateOf.Identifier
	}
	if a.Severity != advisory_model.SeverityUnknown {
		res.Severity = new(a.Severity.String())
	}
	if a.CveID != "" {
		res.Identifiers = append(res.Identifiers, &api.RepositoryAdvisoryIdentifier{Type: "CVE", Value: a.CveID})
	}
	if a.Publisher != nil {
		res.Publisher = ToUser(ctx, a.Publisher, doer)
	}
	for _, v := range a.Vulnerabilities {
		res.Vulnerabilities = append(res.Vulnerabilities, &api.RepositoryAdvisoryVulnerability{
			Package:                &api.RepositoryAdvisoryPackage{Ecosystem: v.Ecosystem, Name: v.PackageName},
			VulnerableVersionRange: v.VulnerableVersionRange,
			PatchedVersions:        v.PatchedVersions,
			VulnerableFunctions:    util.SliceNilAsEmpty(v.VulnerableFunctions),
		})
	}
	if a.CvssV3Vector != "" {
		res.CVSSSeverities.CvssV3 = &api.RepositoryAdvisoryCVSS{VectorString: a.CvssV3Vector, Score: a.CvssV3Score()}
		res.CVSS = res.CVSSSeverities.CvssV3
	}
	if a.CvssV4Vector != "" {
		res.CVSSSeverities.CvssV4 = &api.RepositoryAdvisoryCVSS{VectorString: a.CvssV4Vector, Score: a.CvssV4Score()}
	}
	for _, c := range a.Credits {
		res.Credits = append(res.Credits, &api.RepositoryAdvisoryCredit{Login: c.User.Name, Type: c.Type})
		res.CreditsDetailed = append(res.CreditsDetailed, &api.RepositoryAdvisoryCreditDetailed{User: ToUser(ctx, c.User, doer), Type: c.Type, State: "accepted"})
	}
	return res
}

// ToAPIRepositoryAdvisoryWithPrivateDetails also includes the reporter and the collaborators, which must be loaded,
// for those who can see the discussion
func ToAPIRepositoryAdvisoryWithPrivateDetails(ctx context.Context, a *advisory_model.Advisory, doer *user_model.User) (*api.RepositoryAdvisory, error) {
	res := ToAPIRepositoryAdvisory(ctx, a, doer)
	res.Author = ToUser(ctx, a.Reporter, doer)
	if a.IsReport {
		res.Submission = &api.RepositoryAdvisorySubmission{Accepted: a.State != advisory_model.StateTriage && a.State != advisory_model.StateClosed}
	}
	res.CollaboratingUsers = ToUsers(ctx, doer, a.CollaboratorUsers)
	var err error
	if res.CollaboratingTeams, err = ToTeams(ctx, a.CollaboratorTeams, false); err != nil {
		return nil, err
	}
	return res, nil
}

func ToAPIRepositoryAdvisoryComment(ctx context.Context, a *advisory_model.Advisory, c *advisory_model.Comment, doer *user_model.User) *api.RepositoryAdvisoryComment {
	return &api.RepositoryAdvisoryComment{
		ID:         c.ID,
		HTMLURL:    fmt.Sprintf("%s#advisory-comment-%d", a.HTMLURL(), c.ID),
		User:       ToUser(ctx, c.Poster, doer),
		Body:       c.Content,
		IsInternal: c.IsInternal,
		CreatedAt:  c.CreatedUnix.AsTime(),
		UpdatedAt:  c.UpdatedUnix.AsTime(),
	}
}
