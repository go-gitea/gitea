// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import api "gitea.dev/modules/structs"

// swagger:response StorageQuota
type swaggerResponseStorageQuota struct {
	// in:body
	Body api.StorageQuota `json:"body"`
}
