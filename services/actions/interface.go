// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import "code.gitea.io/gitea/services/context"

type API interface {
	ListActionsSecrets(*context.APIContext)
	CreateOrUpdateSecret(*context.APIContext)
	DeleteSecret(*context.APIContext)
	ListVariables(*context.APIContext)
	GetVariable(*context.APIContext)
	DeleteVariable(*context.APIContext)
	CreateVariable(*context.APIContext)
	UpdateVariable(*context.APIContext)
	GetRegistrationToken(*context.APIContext)
}
