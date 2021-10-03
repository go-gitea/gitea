// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"code.gitea.io/gitea/modules/proxy"
	"code.gitea.io/gitea/modules/setting"

	"github.com/hashicorp/go-version"
)

// GiteaUpdateChecker returns error when new version of Gitea is available
func GiteaUpdateChecker(httpEndpoint string) error {
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: proxy.Proxy(),
		},
	}

	req, err := http.NewRequest("GET", httpEndpoint, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	type v struct {
		Latest struct {
			Version string `json:"version"`
		} `json:"latest"`
	}
	ver := v{}
	err = json.Unmarshal(body, &ver)
	if err != nil {
		return err
	}

	giteaVersion, _ := version.NewVersion(setting.AppVer)
	updateVersion, _ := version.NewVersion(ver.Latest.Version)
	if giteaVersion.LessThan(updateVersion) {
		return fmt.Errorf("Newer version of Gitea available: %s Check the blog for more details https://blog.gitea.io", ver.Latest.Version)
	}

	return nil
}
