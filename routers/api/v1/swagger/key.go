// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	api "code.gitea.io/sdk/gitea"
)

// swagger:response PublicKey
type swaggerResponsePublicKey struct {
	// in:body
	Body api.PublicKey `json:"body"`
}

// swagger:response PublicKeyList
type swaggerResponsePublicKeyList struct {
	// in:body
	Body []api.PublicKey `json:"body"`
}

// swagger:response GPGKey
type swaggerResponseGPGKey struct {
	// in:body
	Body api.GPGKey `json:"body"`
}

// swagger:response GPGKeyList
type swaggerResponseGPGKeyList struct {
	// in:body
	Body []api.GPGKey `json:"body"`
}

// swagger:response DeployKey
type swaggerResponseDeployKey struct {
	// in:body
	Body api.DeployKey `json:"body"`
}

// swagger:response DeployKeyList
type swaggerResponseDeployKeyList struct {
	// in:body
	Body []api.DeployKey `json:"body"`
}
