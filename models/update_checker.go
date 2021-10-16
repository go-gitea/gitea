// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/proxy"
	"code.gitea.io/gitea/modules/setting"

	"github.com/hashicorp/go-version"
)

// RemoteVersion stores the remote version from the JSON endpoint
type RemoteVersion struct {
	ID      int64  `xorm:"pk autoincr"`
	Version string `xorm:"VARCHAR(50)"`
}

func init() {
	db.RegisterModel(new(RemoteVersion))
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

	return UpdateRemoteVersion(ver.Latest.Version)

}

// UpdateRemoteVersion updates the latest available version of Gitea
func UpdateRemoteVersion(version string) (err error) {
	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	currentVersion := &RemoteVersion{ID: 1}
	has, err := sess.Get(currentVersion)
	if err != nil {
		return fmt.Errorf("get: %v", err)
	} else if !has {
		currentVersion.ID = 1
		currentVersion.Version = version

		if _, err = sess.InsertOne(currentVersion); err != nil {
			return fmt.Errorf("insert: %v", err)
		}
		return nil
	}

	if _, err = sess.Update(&RemoteVersion{ID: 1, Version: version}); err != nil {
		return err
	}

	return sess.Commit()
}

// GetRemoteVersion returns the current remote version (or currently installed verson if fail to fetch from DB)
func GetRemoteVersion() string {
	e := db.GetEngine(db.DefaultContext)
	v := &RemoteVersion{ID: 1}
	_, err := e.Get(&v)
	if err != nil {
		// return current version if fail to fetch from DB
		return setting.AppVer
	}
	return v.Version
}

// GetNeedUpdate returns true whether a newer version of Gitea is available
func GetNeedUpdate() bool {
	curVer, err := version.NewVersion(setting.AppVer)
	if err != nil {
		// return false to fail silently
		return false
	}
	remoteVer, err := version.NewVersion(GetRemoteVersion())
	if err != nil {
		// return false to fail silently
		return false
	}
	return curVer.LessThan(remoteVer)
}
