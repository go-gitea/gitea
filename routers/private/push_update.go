// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"encoding/json"
	"strings"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"

	macaron "gopkg.in/macaron.v1"
)

// PushUpdate update public key updates
func PushUpdate(ctx *macaron.Context) {
	var opt models.PushUpdateOptions
	if err := json.NewDecoder(ctx.Req.Request.Body).Decode(&opt); err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	branch := strings.TrimPrefix(opt.RefFullName, git.BranchPrefix)
	if len(branch) == 0 || opt.PusherID <= 0 {
		ctx.Error(404)
		log.Trace("PushUpdate: branch or secret is empty, or pusher ID is not valid")
		return
	}

	err := models.PushUpdate(branch, opt)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(404)
		} else {
			ctx.JSON(500, map[string]interface{}{
				"err": err.Error(),
			})
		}
		return
	}
	ctx.Status(202)
}
