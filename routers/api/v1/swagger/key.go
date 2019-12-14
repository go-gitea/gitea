// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// PublicKey
// swagger:response PublicKey
type swaggerResponsePublicKey struct {
	// in:body
	Body api.PublicKey `json:"body"`
}

// PublicKeyList
// swagger:response PublicKeyList
type swaggerResponsePublicKeyList struct {
	// in:body
	Body []api.PublicKey `json:"body"`
}

// GPGKey
// swagger:response GPGKey
type swaggerResponseGPGKey struct {
	// in:body
	Body api.GPGKey `json:"body"`
}

// GPGKeyList
// swagger:response GPGKeyList
type swaggerResponseGPGKeyList struct {
	// in:body
	Body []api.GPGKey `json:"body"`
}

// DeployKey
// swagger:response DeployKey
type swaggerResponseDeployKey struct {
	// in:body
	Body api.DeployKey `json:"body"`
}

// DeployKeyList
// swagger:response DeployKeyList
type swaggerResponseDeployKeyList struct {
	// in:body
	Body []api.DeployKey `json:"body"`
}
