// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

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
	API.SwaggerURL = AppURL + "api/swagger"
}
