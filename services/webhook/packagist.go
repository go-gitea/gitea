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

type packagistConvertor struct {
	PackageURL string
}

// Create implements PayloadConvertor Create method
func (pc packagistConvertor) Create(_ *api.CreatePayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Delete implements PayloadConvertor Delete method
func (pc packagistConvertor) Delete(_ *api.DeletePayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Fork implements PayloadConvertor Fork method
func (pc packagistConvertor) Fork(_ *api.ForkPayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Push implements PayloadConvertor Push method
// https://packagist.org/about
func (pc packagistConvertor) Push(_ *api.PushPayload) (PackagistPayload, error) {
	return PackagistPayload{
		PackagistRepository: struct {
			URL string `json:"url"`
		}{
			URL: pc.PackageURL,
		},
	}, nil
}

// Issue implements PayloadConvertor Issue method
func (pc packagistConvertor) Issue(_ *api.IssuePayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (pc packagistConvertor) IssueComment(_ *api.IssueCommentPayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (pc packagistConvertor) PullRequest(_ *api.PullRequestPayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Review implements PayloadConvertor Review method
func (pc packagistConvertor) Review(_ *api.PullRequestPayload, _ webhook_module.HookEventType) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Repository implements PayloadConvertor Repository method
func (pc packagistConvertor) Repository(_ *api.RepositoryPayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Wiki implements PayloadConvertor Wiki method
func (pc packagistConvertor) Wiki(_ *api.WikiPayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

// Release implements PayloadConvertor Release method
func (pc packagistConvertor) Release(_ *api.ReleasePayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

func (pc packagistConvertor) Package(_ *api.PackagePayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

func (pc packagistConvertor) Status(_ *api.CommitStatusPayload) (PackagistPayload, error) {
	return PackagistPayload{}, nil
}

func newPackagistRequest(_ context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	meta := &PackagistMeta{}
	if err := json.Unmarshal([]byte(w.Meta), meta); err != nil {
		return nil, nil, fmt.Errorf("newpackagistRequest meta json: %w", err)
	}
	var pc payloadConvertor[PackagistPayload] = packagistConvertor{
		PackageURL: meta.PackageURL,
	}
	return newJSONRequest(pc, w, t, true)
}

func init() {
	RegisterWebhookRequester(webhook_module.PACKAGIST, newPackagistRequest)
}
