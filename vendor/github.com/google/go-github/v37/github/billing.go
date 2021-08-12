// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// BillingService provides access to the billing related functions
// in the GitHub API.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/billing
type BillingService service

// ActionBilling represents a GitHub Action billing.
type ActionBilling struct {
	TotalMinutesUsed     int                  `json:"total_minutes_used"`
	TotalPaidMinutesUsed int                  `json:"total_paid_minutes_used"`
	IncludedMinutes      int                  `json:"included_minutes"`
	MinutesUsedBreakdown MinutesUsedBreakdown `json:"minutes_used_breakdown"`
}

type MinutesUsedBreakdown struct {
	Ubuntu  int `json:"UBUNTU"`
	MacOS   int `json:"MACOS"`
	Windows int `json:"WINDOWS"`
}

// PackageBilling represents a GitHub Package billing.
type PackageBilling struct {
	TotalGigabytesBandwidthUsed     int `json:"total_gigabytes_bandwidth_used"`
	TotalPaidGigabytesBandwidthUsed int `json:"total_paid_gigabytes_bandwidth_used"`
	IncludedGigabytesBandwidth      int `json:"included_gigabytes_bandwidth"`
}

// StorageBilling represents a GitHub Storage billing.
type StorageBilling struct {
	DaysLeftInBillingCycle       int `json:"days_left_in_billing_cycle"`
	EstimatedPaidStorageForMonth int `json:"estimated_paid_storage_for_month"`
	EstimatedStorageForMonth     int `json:"estimated_storage_for_month"`
}

// GetActionsBillingOrg returns the summary of the free and paid GitHub Actions minutes used for an Org.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/billing#get-github-actions-billing-for-an-organization
func (s *BillingService) GetActionsBillingOrg(ctx context.Context, org string) (*ActionBilling, *Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/billing/actions", org)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}
	actionsOrgBilling := new(ActionBilling)
	resp, err := s.client.Do(ctx, req, actionsOrgBilling)
	return actionsOrgBilling, resp, err
}

// GetPackagesBillingOrg returns the free and paid storage used for GitHub Packages in gigabytes for an Org.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/billing#get-github-packages-billing-for-an-organization
func (s *BillingService) GetPackagesBillingOrg(ctx context.Context, org string) (*PackageBilling, *Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/billing/packages", org)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}
	packagesOrgBilling := new(PackageBilling)
	resp, err := s.client.Do(ctx, req, packagesOrgBilling)
	return packagesOrgBilling, resp, err
}

// GetStorageBillingOrg returns the estimated paid and estimated total storage used for GitHub Actions
// and GitHub Packages in gigabytes for an Org.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/billing#get-shared-storage-billing-for-an-organization
func (s *BillingService) GetStorageBillingOrg(ctx context.Context, org string) (*StorageBilling, *Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/billing/shared-storage", org)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}
	storageOrgBilling := new(StorageBilling)
	resp, err := s.client.Do(ctx, req, storageOrgBilling)
	return storageOrgBilling, resp, err
}

// GetActionsBillingUser returns the summary of the free and paid GitHub Actions minutes used for a user.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/billing#get-github-actions-billing-for-a-user
func (s *BillingService) GetActionsBillingUser(ctx context.Context, user string) (*ActionBilling, *Response, error) {
	u := fmt.Sprintf("users/%v/settings/billing/actions", user)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}
	actionsUserBilling := new(ActionBilling)
	resp, err := s.client.Do(ctx, req, actionsUserBilling)
	return actionsUserBilling, resp, err
}

// GetPackagesBillingUser returns the free and paid storage used for GitHub Packages in gigabytes for a user.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/billing#get-github-packages-billing-for-an-organization
func (s *BillingService) GetPackagesBillingUser(ctx context.Context, user string) (*PackageBilling, *Response, error) {
	u := fmt.Sprintf("users/%v/settings/billing/packages", user)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}
	packagesUserBilling := new(PackageBilling)
	resp, err := s.client.Do(ctx, req, packagesUserBilling)
	return packagesUserBilling, resp, err
}

// GetStorageBillingUser returns the estimated paid and estimated total storage used for GitHub Actions
// and GitHub Packages in gigabytes for a user.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/billing#get-shared-storage-billing-for-a-user
func (s *BillingService) GetStorageBillingUser(ctx context.Context, user string) (*StorageBilling, *Response, error) {
	u := fmt.Sprintf("users/%v/settings/billing/shared-storage", user)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}
	storageUserBilling := new(StorageBilling)
	resp, err := s.client.Do(ctx, req, storageUserBilling)
	return storageUserBilling, resp, err
}
