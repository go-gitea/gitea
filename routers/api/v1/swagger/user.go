// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	api "code.gitea.io/sdk/gitea"
)

// swagger:response User
type swaggerResponseUser struct {
	// in:body
	Body api.User `json:"body"`
}

// swagger:response UserList
type swaggerResponseUserList struct {
	// in:body
	Body []api.User `json:"body"`
}

// swagger:response EmailList
type swaggerResponseEmailList struct {
	// in:body
	Body []api.Email `json:"body"`
}

// swagger:model EditUserOption
type swaggerModelEditUserOption struct {
	// in:body
	Options api.EditUserOption
}
