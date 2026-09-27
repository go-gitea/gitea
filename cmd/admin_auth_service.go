// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"

	auth_model "gitea.dev/models/auth"
	auth_service "gitea.dev/services/auth"
)

type authService struct {
	initDB            func(ctx context.Context) error
	createAuthSource  func(ctx context.Context, source *auth_model.Source) error
	getAuthSourceByID func(ctx context.Context, id int64) (*auth_model.Source, error)
	getAuthSourceOfType func(ctx context.Context, id int64, typ auth_model.Type) (*auth_model.Source, error)
	updateAuthSource  func(ctx context.Context, source *auth_model.Source) error
}

func newAuthService() *authService {
	return &authService{
		initDB: func(ctx context.Context) error {
			return initDB(ctx)
		},
		createAuthSource: func(ctx context.Context, source *auth_model.Source) error {
			return auth_service.CreateSource(ctx, source)
		},
		getAuthSourceByID: func(ctx context.Context, id int64) (*auth_model.Source, error) {
			return auth_model.GetSourceByID(ctx, id)
		},
		getAuthSourceOfType: func(ctx context.Context, id int64, typ auth_model.Type) (*auth_model.Source, error) {
			source, err := auth_model.GetSourceByID(ctx, id)
			if err != nil {
				return nil, err
			}
			if source.Type != typ {
				return nil, ErrAuthSourceTypeMismatch{id, source.Type, typ}
			}
			return source, nil
		},
		updateAuthSource: func(ctx context.Context, source *auth_model.Source) error {
			return auth_service.UpdateSource(ctx, source)
		},
	}
}

// ErrAuthSourceTypeMismatch represents a type mismatch error
type ErrAuthSourceTypeMismatch struct {
	ID         int64
	ActualType auth_model.Type
	ExpectedType auth_model.Type
}

func (err ErrAuthSourceTypeMismatch) Error() string {
	return "auth source type mismatch"
}

func IsErrAuthSourceTypeMismatch(err error) bool {
	_, ok := err.(ErrAuthSourceTypeMismatch)
	return ok
}