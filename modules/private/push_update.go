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

// PushUpdate update publick key updates
func PushUpdate(opt models.PushUpdateOptions) error {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + "api/internal/push/update"
	log.GitLogger.Trace("PushUpdate: %s", reqURL)

	body, err := json.Marshal(&opt)
	if err != nil {
		return err
	}

	resp, err := newInternalRequest(reqURL, "POST").Body(body).Response()
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Failed to update public key: %s", decodeJSONError(resp).Err)
	}

	return nil
}
