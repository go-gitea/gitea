// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	system_model "gitea.dev/models/system"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
)

// WarnAndNotice will log the provided message and send a repository notice
func WarnAndNotice(fmtStr string, args ...any) {
	log.Warn(fmtStr, args...)
	if err := system_model.CreateRepositoryNotice(fmt.Sprintf(fmtStr, args...)); err != nil {
		log.Error("create repository notice failed: ", err)
	}
}

func downloadAsset(ctx context.Context, client *http.Client, assetURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return resp.Body, nil
}

func hasBaseURL(toCheck, baseURL string) bool {
	if baseURL == "" {
		return false
	}
	if len(baseURL) > 0 && baseURL[len(baseURL)-1] != '/' {
		baseURL += "/"
	}
	return strings.HasPrefix(toCheck, baseURL)
}

// CheckAndEnsureSafePR will check that a given PR is safe to download
func CheckAndEnsureSafePR(pr *base.PullRequest, commonCloneBaseURL string, g base.Downloader) bool {
	valid := true
	// SECURITY: the patchURL must be checked to have the same baseURL as the current to prevent open redirect
	if pr.PatchURL != "" && !hasBaseURL(pr.PatchURL, commonCloneBaseURL) {
		// TODO: Should we check that this url has the expected format for a patch url?
		WarnAndNotice("PR #%d in %s has invalid PatchURL: %s baseURL: %s", pr.Number, g, pr.PatchURL, commonCloneBaseURL)
		pr.PatchURL = ""
		valid = false
	}

	// SECURITY: the headCloneURL must be checked to have the same baseURL as the current to prevent open redirect
	if pr.Head.CloneURL != "" && !hasBaseURL(pr.Head.CloneURL, commonCloneBaseURL) {
		// TODO: Should we check that this url has the expected format for a patch url?
		WarnAndNotice("PR #%d in %s has invalid HeadCloneURL: %s baseURL: %s", pr.Number, g, pr.Head.CloneURL, commonCloneBaseURL)
		pr.Head.CloneURL = ""
		valid = false
	}

	// SECURITY: SHAs must be valid Git object IDs.
	// The repository object format is not yet available at this stage.
	if pr.MergeCommitSHA != "" && !git.IsStringValidObjectID(nil, pr.MergeCommitSHA) {
		WarnAndNotice("PR #%d in %s has invalid MergeCommitSHA: %s", pr.Number, g, pr.MergeCommitSHA)
		pr.MergeCommitSHA = ""
	}
	if pr.Head.SHA != "" && !git.IsStringValidObjectID(nil, pr.Head.SHA) {
		WarnAndNotice("PR #%d in %s has invalid HeadSHA: %s", pr.Number, g, pr.Head.SHA)
		pr.Head.SHA = ""
		valid = false
	}
	if pr.Base.SHA != "" && !git.IsStringValidObjectID(nil, pr.Base.SHA) {
		WarnAndNotice("PR #%d in %s has invalid BaseSHA: %s", pr.Number, g, pr.Base.SHA)
		pr.Base.SHA = ""
		valid = false
	}

	// SECURITY: Refs must be valid refs or SHAs
	if pr.Head.Ref != "" && !git.IsValidRefPattern(pr.Head.Ref) {
		WarnAndNotice("PR #%d in %s has invalid HeadRef: %s", pr.Number, g, pr.Head.Ref)
		pr.Head.Ref = ""
		valid = false
	}
	if pr.Base.Ref != "" && !git.IsValidRefPattern(pr.Base.Ref) {
		WarnAndNotice("PR #%d in %s has invalid BaseRef: %s", pr.Number, g, pr.Base.Ref)
		pr.Base.Ref = ""
		valid = false
	}

	pr.EnsuredSafe = true

	return valid
}
