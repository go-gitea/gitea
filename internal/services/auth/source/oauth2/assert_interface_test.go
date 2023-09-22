// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package oauth2_test

import (
	auth_model "code.gitea.io/gitea/internal/models/auth"
	"code.gitea.io/gitea/internal/services/auth"
	"code.gitea.io/gitea/internal/services/auth/source/oauth2"
)

// This test file exists to assert that our Source exposes the interfaces that we expect
// It tightly binds the interfaces and implementation without breaking go import cycles

type sourceInterface interface {
	auth_model.Config
	auth_model.SourceSettable
	auth_model.RegisterableSource
	auth.PasswordAuthenticator
}

var _ (sourceInterface) = &oauth2.Source{}
