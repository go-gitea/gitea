// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2_test

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/services/auth/source/oauth2"
)

type sourceInterface interface {
	models.LoginConfig
	models.LoginSourceSettable
	models.RegisterableSource
}

var _ (sourceInterface) = &oauth2.Source{}
