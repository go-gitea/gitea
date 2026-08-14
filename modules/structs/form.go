// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"net/http"

	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/translation/i18n"

	"gitea.com/go-chi/binding"
)

// ValidateContext is a special context for form validation middleware
type ValidateContext struct {
	Locale i18n.LocaleTranslation
	Data   reqctx.ContextData
	Req    *http.Request
	Resp   http.ResponseWriter
}

type FormDefaultValidator struct{}

func (FormDefaultValidator) Validate(ctx *ValidateContext, errs binding.Errors) binding.Errors {
	// this default validator only needs to return the errs as is because the "binding" function has already validated
	return errs
}
