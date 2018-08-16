// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"encoding/json"
	"fmt"
	"net/url"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// GetRepository return the repository by its ID and a bool about if it's allowed to have PR
func GetRepository(repoID int64) (*models.Repository, bool, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/repository/%d", repoID)
	log.GitLogger.Trace("GetRepository: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, false, err
	}

	var repoInfo struct {
		Repository       *models.Repository
		AllowPullRequest bool
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return nil, false, err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return nil, false, fmt.Errorf("failed to retrieve repository: %s", decodeJSONError(resp).Err)
	}

	return repoInfo.Repository, repoInfo.AllowPullRequest, nil
}

// ActivePullRequest returns an active pull request if it exists
func ActivePullRequest(baseRepoID int64, headRepoID int64, baseBranch, headBranch string) (*models.PullRequest, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/active-pull-request?baseRepoID=%d&headRepoID=%d&baseBranch=%s&headBranch=%s", baseRepoID, headRepoID, url.QueryEscape(baseBranch), url.QueryEscape(headBranch))
	log.GitLogger.Trace("ActivePullRequest: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, err
	}

	var pr *models.PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("failed to retrieve pull request: %s", decodeJSONError(resp).Err)
	}

	return pr, nil
}
