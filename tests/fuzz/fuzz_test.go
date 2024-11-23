// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package fuzz

import (
	"bytes"
	"context"
	"io"
	"testing"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
)

func newFuzzRenderContext() *markup.RenderContext {
	return markup.NewRenderContext(context.Background()).
		WithLinks(markup.Links{Base: "https://example.com/go-gitea/gitea"}).
		WithMetas(map[string]string{"user": "go-gitea", "repo": "gitea"})
}

func FuzzMarkdownRenderRaw(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		setting.AppURL = "http://localhost:3000/"
		markdown.RenderRaw(newFuzzRenderContext(), bytes.NewReader(data), io.Discard)
	})
}

func FuzzMarkupPostProcess(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		setting.AppURL = "http://localhost:3000/"
		markup.PostProcess(newFuzzRenderContext(), bytes.NewReader(data), io.Discard)
	})
}
