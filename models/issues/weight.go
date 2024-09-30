// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"

	"code.gitea.io/gitea/models/db"
)

// GetIssueTotalWeight returns the total weight for issues by given conditions.
func GetIssueTotalWeight(ctx context.Context, opts *IssuesOptions) (int, int, error) {
	if len(opts.IssueIDs) <= MaxQueryParameters {
		return getIssueTotalWeightChunk(ctx, opts, opts.IssueIDs)
	}

	// If too long a list of IDs is provided,
	// we get the statistics in smaller chunks and get accumulates
	var weightSum int
	var closedWeightSum int
	for i := 0; i < len(opts.IssueIDs); {
		chunk := i + MaxQueryParameters
		if chunk > len(opts.IssueIDs) {
			chunk = len(opts.IssueIDs)
		}
		weight, closedWeight, err := getIssueTotalWeightChunk(ctx, opts, opts.IssueIDs[i:chunk])
		if err != nil {
			return 0, 0, err
		}
		weightSum += weight
		closedWeightSum += closedWeight
		i = chunk
	}

	return weightSum, closedWeightSum, nil
}

func getIssueTotalWeightChunk(ctx context.Context, opts *IssuesOptions, issueIDs []int64) (int, int, error) {
	type totalWeight struct {
		Weight       int
		WeightClosed int
	}

	tw := &totalWeight{}

	session := db.GetEngine(ctx).Table("issue").
		Select("sum(weight) as weight, sum(CASE WHEN is_closed THEN 0 ELSE weight END) as weight_closed")

	has, err := applyIssuesOptions(session, opts, issueIDs).
		Get(tw)

	if err != nil {
		return 0, 0, err
	} else if !has {
		return 0, 0, nil
	}

	return tw.Weight, tw.WeightClosed, nil
}
