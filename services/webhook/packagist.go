// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"errors"

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

// JSONPayload Marshals the PackagistPayload to json
func (f *PackagistPayload) JSONPayload() ([]byte, error) {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

var _ PayloadConvertor = &PackagistPayload{}

// Create implements PayloadConvertor Create method
func (f *PackagistPayload) Create(_ *api.CreatePayload) (api.Payloader, error) {
	return nil, nil
}

// Delete implements PayloadConvertor Delete method
func (f *PackagistPayload) Delete(_ *api.DeletePayload) (api.Payloader, error) {
	return nil, nil
}

// Fork implements PayloadConvertor Fork method
func (f *PackagistPayload) Fork(_ *api.ForkPayload) (api.Payloader, error) {
	return nil, nil
}

// Push implements PayloadConvertor Push method
func (f *PackagistPayload) Push(_ *api.PushPayload) (api.Payloader, error) {
	return f, nil
}

// Issue implements PayloadConvertor Issue method
func (f *PackagistPayload) Issue(_ *api.IssuePayload) (api.Payloader, error) {
	return nil, nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (f *PackagistPayload) IssueComment(_ *api.IssueCommentPayload) (api.Payloader, error) {
	return nil, nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (f *PackagistPayload) PullRequest(_ *api.PullRequestPayload) (api.Payloader, error) {
	return nil, nil
}

// Review implements PayloadConvertor Review method
func (f *PackagistPayload) Review(_ *api.PullRequestPayload, _ webhook_module.HookEventType) (api.Payloader, error) {
	return nil, nil
}

// Repository implements PayloadConvertor Repository method
func (f *PackagistPayload) Repository(_ *api.RepositoryPayload) (api.Payloader, error) {
	return nil, nil
}

// Wiki implements PayloadConvertor Wiki method
func (f *PackagistPayload) Wiki(_ *api.WikiPayload) (api.Payloader, error) {
	return nil, nil
}

// Release implements PayloadConvertor Release method
func (f *PackagistPayload) Release(_ *api.ReleasePayload) (api.Payloader, error) {
	return nil, nil
}

func (f *PackagistPayload) Package(_ *api.PackagePayload) (api.Payloader, error) {
	return nil, nil
}

// GetPackagistPayload converts a packagist webhook into a PackagistPayload
func GetPackagistPayload(p api.Payloader, event webhook_module.HookEventType, meta string) (api.Payloader, error) {
	s := new(PackagistPayload)

	packagist := &PackagistMeta{}
	if err := json.Unmarshal([]byte(meta), &packagist); err != nil {
		return s, errors.New("GetPackagistPayload meta json:" + err.Error())
	}
	s.PackagistRepository.URL = packagist.PackageURL
	return convertPayloader(s, p, event)
}
