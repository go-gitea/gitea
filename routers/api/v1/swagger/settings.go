// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import api "code.gitea.io/gitea/modules/structs"

// GeneralRepoSettings
// swagger:response GeneralRepoSettings
type swaggerResponseGeneralRepoSettings struct {
	// in:body
	Body api.GeneralRepoSettings `json:"body"`
}

// GeneralUISettings
// swagger:response GeneralUISettings
type swaggerResponseGeneralUISettings struct {
	// in:body
	Body api.GeneralUISettings `json:"body"`
}
