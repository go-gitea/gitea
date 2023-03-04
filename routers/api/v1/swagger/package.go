// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
)

// Package
// swagger:response Package
type swaggerResponsePackage struct {
	// in:body
	Body api.Package `json:"body"`
}

// PackageList
// swagger:response PackageList
type swaggerResponsePackageList struct {
	// in:body
	Body []api.Package `json:"body"`
}

// PackageFileList
// swagger:response PackageFileList
type swaggerResponsePackageFileList struct {
	// in:body
	Body []api.PackageFile `json:"body"`
}
