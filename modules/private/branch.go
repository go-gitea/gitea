// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"crypto/tls"
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

	resp, err := newRequest(reqURL, "GET").SetTLSClientConfig(&tls.Config{
		InsecureSkipVerify: true,
	}).Response()
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
