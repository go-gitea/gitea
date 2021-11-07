// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models/packages"
	api "code.gitea.io/gitea/modules/structs"
)

// ToPackage convert a packages.PackageDescriptor to api.Package
func ToPackage(pd *packages.PackageDescriptor) *api.Package {
	return &api.Package{
		ID:        pd.Version.ID,
		Creator:   ToUser(pd.Creator, nil),
		Type:      string(pd.Package.Type),
		Name:      pd.Package.Name,
		Version:   pd.Version.Version,
		CreatedAt: pd.Version.CreatedUnix.AsTime(),
	}
}

// ToPackageFile converts packages.PackageFile to api.PackageFile
func ToPackageFile(pf *packages.PackageFile, pb *packages.PackageBlob) *api.PackageFile {
	return &api.PackageFile{
		ID:         pf.ID,
		Size:       pb.Size,
		Name:       pf.Name,
		HashMD5:    pb.HashMD5,
		HashSHA1:   pb.HashSHA1,
		HashSHA256: pb.HashSHA256,
		HashSHA512: pb.HashSHA512,
	}
}
