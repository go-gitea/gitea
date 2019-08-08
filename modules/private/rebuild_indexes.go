// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"fmt"

	"code.gitea.io/gitea/modules/setting"
)

// RebuildRepoIndex rebuild a repository index
func RebuildRepoIndex(repoID int64) (bool, error) {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/maint/rebuild-repo-index/%d", repoID)
	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == 503 {
		// Server is too busy; back off a little
		return true, nil
	}

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return false, fmt.Errorf("Failed to rebuild indexes for repository: %s", decodeJSONError(resp).Err)
	}
	return false, nil
}

// RebuildIssueIndex rebuild issue index for a repo
func RebuildIssueIndex(repoID int64) (bool, error) {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/maint/rebuild-issue-index/%d", repoID)
	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == 503 {
		// Server is too busy; back off a little
		return true, nil
	}

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return false, fmt.Errorf("Failed to rebuild indexes for repository: %s", decodeJSONError(resp).Err)
	}
	return false, nil
}
