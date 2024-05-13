// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import api "code.gitea.io/gitea/modules/structs"

// SecretList
// swagger:response SecretList
type swaggerResponseSecretList struct {
	// in:body
	Body []api.Secret `json:"body"`
}

// Secret
// swagger:response Secret
type swaggerResponseSecret struct {
	// in:body
	Body api.Secret `json:"body"`
}

// ActionVariable
// swagger:response ActionVariable
type swaggerResponseActionVariable struct {
	// in:body
	Body api.ActionVariable `json:"body"`
}

// VariableList
// swagger:response VariableList
type swaggerResponseVariableList struct {
	// in:body
	Body []api.ActionVariable `json:"body"`
}
