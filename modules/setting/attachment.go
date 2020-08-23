// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"path"
	"path/filepath"
	"strings"
)

var (
	// Attachment settings
	Attachment = struct {
		StoreType   string
		Path        string
		ServeDirect bool
		Minio       struct {
			Endpoint        string
			AccessKeyID     string
			SecretAccessKey string
			UseSSL          bool
			Bucket          string
			Location        string
			BasePath        string
		}
		AllowedTypes string
		MaxSize      int64
		MaxFiles     int
		Enabled      bool
	}{
		StoreType:   "local",
		ServeDirect: false,
		Minio: struct {
			Endpoint        string
			AccessKeyID     string
			SecretAccessKey string
			UseSSL          bool
			Bucket          string
			Location        string
			BasePath        string
		}{},
		AllowedTypes: "image/jpeg,image/png,application/zip,application/gzip",
		MaxSize:      4,
		MaxFiles:     5,
		Enabled:      true,
	}
)

func newAttachmentService() {
	sec := Cfg.Section("attachment")
	Attachment.StoreType = sec.Key("STORE_TYPE").MustString("local")
	Attachment.ServeDirect = sec.Key("SERVE_DIRECT").MustBool(false)
	switch Attachment.StoreType {
	case "local":
		Attachment.Path = sec.Key("PATH").MustString(path.Join(AppDataPath, "attachments"))
		if !filepath.IsAbs(Attachment.Path) {
			Attachment.Path = path.Join(AppWorkPath, Attachment.Path)
		}
	case "minio":
		Attachment.Minio.Endpoint = sec.Key("MINIO_ENDPOINT").MustString("localhost:9000")
		Attachment.Minio.AccessKeyID = sec.Key("MINIO_ACCESS_KEY_ID").MustString("")
		Attachment.Minio.SecretAccessKey = sec.Key("MINIO_SECRET_ACCESS_KEY").MustString("")
		Attachment.Minio.Bucket = sec.Key("MINIO_BUCKET").MustString("gitea")
		Attachment.Minio.Location = sec.Key("MINIO_LOCATION").MustString("us-east-1")
		Attachment.Minio.BasePath = sec.Key("MINIO_BASE_PATH").MustString("attachments/")
		Attachment.Minio.UseSSL = sec.Key("MINIO_USE_SSL").MustBool(false)
	}

	Attachment.AllowedTypes = strings.Replace(sec.Key("ALLOWED_TYPES").MustString("image/jpeg,image/png,application/zip,application/gzip"), "|", ",", -1)
	Attachment.MaxSize = sec.Key("MAX_SIZE").MustInt64(4)
	Attachment.MaxFiles = sec.Key("MAX_FILES").MustInt(5)
	Attachment.Enabled = sec.Key("ENABLED").MustBool(true)
}
