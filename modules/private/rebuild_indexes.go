// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"fmt"

	"code.gitea.io/gitea/modules/setting"
)

// RebuildRepoIndex rebuild a repository index
func RebuildRepoIndex() error {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/maint/rebuild-repo-index")
	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Failed to rebuild repository index: %s", decodeJSONError(resp).Err)
	}
	return nil
}

// RebuildIssueIndex rebuild issue index for a repo
func RebuildIssueIndex() error {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/maint/rebuild-issue-index")
	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Failed to rebuild issue index: %s", decodeJSONError(resp).Err)
	}
	return nil
}
