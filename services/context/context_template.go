// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"time"
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
