// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import "code.gitea.io/gitea/services/context"

// API for actions of a repository or organization
type API interface {
	// ListActionsSecrets list secrets
	ListActionsSecrets(*context.APIContext)
	// CreateOrUpdateSecret create or update a secret
	CreateOrUpdateSecret(*context.APIContext)
	// DeleteSecret delete a secret
	DeleteSecret(*context.APIContext)
	// ListVariables list variables
	ListVariables(*context.APIContext)
	// GetVariable get a variable
	GetVariable(*context.APIContext)
	// DeleteVariable delete a variable
	DeleteVariable(*context.APIContext)
	// CreateVariable create a variable
	CreateVariable(*context.APIContext)
	// UpdateVariable update a variable
	UpdateVariable(*context.APIContext)
	// GetRegistrationToken get registration token
	GetRegistrationToken(*context.APIContext)
}
