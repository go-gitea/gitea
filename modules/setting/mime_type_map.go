// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"mime"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// MimeTypeMap defines custom mime type mapping settings
var MimeTypeMap = struct {
	Enabled bool
	Map     map[string]string
}{
	Enabled: false,
	Map:     map[string]string{},
}

func newMimeTypeMap() {
	sec := Cfg.Section("repository.mimetype_mapping")
	keys := sec.Keys()
	m := make(map[string]string, len(keys))
	for _, key := range keys {
		m[strings.ToLower(key.Name())] = key.Value()
		err := mime.AddExtensionType(key.Name(), key.Value())
		if err != nil {
			log.Warn("mime.AddExtensionType(%s,%s): %v", key.Name(), key.Value(), err)
		}
	}
	MimeTypeMap.Map = m
	if len(keys) > 0 {
		MimeTypeMap.Enabled = true
	}
}
