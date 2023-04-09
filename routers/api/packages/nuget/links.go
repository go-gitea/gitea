// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nuget

import (
	"fmt"
)

type linkBuilder struct {
	Base string
}

// GetRegistrationIndexURL builds the registration index url
func (l *linkBuilder) GetRegistrationIndexURL(id string) string {
	return fmt.Sprintf("%s/registration/%s/index.json", l.Base, id)
}

// GetRegistrationLeafURL builds the registration leaf url
func (l *linkBuilder) GetRegistrationLeafURL(id, version string) string {
	return fmt.Sprintf("%s/registration/%s/%s.json", l.Base, id, version)
}

// GetPackageDownloadURL builds the download url
func (l *linkBuilder) GetPackageDownloadURL(id, version string) string {
	return fmt.Sprintf("%s/package/%s/%s/%s.%s.nupkg", l.Base, id, version, id, version)
}

// GetPackageMetadataURL builds the package metadata url
func (l *linkBuilder) GetPackageMetadataURL(id, version string) string {
	return fmt.Sprintf("%s/Packages(Id='%s',Version='%s')", l.Base, id, version)
}
