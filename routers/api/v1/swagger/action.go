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
