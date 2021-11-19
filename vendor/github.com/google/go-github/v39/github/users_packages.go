// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// List the packages for a user. Passing the empty string for "user" will
// list packages for the authenticated user.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#list-packages-for-the-authenticated-users-namespace
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#list-packages-for-a-user
func (s *UsersService) ListPackages(ctx context.Context, user string, opts *PackageListOptions) ([]*Package, *Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("user/%v/packages", user)
	} else {
		u = "user/packages"
	}
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var packages []*Package
	resp, err := s.client.Do(ctx, req, &packages)
	if err != nil {
		return nil, resp, err
	}

	return packages, resp, nil
}

// Get a package by name for a user. Passing the empty string for "user" will
// get the package for the authenticated user.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#get-a-package-for-the-authenticated-user
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#get-a-package-for-a-user
func (s *UsersService) GetPackage(ctx context.Context, user, packageType, packageName string) (*Package, *Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v", user, packageType, packageName)
	} else {
		u = fmt.Sprintf("user/packages/%v/%v", packageType, packageName)
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var pack *Package
	resp, err := s.client.Do(ctx, req, &pack)
	if err != nil {
		return nil, resp, err
	}

	return pack, resp, nil
}

// Delete a package from a user. Passing the empty string for "user" will
// delete the package for the authenticated user.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#delete-a-package-for-the-authenticated-user
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#delete-a-package-for-a-user
func (s *UsersService) DeletePackage(ctx context.Context, user, packageType, packageName string) (*Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v", user, packageType, packageName)
	} else {
		u = fmt.Sprintf("user/packages/%v/%v", packageType, packageName)
	}

	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(ctx, req, nil)
}

// Restore a package to a user. Passing the empty string for "user" will
// restore the package for the authenticated user.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#restore-a-package-for-the-authenticated-user
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#restore-a-package-for-a-user
func (s *UsersService) RestorePackage(ctx context.Context, user, packageType, packageName string) (*Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v/restore", user, packageType, packageName)
	} else {
		u = fmt.Sprintf("user/packages/%v/%v/restore", packageType, packageName)
	}

	req, err := s.client.NewRequest("POST", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(ctx, req, nil)
}

// Get all versions of a package for a user. Passing the empty string for "user" will
// get versions for the authenticated user.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#get-all-package-versions-for-a-package-owned-by-the-authenticated-user
// GitHub API docs: https://docs.github.com/en/rest/reference/users#delete-an-email-address-for-the-authenticated-user
func (s *UsersService) PackageGetAllVersions(ctx context.Context, user, packageType, packageName string) ([]*PackageVersion, *Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v/versions", user, packageType, packageName)
	} else {
		u = fmt.Sprintf("user/packages/%v/%v/versions", packageType, packageName)
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var versions []*PackageVersion
	resp, err := s.client.Do(ctx, req, &versions)
	if err != nil {
		return nil, resp, err
	}

	return versions, resp, nil
}

// Get a specific version of a package for a user. Passing the empty string for "user" will
// get the version for the authenticated user.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#get-a-package-version-for-the-authenticated-user
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#get-a-package-version-for-a-user
func (s *UsersService) PackageGetVersion(ctx context.Context, user, packageType, packageName string, packageVersionID int64) (*PackageVersion, *Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v/versions/%v", user, packageType, packageName, packageVersionID)
	} else {
		u = fmt.Sprintf("user/packages/%v/%v/versions/%v", packageType, packageName, packageVersionID)
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var version *PackageVersion
	resp, err := s.client.Do(ctx, req, &version)
	if err != nil {
		return nil, resp, err
	}

	return version, resp, nil
}

// Delete a package version for a user. Passing the empty string for "user" will
// delete the version for the authenticated user.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#delete-a-package-version-for-the-authenticated-user
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#delete-package-version-for-a-user
func (s *UsersService) PackageDeleteVersion(ctx context.Context, user, packageType, packageName string, packageVersionID int64) (*Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v/versions/%v", user, packageType, packageName, packageVersionID)
	} else {
		u = fmt.Sprintf("user/packages/%v/%v/versions/%v", packageType, packageName, packageVersionID)
	}

	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(ctx, req, nil)
}

// Restore a package version to a user. Passing the empty string for "user" will
// restore the version for the authenticated user.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#restore-a-package-version-for-the-authenticated-user
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#restore-package-version-for-a-user
func (s *UsersService) PackageRestoreVersion(ctx context.Context, user, packageType, packageName string, packageVersionID int64) (*Response, error) {
	var u string
	if user != "" {
		u = fmt.Sprintf("users/%v/packages/%v/%v/versions/%v/restore", user, packageType, packageName, packageVersionID)
	} else {
		u = fmt.Sprintf("user/packages/%v/%v/versions/%v/restore", packageType, packageName, packageVersionID)
	}

	req, err := s.client.NewRequest("POST", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(ctx, req, nil)
}
