// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"strconv"

	disclosure_model "gitea.dev/models/disclosure"
	repo_model "gitea.dev/models/repo"
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
		HTMLURL:          repo.Link() + "/security/" + strconv.FormatInt(d.ID, 10),
	}
	if d.CVSSVector != "" {
		result.CVSS = &api.SecurityDisclosureCVSS{
			Version: disclosure_model.CVSSVersion(d.CVSSVector),
			Vector:  d.CVSSVector,
		}
	}
	return result
}
