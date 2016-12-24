// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"code.gitea.io/gitea/models"
)

// APIOrganization contains organization and team
type APIOrganization struct {
	Organization *models.User
	Team         *models.Team
}
