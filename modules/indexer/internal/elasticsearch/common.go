// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package elasticsearch

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/json"
)

// elasticRootResponse contains a subset of the response usable for version detection.
type elasticRootResponse struct {
	Version struct {
		Number string `json:"number"`
	} `json:"version"`
}

// DetectVersion detects the major version of the elasticsearch server.
// Currently only supports version 7 and 8.
func DetectVersion(url string) (int, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return parseElasticVersion(resp.Body)
}

func parseElasticVersion(body io.Reader) (int, error) {
	var root elasticRootResponse
	if err := json.NewDecoder(body).Decode(&root); err != nil {
		return 0, err
	}

	majorStr, _, ok := strings.Cut(root.Version.Number, ".")
	if !ok {
		return 0, errors.New("invalid version number")
	}

	if majorStr == "8" {
		return 8, nil
	} else if majorStr == "7" {
		return 7, nil
	}
	return 0, errors.New("unsupported ElasticSearch version")
}
