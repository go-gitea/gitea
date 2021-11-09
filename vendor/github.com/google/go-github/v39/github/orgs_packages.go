// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// List the packages for an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#list-packages-for-an-organization
func (s *OrganizationsService) ListPackages(ctx context.Context, org string, opts *PackageListOptions) ([]*Package, *Response, error) {
	u := fmt.Sprintf("orgs/%v/packages", org)
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

// Get a package by name from an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#get-a-package-for-an-organization
func (s *OrganizationsService) GetPackage(ctx context.Context, org, packageType, packageName string) (*Package, *Response, error) {
	u := fmt.Sprintf("orgs/%v/packages/%v/%v", org, packageType, packageName)
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

// Delete a package from an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#delete-a-package-for-an-organization
func (s *OrganizationsService) DeletePackage(ctx context.Context, org, packageType, packageName string) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/packages/%v/%v", org, packageType, packageName)
	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(ctx, req, nil)
}

// Restore a package to an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#restore-a-package-for-an-organization
func (s *OrganizationsService) RestorePackage(ctx context.Context, org, packageType, packageName string) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/packages/%v/%v/restore", org, packageType, packageName)
	req, err := s.client.NewRequest("POST", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(ctx, req, nil)
}

// Get all versions of a package in an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#get-all-package-versions-for-a-package-owned-by-an-organization
func (s *OrganizationsService) PackageGetAllVersions(ctx context.Context, org, packageType, packageName string, opts *PackageListOptions) ([]*PackageVersion, *Response, error) {
	u := fmt.Sprintf("orgs/%v/packages/%v/%v/versions", org, packageType, packageName)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
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

// Get a specific version of a package in an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#get-a-package-version-for-an-organization
func (s *OrganizationsService) PackageGetVersion(ctx context.Context, org, packageType, packageName string, packageVersionID int64) (*PackageVersion, *Response, error) {
	u := fmt.Sprintf("orgs/%v/packages/%v/%v/versions/%v", org, packageType, packageName, packageVersionID)
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

// Delete a package version from an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#delete-package-version-for-an-organization
func (s *OrganizationsService) PackageDeleteVersion(ctx context.Context, org, packageType, packageName string, packageVersionID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/packages/%v/%v/versions/%v", org, packageType, packageName, packageVersionID)
	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(ctx, req, nil)
}

// Restore a package version to an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/packages#restore-package-version-for-an-organization
func (s *OrganizationsService) PackageRestoreVersion(ctx context.Context, org, packageType, packageName string, packageVersionID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/packages/%v/%v/versions/%v/restore", org, packageType, packageName, packageVersionID)
	req, err := s.client.NewRequest("POST", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(ctx, req, nil)
}
