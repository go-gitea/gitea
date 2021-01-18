// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"sync"

	"code.gitea.io/gitea/modules/setting"
	"gitea.com/go-chi/captcha"
)

var imageCaptchaOnce sync.Once

// GetImageCaptcha returns global image captcha
func GetImageCaptcha() *captcha.Captcha {
	var cpt *captcha.Captcha
	imageCaptchaOnce.Do(func() {
		cpt = captcha.NewCaptcha(captcha.Options{
			SubURL: setting.AppSubURL,
		})
	})
	return cpt
}
