// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"strings"
)

var (
	// Attachment settings
	Attachment = struct {
		Storage
		AllowedTypes string
		MaxSize      int64
		MaxFiles     int
		Enabled      bool
	}{
		Storage: Storage{
			Type:        LocalStorageType,
			ServeDirect: false,
		},
		AllowedTypes: "image/jpeg,image/png,application/zip,application/gzip",
		MaxSize:      4,
		MaxFiles:     5,
		Enabled:      true,
	}
)

func newAttachmentService() {
	sec := Cfg.Section("attachment")
	storageType := sec.Key("STORAGE_TYPE").MustString("")

	Attachment.Storage = getStorage("attachment", storageType, sec)

	// Other settings
	Attachment.AllowedTypes = strings.Replace(sec.Key("ALLOWED_TYPES").MustString("image/jpeg,image/png,application/zip,application/gzip"), "|", ",", -1)
	Attachment.MaxSize = sec.Key("MAX_SIZE").MustInt64(4)
	Attachment.MaxFiles = sec.Key("MAX_FILES").MustInt(5)
	Attachment.Enabled = sec.Key("ENABLED").MustBool(true)
}
