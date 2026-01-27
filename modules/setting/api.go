// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/url"
	"path"

	"code.gitea.io/gitea/modules/log"
)

// API settings
var API = struct {
	EnableSwagger          bool
	SwaggerURL             string
	MaxResponseItems       int
	DefaultPagingNum       int
	DefaultGitTreesPerPage int
	DefaultMaxBlobSize     int64
	DefaultMaxResponseSize int64
}{
	EnableSwagger:          true,
	SwaggerURL:             "",
	MaxResponseItems:       50,
	DefaultPagingNum:       30,
	DefaultGitTreesPerPage: 1000,
	DefaultMaxBlobSize:     10485760,
	DefaultMaxResponseSize: 104857600,
}

func loadAPIFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "api", &API)

	defaultAppURL := string(Protocol) + "://" + Domain + ":" + HTTPPort
	u, err := url.Parse(rootCfg.Section("server").Key("ROOT_URL").MustString(defaultAppURL))
	if err != nil {
		log.Fatal("Invalid ROOT_URL '%s': %s", AppURL, err)
	}
	u.Path = path.Join(u.Path, "api", "swagger")
	API.SwaggerURL = u.String()
}
