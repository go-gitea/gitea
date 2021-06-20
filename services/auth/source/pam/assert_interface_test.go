// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pam_test

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/pam"
)

type sourceInterface interface {
	auth.PasswordAuthenticator
	models.LoginConfig
	models.LoginSourceSettable
}

var _ (sourceInterface) = &pam.Source{}
