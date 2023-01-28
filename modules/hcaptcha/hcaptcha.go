// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hcaptcha

import (
	"context"

	"code.gitea.io/gitea/modules/setting"

	"go.jolheiser.com/hcaptcha"
)

// Verify calls hCaptcha API to verify token
func Verify(ctx context.Context, response string) (bool, error) {
	client, err := hcaptcha.New(setting.Service.HcaptchaSecret, hcaptcha.WithContext(ctx))
	if err != nil {
		return false, err
	}

	resp, err := client.Verify(response, hcaptcha.PostOptions{
		Sitekey: setting.Service.HcaptchaSitekey,
	})
	if err != nil {
		return false, err
	}

	var respErr error
	if len(resp.ErrorCodes) > 0 {
		respErr = resp.ErrorCodes[0]
	}
	return resp.Success, respErr
}
