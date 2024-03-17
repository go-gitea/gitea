// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"fmt"
	"net/http"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	webhook_module "code.gitea.io/gitea/modules/webhook"
)

type (
	// PackagistPayload represents
	PackagistPayload struct {
		PackagistRepository struct {
			URL string `json:"url"`
		} `json:"repository"`
	}

	// PackagistMeta contains the metadata for the webhook
	PackagistMeta struct {
		Username   string `json:"username"`
		APIToken   string `json:"api_token"`
		PackageURL string `json:"package_url"`
	}
)

// GetPackagistHook returns packagist metadata
func GetPackagistHook(w *webhook_model.Webhook) *PackagistMeta {
	s := &PackagistMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetPackagistHook(%d): %v", w.ID, err)
	}
	return s
}

// Create implements PayloadConverter Create method
func (pc packagistConverter) Create(_ *api.CreatePayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Delete implements PayloadConverter Delete method
func (pc packagistConverter) Delete(_ *api.DeletePayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Fork implements PayloadConverter Fork method
func (pc packagistConverter) Fork(_ *api.ForkPayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Push implements PayloadConverter Push method
// https://packagist.org/about
func (pc packagistConverter) Push(_ *api.PushPayload) (PackagistPayload, error) {
	return PackagistPayload{
		PackagistRepository: struct {
			URL string `json:"url"`
		}{
			URL: pc.PackageURL,
		},
	}, nil
}

// Issue implements PayloadConverter Issue method
func (pc packagistConverter) Issue(_ *api.IssuePayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// IssueComment implements PayloadConverter IssueComment method
func (pc packagistConverter) IssueComment(_ *api.IssueCommentPayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// PullRequest implements PayloadConverter PullRequest method
func (pc packagistConverter) PullRequest(_ *api.PullRequestPayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Review implements PayloadConverter Review method
func (pc packagistConverter) Review(_ *api.PullRequestPayload, _ webhook_module.HookEventType) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Repository implements PayloadConverter Repository method
func (pc packagistConverter) Repository(_ *api.RepositoryPayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Wiki implements PayloadConverter Wiki method
func (pc packagistConverter) Wiki(_ *api.WikiPayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Release implements PayloadConverter Release method
func (pc packagistConverter) Release(_ *api.ReleasePayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

func (pc packagistConverter) Package(_ *api.PackagePayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

type packagistConverter struct {
	PackageURL string
}

var _ payloadConverter[PackagistPayload] = packagistConverter{}

func newPackagistRequest(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	meta := &PackagistMeta{}
	if err := json.Unmarshal([]byte(w.Meta), meta); err != nil {
		return nil, nil, fmt.Errorf("newpackagistRequest meta json: %w", err)
	}
	pc := packagistConverter{
		PackageURL: meta.PackageURL,
	}
	return newJSONRequest(pc, w, t, true)
}
