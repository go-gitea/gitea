package graphql

import (
	"context"
	"errors"

	"github.com/vektah/gqlparser/v2/gqlerror"
)

type ErrorPresenterFunc func(ctx context.Context, err error) *gqlerror.Error

func DefaultErrorPresenter(ctx context.Context, err error) *gqlerror.Error {
	return err.(*gqlerror.Error)
}

func ErrorOnPath(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	var gqlerr *gqlerror.Error
	if errors.As(err, &gqlerr) {
		if gqlerr.Path == nil {
			gqlerr.Path = GetPath(ctx)
		}
		return gqlerr
	}
	return gqlerror.WrapPath(GetPath(ctx), err)
}
