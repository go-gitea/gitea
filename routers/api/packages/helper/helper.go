// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package helper

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// LogAndProcessError logs an error and calls a custom callback with the processed error message.
// If the error is an InternalServerError the message is stripped if the user is not an admin.
func LogAndProcessError(ctx *context.Context, status int, obj any, cb func(string)) {
	var message string
	if err, ok := obj.(error); ok {
		message = err.Error()
	} else if obj != nil {
		message = fmt.Sprintf("%s", obj)
	}
	if status == http.StatusInternalServerError {
		log.ErrorWithSkip(1, message)

		if setting.IsProd && (ctx.Doer == nil || !ctx.Doer.IsAdmin) {
			message = ""
		}
	} else {
		log.Debug(message)
	}

	if cb != nil {
		cb(message)
	}
}
