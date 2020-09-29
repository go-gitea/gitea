// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
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
	Attachment.Storage.Type = sec.Key("STORAGE_TYPE").MustString("")
	if Attachment.Storage.Type == "" {
		Attachment.Storage.Type = "default"
	}

	if Attachment.Storage.Type != LocalStorageType && Attachment.Storage.Type != MinioStorageType {
		storage, ok := storages[Attachment.Storage.Type]
		if !ok {
			log.Fatal("Failed to get attachment storage type: %s", Attachment.Storage.Type)
		}
		Attachment.Storage = storage

		for _, key := range storage.Section.Keys() {
			if !sec.HasKey(key.Name()) {
				_, _ = sec.NewKey(key.Name(), key.Value())
			}
		}
		Attachment.Storage.Section = sec
	}

	// Override
	Attachment.ServeDirect = sec.Key("SERVE_DIRECT").MustBool(Attachment.ServeDirect)

	Attachment.Storage.Path = sec.Key("PATH").MustString(filepath.Join(AppDataPath, "attachments"))
	if !filepath.IsAbs(Attachment.Storage.Path) {
		Attachment.Storage.Path = filepath.Join(AppWorkPath, Attachment.Storage.Path)
		sec.Key("PATH").SetValue(Attachment.Storage.Path)
	}

	sec.Key("MINIO_BASE_PATH").MustString("attachments/")

	Attachment.AllowedTypes = strings.Replace(sec.Key("ALLOWED_TYPES").MustString("image/jpeg,image/png,application/zip,application/gzip"), "|", ",", -1)
	Attachment.MaxSize = sec.Key("MAX_SIZE").MustInt64(4)
	Attachment.MaxFiles = sec.Key("MAX_FILES").MustInt(5)
	Attachment.Enabled = sec.Key("ENABLED").MustBool(true)
}
