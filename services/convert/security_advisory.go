// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"fmt"

	advisory_model "gitea.dev/models/advisory"
	"gitea.dev/models/organization"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/cvss"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

func nullableAdvisoryString(value string) *string {
	return util.Iif(value == "", nil, new(value))
}

// ToAPIRepositoryAdvisory converts the public parts of an advisory with loaded attributes
func ToAPIRepositoryAdvisory(ctx context.Context, advisory *advisory_model.Advisory, doer *user_model.User) *api.RepositoryAdvisory {
	result := &api.RepositoryAdvisory{
		Identifier:      advisory.Identifier,
		CveID:           nullableAdvisoryString(advisory.CveID),
		URL:             advisoryAPIURL(ctx, advisory),
		HTMLURL:         advisory.HTMLURL(),
		Summary:         advisory.Summary,
		Description:     advisory.Description,
		State:           advisory.State.String(),
		CreatedAt:       advisory.CreatedUnix.AsTime(),
		UpdatedAt:       advisory.UpdatedUnix.AsTime(),
		PublishedAt:     util.Iif(advisory.PublishedUnix == 0, nil, advisory.PublishedUnix.AsTimePtr()),
		ClosedAt:        util.Iif(advisory.ClosedUnix == 0, nil, advisory.ClosedUnix.AsTimePtr()),
		WithdrawnAt:     util.Iif(advisory.WithdrawnUnix == 0, nil, advisory.WithdrawnUnix.AsTimePtr()),
		Identifiers:     []*api.RepositoryAdvisoryIdentifier{{Type: "GTSA", Value: advisory.Identifier}},
		Vulnerabilities: make([]*api.RepositoryAdvisoryVulnerability, 0, len(advisory.Vulnerabilities)),
		CVSSSeverities:  &api.RepositoryAdvisoryCVSSSeverities{},
		CweIDs:          util.SliceNilAsEmpty(advisory.CweIDs),
		Credits:         make([]*api.RepositoryAdvisoryCredit, 0, len(advisory.Credits)),
		CreditsDetailed: make([]*api.RepositoryAdvisoryCreditDetailed, 0, len(advisory.Credits)),
		Labels:          []*api.Label{},
		CloseReason:     advisory.CloseReason.String(),
	}
	if advisory.DuplicateOf != nil {
		result.DuplicateOf = advisory.DuplicateOf.Identifier
	}
	if advisory.Severity != advisory_model.SeverityUnknown {
		result.Severity = new(advisory.Severity.String())
	}
	if advisory.CveID != "" {
		result.Identifiers = append(result.Identifiers, &api.RepositoryAdvisoryIdentifier{Type: "CVE", Value: advisory.CveID})
	}
	if advisory.Publisher != nil {
		result.Publisher = ToUser(ctx, advisory.Publisher, doer)
	}
	for _, vulnerability := range advisory.Vulnerabilities {
		result.Vulnerabilities = append(result.Vulnerabilities, &api.RepositoryAdvisoryVulnerability{
			Package:                &api.RepositoryAdvisoryPackage{Ecosystem: vulnerability.Ecosystem, Name: nullableAdvisoryString(vulnerability.PackageName)},
			VulnerableVersionRange: nullableAdvisoryString(vulnerability.VulnerableVersionRange),
			PatchedVersions:        nullableAdvisoryString(vulnerability.PatchedVersions),
			VulnerableFunctions:    util.SliceNilAsEmpty(vulnerability.VulnerableFunctions),
		})
	}
	switch advisory.CvssVersion() {
	case cvss.Version31:
		result.CVSS = &api.RepositoryAdvisoryCVSS{VectorString: advisory.CvssVector, Score: advisory.CvssScore()}
		result.CVSSSeverities.CvssV3 = result.CVSS
	case cvss.Version40:
		result.CVSSSeverities.CvssV4 = &api.RepositoryAdvisoryCVSS{VectorString: advisory.CvssVector, Score: advisory.CvssScore()}
	}
	for _, credit := range advisory.VisibleCredits(ctx, doer) {
		result.Credits = append(result.Credits, &api.RepositoryAdvisoryCredit{Login: credit.User.Name, Type: credit.Type})
		result.CreditsDetailed = append(result.CreditsDetailed, &api.RepositoryAdvisoryCreditDetailed{User: ToUser(ctx, credit.User, doer), Type: credit.Type, State: "accepted"})
	}
	return result
}

// AddAPIRepositoryAdvisoryPrivateDetails is only for viewers who can see the discussion.
func AddAPIRepositoryAdvisoryPrivateDetails(ctx context.Context, result *api.RepositoryAdvisory, advisory *advisory_model.Advisory, doer *user_model.User, allTeams bool) error {
	result.Author = ToUser(ctx, advisory.Reporter, doer)
	result.Labels = ToLabelList(advisory.Labels, advisory.Repo, advisory.Repo.Owner)
	if advisory.IsReport {
		result.Submission = &api.RepositoryAdvisorySubmission{Accepted: advisory.State != advisory_model.StateTriage && advisory.State != advisory_model.StateClosed}
	}
	result.CollaboratingUsers = ToUsers(ctx, doer, advisory.CollaboratorUsers)
	teams := advisory.CollaboratorTeams
	if !allTeams {
		teams = make([]*organization.Team, 0, len(advisory.CollaboratorTeams))
		for _, team := range advisory.CollaboratorTeams {
			visible, err := organization.IsTeamMember(ctx, team.OrgID, team.ID, doer.ID)
			if err != nil {
				return err
			}
			if !visible {
				visible, err = team.CanNonMemberReadMeta(ctx, advisory.Repo.Owner, doer)
				if err != nil {
					return err
				}
			}
			if visible {
				teams = append(teams, team)
			}
		}
	}
	var err error
	result.CollaboratingTeams, err = ToTeams(ctx, teams, false)
	return err
}

func advisoryAPIURL(ctx context.Context, advisory *advisory_model.Advisory) string {
	return advisory.Repo.APIURL(ctx) + "/security-advisories/" + advisory.Identifier
}

func ToAPIRepositoryAdvisoryComment(ctx context.Context, advisory *advisory_model.Advisory, comment *advisory_model.Comment, doer *user_model.User) *api.RepositoryAdvisoryComment {
	advisoryURL := advisoryAPIURL(ctx, advisory)
	return &api.RepositoryAdvisoryComment{
		ID:             comment.ID,
		URL:            fmt.Sprintf("%s/comments/%d", advisoryURL, comment.ID),
		HTMLURL:        fmt.Sprintf("%s#advisory-comment-%d", advisory.HTMLURL(), comment.ID),
		AdvisoryURL:    advisoryURL,
		User:           ToUser(ctx, comment.Poster, doer),
		Body:           comment.Content,
		IsConfidential: comment.IsConfidential,
		CreatedAt:      comment.CreatedUnix.AsTime(),
		UpdatedAt:      comment.UpdatedUnix.AsTime(),
	}
}
