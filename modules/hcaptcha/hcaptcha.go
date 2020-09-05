// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package hcaptcha

import (
	"context"

	"code.gitea.io/gitea/modules/setting"

	"go.jolheiser.com/hcaptcha"
)

// Verify calls hCaptcha API to verify token
func Verify(ctx context.Context, response string) (bool, error) {
	client, err := hcaptcha.New(setting.Service.RecaptchaSecret, hcaptcha.WithContext(ctx))
	if err != nil {
		return false, err
	}

	resp, err := client.Verify(response, hcaptcha.PostOptions{
		Sitekey: setting.Service.RecaptchaSitekey,
	})
	if err != nil {
		return false, err
	}

	return resp.Success, resp.ErrorCodes[0]
}
