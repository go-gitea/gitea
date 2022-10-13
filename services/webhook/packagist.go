// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"errors"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

type (
	// PackagistPayload represents
	PackagistPayload struct {
		PackagistRepository struct {
			URL string `json:"url"`
		} `json:"repository"`
	}

	// PackagistMeta contains the meta data for the webhook
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
func (f *PackagistPayload) Create(p *api.CreatePayload) (api.Payloader, error) {
	return nil, nil
}

// Delete implements PayloadConvertor Delete method
func (f *PackagistPayload) Delete(p *api.DeletePayload) (api.Payloader, error) {
	return nil, nil
}

// Fork implements PayloadConvertor Fork method
func (f *PackagistPayload) Fork(p *api.ForkPayload) (api.Payloader, error) {
	return nil, nil
}

// Push implements PayloadConvertor Push method
func (f *PackagistPayload) Push(p *api.PushPayload) (api.Payloader, error) {
	return f, nil
}

// Issue implements PayloadConvertor Issue method
func (f *PackagistPayload) Issue(p *api.IssuePayload) (api.Payloader, error) {
	return nil, nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (f *PackagistPayload) IssueComment(p *api.IssueCommentPayload) (api.Payloader, error) {
	return nil, nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (f *PackagistPayload) PullRequest(p *api.PullRequestPayload) (api.Payloader, error) {
	return nil, nil
}

// Review implements PayloadConvertor Review method
func (f *PackagistPayload) Review(p *api.PullRequestPayload, event webhook_model.HookEventType) (api.Payloader, error) {
	return nil, nil
}

// Repository implements PayloadConvertor Repository method
func (f *PackagistPayload) Repository(p *api.RepositoryPayload) (api.Payloader, error) {
	return nil, nil
}

// Wiki implements PayloadConvertor Wiki method
func (f *PackagistPayload) Wiki(p *api.WikiPayload) (api.Payloader, error) {
	return nil, nil
}

// Release implements PayloadConvertor Release method
func (f *PackagistPayload) Release(p *api.ReleasePayload) (api.Payloader, error) {
	return nil, nil
}

// GetPackagistPayload converts a packagist webhook into a PackagistPayload
func GetPackagistPayload(p api.Payloader, event webhook_model.HookEventType, meta string) (api.Payloader, error) {
	s := new(PackagistPayload)

	packagist := &PackagistMeta{}
	if err := json.Unmarshal([]byte(meta), &packagist); err != nil {
		return s, errors.New("GetPackagistPayload meta json:" + err.Error())
	}
	s.PackagistRepository.URL = packagist.PackageURL
	return convertPayloader(s, p, event)
}
