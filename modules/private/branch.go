// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"encoding/json"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// GetProtectedBranchBy get protected branch information
func GetProtectedBranchBy(repoID int64, branchName string) (*models.ProtectedBranch, error) {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/branch/%d/%s", repoID, branchName)
	log.GitLogger.Trace("GetProtectedBranchBy: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, err
	}

	var branch models.ProtectedBranch
	if err := json.NewDecoder(resp.Body).Decode(&branch); err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Failed to update public key: %s", decodeJSONError(resp).Err)
	}

	return &branch, nil
}

// CanUserPush returns if user can push
func CanUserPush(protectedBranchID, userID int64) (bool, error) {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/protectedbranch/%d/%d", protectedBranchID, userID)
	log.GitLogger.Trace("CanUserPush: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return false, err
	}

	var canPush = make(map[string]interface{})
	if err := json.NewDecoder(resp.Body).Decode(&canPush); err != nil {
		return false, err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return false, fmt.Errorf("Failed to retrieve push user: %s", decodeJSONError(resp).Err)
	}

	return canPush["can_push"].(bool), nil
}
