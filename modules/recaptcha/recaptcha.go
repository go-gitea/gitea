// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package recaptcha

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// Response is the structure of JSON returned from API
type Response struct {
	Success     bool      `json:"success"`
	ChallengeTS time.Time `json:"challenge_ts"`
	Hostname    string    `json:"hostname"`
	ErrorCodes  []string  `json:"error-codes"`
}

const apiURL = "api/siteverify"

// Verify calls Google Recaptcha API to verify token
func Verify(response string) (bool, error) {
	resp, err := http.PostForm(util.URLJoin(setting.Service.RecaptchaURL, apiURL),
		url.Values{"secret": {setting.Service.RecaptchaSecret}, "response": {response}})
	if err != nil {
		return false, fmt.Errorf("Failed to send CAPTCHA response: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("Failed to read CAPTCHA response: %s", err)
	}
	var jsonResponse Response
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		return false, fmt.Errorf("Failed to parse CAPTCHA response: %s", err)
	}

	return jsonResponse.Success, nil
}
