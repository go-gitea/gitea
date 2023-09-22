// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pam_test

import (
	auth_model "code.gitea.io/gitea/internal/models/auth"
	"code.gitea.io/gitea/internal/services/auth"
	"code.gitea.io/gitea/internal/services/auth/source/pam"
)

// This test file exists to assert that our Source exposes the interfaces that we expect
// It tightly binds the interfaces and implementation without breaking go import cycles

type sourceInterface interface {
	auth.PasswordAuthenticator
	auth_model.Config
	auth_model.SourceSettable
}

var _ (sourceInterface) = &pam.Source{}
