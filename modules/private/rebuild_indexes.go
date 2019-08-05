// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"fmt"

	"code.gitea.io/gitea/modules/setting"
)

// RebuildRepoIndex rebuild a repository index
func RebuildRepoIndex(repoID int64) error {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/maint/rebuild-index/repo/%d", repoID)
	resp, err := newInternalRequest(reqURL, "POST").Response()
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Failed to rebuild indexes for repository: %s", decodeJSONError(resp).Err)
	}
	return nil
}
