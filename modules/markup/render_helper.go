// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"
	"html/template"
)

// ProcessorHelper is a helper for the rendering processors (it could be renamed to RenderHelper in the future).
// The main purpose of this helper is to decouple some functions which are not directly available in this package.
type ProcessorHelper struct {
	IsUsernameMentionable func(ctx context.Context, username string) bool

	ElementDir string // the direction of the elements, eg: "ltr", "rtl", "auto", default to no direction attribute

	RenderRepoFileCodePreview func(ctx context.Context, options RenderCodePreviewOptions) (template.HTML, error)
}

var DefaultProcessorHelper ProcessorHelper
