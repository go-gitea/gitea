// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/modules/setting"
)

// UpdatePublicKeyInRepo update public key and if necessary deploy key updates
func UpdatePublicKeyInRepo(ctx context.Context, keyID, repoID int64) error {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/ssh/%d/update/%d", keyID, repoID)
	req := newInternalRequest(ctx, reqURL, "POST")
	_, extra := requestJSONResp(req, &responseText{})
	return extra.Error
}

// AuthorizedPublicKeyByContent searches content as prefix (leak e-mail part)
// and returns public key found.
func AuthorizedPublicKeyByContent(ctx context.Context, content string) (string, ResponseExtra) {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + "api/internal/ssh/authorized_keys"
	req := newInternalRequest(ctx, reqURL, "POST")
	req.Param("content", content)
	resp, extra := requestJSONResp(req, &responseText{})
	return resp.Text, extra
}
