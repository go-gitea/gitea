// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"strconv"

	disclosure_model "gitea.dev/models/disclosure"
	repo_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
)

// ToSecurityDisclosure converts a disclosure_model.Disclosure to an api.SecurityDisclosure
func ToSecurityDisclosure(ctx context.Context, repo *repo_model.Repository, d *disclosure_model.Disclosure) *api.SecurityDisclosure {
	result := &api.SecurityDisclosure{
		ID:               d.ID,
		Identifier:       d.Identifier(),
		Title:            d.Title,
		Summary:          d.Summary,
		Description:      d.Content,
		Severity:         string(d.Severity),
		CweIds:           d.CweIds,
		CveID:            d.CveID,
		References:       d.References,
		Credits:          toAPICredits(ctx, d.Credits),
		AffectedVersions: d.AffectedVersions(),
		FixedVersions:    d.FixedVersions(),
		Status:           string(d.Status),
		WithdrawReason:   d.WithdrawReason,
		Author:           ToAuthor(ctx, d.Author, "", nil),
		PublishedBy:      ToAuthor(ctx, d.Publisher, "", nil),
		CreatedAt:        d.CreatedUnix.AsTime(),
		UpdatedAt:        d.UpdatedUnix.AsTime(),
		PublishedAt:      util.Iif(d.PublishedAt > 0, d.PublishedAt.AsTimePtr(), nil),
		URL:              repo.APIURL() + "/security/disclosures/" + strconv.FormatInt(d.ID, 10),
		HTMLURL:          repo.Link() + "/security/disclosures/" + strconv.FormatInt(d.ID, 10),
	}
	if d.CVSSVector != "" {
		result.CVSS = &api.SecurityDisclosureCVSS{
			Version: disclosure_model.CVSSVersion(d.CVSSVector),
			Vector:  d.CVSSVector,
			Score:   d.CVSSScore(),
		}
	}
	return result
}

// toAPICredits converts the stored credits to their API representation
func toAPICredits(ctx context.Context, credits disclosure_model.CreditList) []api.SecurityDisclosureCredit {
	if len(credits) == 0 {
		return nil
	}
	// resolve local users that were not loaded yet (e.g. right after creation)
	userIDs := make([]int64, 0, len(credits))
	for _, c := range credits {
		if c.User == nil && c.UserID > 0 {
			userIDs = append(userIDs, c.UserID)
		}
	}
	userMap := map[int64]*user_model.User{}
	if len(userIDs) > 0 {
		var err error
		userMap, err = user_model.GetUsersMapByIDs(ctx, userIDs)
		if err != nil {
			return nil // the name is still returned, only the user link is missing
		}
	}
	result := make([]api.SecurityDisclosureCredit, 0, len(credits))
	for _, c := range credits {
		apiCredit := api.SecurityDisclosureCredit{
			Role: api.SecurityDisclosureCreditRole(c.Role),
			Name: c.Name,
			URL:  c.URL,
		}
		user := c.User
		if user == nil {
			user = userMap[c.UserID]
		}
		if user != nil {
			apiCredit.User = ToAuthor(ctx, user, "", nil)
		}
		result = append(result, apiCredit)
	}
	return result
}
