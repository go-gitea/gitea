// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/setting"
)

// RestoreParams structure holds a data for restore repository
type RestoreParams struct {
	RepoDir    string
	OwnerName  string
	RepoName   string
	Units      []string
	Validation bool
}

// RestoreRepo calls the internal RestoreRepo function
func RestoreRepo(ctx context.Context, repoDir, ownerName, repoName string, units []string, validation bool) ResponseExtra {
	reqURL := setting.LocalURL + "api/internal/restore_repo"

	req := newInternalRequest(ctx, reqURL, "POST", RestoreParams{
		RepoDir:    repoDir,
		OwnerName:  ownerName,
		RepoName:   repoName,
		Units:      units,
		Validation: validation,
	})
	req.SetTimeout(3*time.Second, 0) // since the request will spend much time, don't timeout
	return requestJSONClientMsg(req, fmt.Sprintf("Restore repo %s/%s successfully", ownerName, repoName))
}
