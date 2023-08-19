// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package nuget

import (
	"fmt"
	"net/url"
)

type nextOptions struct {
	Path  string
	Query url.Values
}

type linkBuilder struct {
	Base string
	Next *nextOptions
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

func (l *linkBuilder) GetNextURL() string {
	u, _ := url.Parse(l.Base)
	u = u.JoinPath(l.Next.Path)
	q := u.Query()
	for k, vs := range l.Next.Query {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
