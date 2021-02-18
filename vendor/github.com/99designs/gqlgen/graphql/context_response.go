package graphql

import (
	"context"
	"fmt"
	"sync"

	"github.com/vektah/gqlparser/v2/gqlerror"
)

type responseContext struct {
	errorPresenter ErrorPresenterFunc
	recover        RecoverFunc

	errors   gqlerror.List
	errorsMu sync.Mutex

	extensions   map[string]interface{}
	extensionsMu sync.Mutex
}

const resultCtx key = "result_context"

func getResponseContext(ctx context.Context) *responseContext {
	val, ok := ctx.Value(resultCtx).(*responseContext)
	if !ok {
		panic("missing response context")
	}
	return val
}

func WithResponseContext(ctx context.Context, presenterFunc ErrorPresenterFunc, recoverFunc RecoverFunc) context.Context {
	return context.WithValue(ctx, resultCtx, &responseContext{
		errorPresenter: presenterFunc,
		recover:        recoverFunc,
	})
}

// AddErrorf writes a formatted error to the client, first passing it through the error presenter.
func AddErrorf(ctx context.Context, format string, args ...interface{}) {
	AddError(ctx, fmt.Errorf(format, args...))
}

// AddError sends an error to the client, first passing it through the error presenter.
func AddError(ctx context.Context, err error) {
	c := getResponseContext(ctx)

	c.errorsMu.Lock()
	defer c.errorsMu.Unlock()

	c.errors = append(c.errors, c.errorPresenter(ctx, ErrorOnPath(ctx, err)))
}

func Recover(ctx context.Context, err interface{}) (userMessage error) {
	c := getResponseContext(ctx)
	return ErrorOnPath(ctx, c.recover(ctx, err))
}

// HasFieldError returns true if the given field has already errored
func HasFieldError(ctx context.Context, rctx *FieldContext) bool {
	c := getResponseContext(ctx)

	c.errorsMu.Lock()
	defer c.errorsMu.Unlock()

	if len(c.errors) == 0 {
		return false
	}

	path := rctx.Path()
	for _, err := range c.errors {
		if equalPath(err.Path, path) {
			return true
		}
	}
	return false
}

// GetFieldErrors returns a list of errors that occurred in the given field
func GetFieldErrors(ctx context.Context, rctx *FieldContext) gqlerror.List {
	c := getResponseContext(ctx)

	c.errorsMu.Lock()
	defer c.errorsMu.Unlock()

	if len(c.errors) == 0 {
		return nil
	}

	path := rctx.Path()
	var errs gqlerror.List
	for _, err := range c.errors {
		if equalPath(err.Path, path) {
			errs = append(errs, err)
		}
	}
	return errs
}

func GetErrors(ctx context.Context) gqlerror.List {
	resCtx := getResponseContext(ctx)
	resCtx.errorsMu.Lock()
	defer resCtx.errorsMu.Unlock()

	if len(resCtx.errors) == 0 {
		return nil
	}

	errs := resCtx.errors
	cpy := make(gqlerror.List, len(errs))
	for i := range errs {
		errCpy := *errs[i]
		cpy[i] = &errCpy
	}
	return cpy
}

// RegisterExtension allows you to add a new extension into the graphql response
func RegisterExtension(ctx context.Context, key string, value interface{}) {
	c := getResponseContext(ctx)
	c.extensionsMu.Lock()
	defer c.extensionsMu.Unlock()

	if c.extensions == nil {
		c.extensions = make(map[string]interface{})
	}

	if _, ok := c.extensions[key]; ok {
		panic(fmt.Errorf("extension already registered for key %s", key))
	}

	c.extensions[key] = value
}

// GetExtensions returns any extensions registered in the current result context
func GetExtensions(ctx context.Context) map[string]interface{} {
	ext := getResponseContext(ctx).extensions
	if ext == nil {
		return map[string]interface{}{}
	}

	return ext
}

func GetExtension(ctx context.Context, name string) interface{} {
	ext := getResponseContext(ctx).extensions
	if ext == nil {
		return nil
	}

	return ext[name]
}
