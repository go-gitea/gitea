// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// CamoEncode encodes a lnk to fit with the go-camo and camo proxy links. The purposes of camo-proxy are:
// 1. Allow accessing "http://" images on a HTTPS site by using the "https://" URLs provided by camo-proxy.
// 2. Hide the visitor's real IP (protect privacy) when accessing external images.
func CamoEncode(link string) string {
	if strings.HasPrefix(link, setting.Camo.ServerURL) {
		return link
	}

	mac := hmac.New(sha1.New, []byte(setting.Camo.HMACKey))
	_, _ = mac.Write([]byte(link)) // hmac does not return errors
	macSum := b64encode(mac.Sum(nil))
	encodedURL := b64encode([]byte(link))

	return util.URLJoin(setting.Camo.ServerURL, macSum, encodedURL)
}

func b64encode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func camoHandleLink(link string) string {
	if setting.Camo.Enabled {
		lnkURL, err := url.Parse(link)
		if err == nil && lnkURL.IsAbs() && !strings.HasPrefix(link, setting.AppURL) &&
			(setting.Camo.Always || lnkURL.Scheme != "https") {
			return CamoEncode(link)
		}
	}
	return link
}
