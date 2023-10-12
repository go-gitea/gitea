// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"errors"
	"time"

	"code.gitea.io/gitea/modules/log"
)

var _ context.Context = TemplateContext(nil)

func NewTemplateContext(ctx context.Context) TemplateContext {
	return TemplateContext{"_ctx": ctx}
}

func (c TemplateContext) parentContext() context.Context {
	return c["_ctx"].(context.Context)
}

func (c TemplateContext) Deadline() (deadline time.Time, ok bool) {
	return c.parentContext().Deadline()
}

func (c TemplateContext) Done() <-chan struct{} {
	return c.parentContext().Done()
}

func (c TemplateContext) Err() error {
	return c.parentContext().Err()
}

func (c TemplateContext) Value(key any) any {
	return c.parentContext().Value(key)
}

// DataRaceCheck checks whether the template context function "ctx()" returns the consistent context
// as the current template's rendering context (request context), to help to find data race issues as early as possible.
// When the code is proven to be correct and stable, this function should be removed.
func (c TemplateContext) DataRaceCheck(dataCtx context.Context) (string, error) {
	if c.parentContext() != dataCtx {
		log.Error("TemplateContext.DataRaceCheck: parent context mismatch\n%s", log.Stack(2))
		return "", errors.New("parent context mismatch")
	}
	return "", nil
}
