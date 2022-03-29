// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// CamoEncode encodes a lnk to fit with the go-camo and camo proxy links
func CamoEncode(link string) string {
	if strings.HasPrefix(link, setting.CamoServerURL) || len(setting.CamoHMACKey) == 0 {
		return link
	}

	mac := hmac.New(sha1.New, []byte(setting.CamoHMACKey))
	_, _ = mac.Write([]byte(link)) // hmac does not return errors
	macSum := b64encode(mac.Sum(nil))
	encodedURL := b64encode([]byte(link))

	return util.URLJoin(setting.CamoServerURL, macSum, encodedURL)
}

func b64encode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}
