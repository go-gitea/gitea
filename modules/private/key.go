// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"
	"fmt"

	"gitea.dev/modules/setting"
)

func UpdatePublicKeyLastUsed(ctx context.Context, keyID, repoID int64) error {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/ssh/%d/update/%d", keyID, repoID)
	req := newInternalRequestAPI(ctx, reqURL, "POST")
	_, extra := requestJSONResp(req, &ResponseText{})
	return extra.Error
}

func AuthorizedPublicKeyForSSH(ctx context.Context, content string) (*ResponseText, ResponseExtra) {
	reqURL := setting.LocalURL + "api/internal/ssh/authorized_keys"
	req := newInternalRequestAPI(ctx, reqURL, "POST")
	req.Param("content", content)
	return requestJSONResp(req, &ResponseText{})
}
