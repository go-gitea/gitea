// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"fmt"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// InitWiki initwiki via repo id
func InitWiki(repoID int64) error {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/repositories/%d/wiki/init", repoID)
	log.GitLogger.Trace("InitWiki: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Failed to init wiki: %s", decodeJSONError(resp).Err)
	}

	return nil
}
