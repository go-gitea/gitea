// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package updatechecker

import (
	"io/ioutil"
	"net/http"

	"code.gitea.io/gitea/modules/appstate"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/proxy"
	"code.gitea.io/gitea/modules/setting"

	"github.com/hashicorp/go-version"
)

// CheckerState stores the remote version from the JSON endpoint
type CheckerState struct {
	LatestVersion string
}

// Name returns the name of the state item for update checker
func (r *CheckerState) Name() string {
	return "update-checker"
}

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

	type respType struct {
		Latest struct {
			Version string `json:"version"`
		} `json:"latest"`
	}
	respData := respType{}
	err = json.Unmarshal(body, &respData)
	if err != nil {
		return err
	}

	return UpdateRemoteVersion(respData.Latest.Version)

}

// UpdateRemoteVersion updates the latest available version of Gitea
func UpdateRemoteVersion(version string) (err error) {
	return appstate.AppState.Set(&CheckerState{LatestVersion: version})
}

// GetRemoteVersion returns the current remote version (or currently installed verson if fail to fetch from DB)
func GetRemoteVersion() string {
	item := new(CheckerState)
	if err := appstate.AppState.Get(item); err != nil {
		return ""
	}
	return item.LatestVersion
}

// GetNeedUpdate returns true whether a newer version of Gitea is available
func GetNeedUpdate() bool {
	curVer, err := version.NewVersion(setting.AppVer)
	if err != nil {
		// return false to fail silently
		return false
	}
	remoteVerStr := GetRemoteVersion()
	if remoteVerStr == "" {
		// no remote version is known
		return false
	}
	remoteVer, err := version.NewVersion(remoteVerStr)
	if err != nil {
		// return false to fail silently
		return false
	}
	return curVer.LessThan(remoteVer)
}
