// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package updatechecker

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"gitea.dev/modules/json"
	"gitea.dev/modules/proxy"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/system"

	"github.com/hashicorp/go-version"
)

// CheckerState stores the remote version from the JSON endpoint
type CheckerState struct {
	LatestVersion string
}

func (CheckerState) StateName() string {
	return "update-checker"
}

// maxResponseSize caps the version check response. It is a small JSON document,
// so this is far above any legitimate payload while still bounding memory use.
const maxResponseSize = 1024 * 1024 // 1 MiB

// GiteaUpdateChecker returns error when new version of Gitea is available
func GiteaUpdateChecker(httpEndpoint string) error {
	latestVersion, err := fetchLatestVersion(httpEndpoint)
	if err != nil {
		return err
	}
	return UpdateRemoteVersion(context.Background(), latestVersion)
}

// fetchLatestVersion reads the latest released version from the JSON endpoint
func fetchLatestVersion(httpEndpoint string) (string, error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: proxy.Proxy(),
		},
	}

	req, err := http.NewRequest(http.MethodGet, httpEndpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return "", err
	}
	if len(body) > maxResponseSize {
		return "", fmt.Errorf("update check response exceeds %d bytes", maxResponseSize)
	}

	type respType struct {
		Latest struct {
			Version string `json:"version"`
		} `json:"latest"`
	}
	respData := respType{}
	err = json.Unmarshal(body, &respData)
	if err != nil {
		return "", err
	}

	return respData.Latest.Version, nil
}

// UpdateRemoteVersion updates the latest available version of Gitea
func UpdateRemoteVersion(ctx context.Context, version string) (err error) {
	return system.AppState.Set(ctx, &CheckerState{LatestVersion: version})
}

// GetRemoteVersion returns the current remote version (or currently installed version if fail to fetch from DB)
func GetRemoteVersion(ctx context.Context) string {
	item := new(CheckerState)
	if err := system.AppState.Get(ctx, item); err != nil {
		return ""
	}
	return item.LatestVersion
}

// GetNeedUpdate returns true whether a newer version of Gitea is available
func GetNeedUpdate(ctx context.Context) bool {
	curVer, err := version.NewVersion(setting.AppVer)
	if err != nil {
		// return false to fail silently
		return false
	}
	remoteVerStr := GetRemoteVersion(ctx)
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
