// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ListHookDeliveries lists webhook deliveries for a webhook configured in an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/orgs#list-deliveries-for-an-organization-webhook
func (s *OrganizationsService) ListHookDeliveries(ctx context.Context, org string, id int64, opts *ListCursorOptions) ([]*HookDelivery, *Response, error) {
	u := fmt.Sprintf("orgs/%v/hooks/%v/deliveries", org, id)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	deliveries := []*HookDelivery{}
	resp, err := s.client.Do(ctx, req, &deliveries)
	if err != nil {
		return nil, resp, err
	}

	return deliveries, resp, nil
}

// GetHookDelivery returns a delivery for a webhook configured in an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/orgs#get-a-webhook-delivery-for-an-organization-webhook
func (s *OrganizationsService) GetHookDelivery(ctx context.Context, owner string, hookID, deliveryID int64) (*HookDelivery, *Response, error) {
	u := fmt.Sprintf("orgs/%v/hooks/%v/deliveries/%v", owner, hookID, deliveryID)
	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	h := new(HookDelivery)
	resp, err := s.client.Do(ctx, req, h)
	if err != nil {
		return nil, resp, err
	}

	return h, resp, nil
}

// RedeliverHookDelivery redelivers a delivery for a webhook configured in an organization.
//
// GitHub API docs: https://docs.github.com/en/rest/reference/orgs#redeliver-a-delivery-for-an-organization-webhook
func (s *OrganizationsService) RedeliverHookDelivery(ctx context.Context, owner string, hookID, deliveryID int64) (*HookDelivery, *Response, error) {
	u := fmt.Sprintf("orgs/%v/hooks/%v/deliveries/%v/attempts", owner, hookID, deliveryID)
	req, err := s.client.NewRequest("POST", u, nil)
	if err != nil {
		return nil, nil, err
	}

	h := new(HookDelivery)
	resp, err := s.client.Do(ctx, req, h)
	if err != nil {
		return nil, resp, err
	}

	return h, resp, nil
}
