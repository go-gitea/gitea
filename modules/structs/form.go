// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"codenest.dev/modules/translation/i18n"
	"codenest.dev/modules/web/binding"
)

// ValidateContext is a special context for form validation middleware
type ValidateContext struct {
	Locale i18n.LocaleTranslation
}

type FormDefaultValidator struct{}

func (FormDefaultValidator) Validate(ctx *ValidateContext, errs binding.Errors) binding.Errors {
	// this default validator only needs to return the errs as is because the "binding" function has already validated
	return errs
}
