// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"sync"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/go-chi/captcha"
)

var (
	imageCaptchaOnce sync.Once
	cpt              *captcha.Captcha
)

// GetImageCaptcha returns global image captcha
func GetImageCaptcha() *captcha.Captcha {
	imageCaptchaOnce.Do(func() {
		cpt = captcha.NewCaptcha(captcha.Options{
			SubURL: setting.AppSubURL,
		})
		cpt.Store = cache.GetCache()
	})
	return cpt
}
