// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"fmt"
	"strings"

	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	base "code.gitea.io/gitea/modules/migration"
)

// WarnAndNotice will log the provided message and send a repository notice
func WarnAndNotice(fmtStr string, args ...any) {
	log.Warn(fmtStr, args...)
	if err := system_model.CreateRepositoryNotice(fmt.Sprintf(fmtStr, args...)); err != nil {
		log.Error("create repository notice failed: ", err)
	}
}

func hasBaseURL(toCheck, baseURL string) bool {
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

	// SECURITY: SHAs Must be a SHA
	if pr.MergeCommitSHA != "" && !git.IsValidSHAPattern(pr.MergeCommitSHA) {
		WarnAndNotice("PR #%d in %s has invalid MergeCommitSHA: %s", pr.Number, g, pr.MergeCommitSHA)
		pr.MergeCommitSHA = ""
	}
	if pr.Head.SHA != "" && !git.IsValidSHAPattern(pr.Head.SHA) {
		WarnAndNotice("PR #%d in %s has invalid HeadSHA: %s", pr.Number, g, pr.Head.SHA)
		pr.Head.SHA = ""
		valid = false
	}
	if pr.Base.SHA != "" && !git.IsValidSHAPattern(pr.Base.SHA) {
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
