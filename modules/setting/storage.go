// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"strings"

	"code.gitea.io/gitea/modules/log"
	ini "gopkg.in/ini.v1"
)

// enumerate all storage types
const (
	LocalStorageType = "local"
	MinioStorageType = "minio"
)

// Storage represents configuration of storages
type Storage struct {
	Type        string
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
}

var (
	storages = make(map[string]Storage)
)

func getStorage(sec *ini.Section) Storage {
	var storage Storage
	storage.Type = sec.Key("STORAGE_TYPE").MustString(LocalStorageType)
	storage.ServeDirect = sec.Key("SERVE_DIRECT").MustBool(false)
	switch storage.Type {
	case LocalStorageType:
	case MinioStorageType:
		storage.Minio.Endpoint = sec.Key("MINIO_ENDPOINT").MustString("localhost:9000")
		storage.Minio.AccessKeyID = sec.Key("MINIO_ACCESS_KEY_ID").MustString("")
		storage.Minio.SecretAccessKey = sec.Key("MINIO_SECRET_ACCESS_KEY").MustString("")
		storage.Minio.Bucket = sec.Key("MINIO_BUCKET").MustString("gitea")
		storage.Minio.Location = sec.Key("MINIO_LOCATION").MustString("us-east-1")
		storage.Minio.UseSSL = sec.Key("MINIO_USE_SSL").MustBool(false)
	}
	return storage
}

func newStorageService() {
	sec := Cfg.Section("storage")
	storages["default"] = getStorage(sec)

	for _, sec := range Cfg.Section("storage").ChildSections() {
		name := strings.TrimPrefix(sec.Name(), "storage.")
		if name == "default" || name == LocalStorageType || name == MinioStorageType {
			log.Error("storage name %s is system reserved!", name)
			continue
		}
		storages[name] = getStorage(sec)
	}
}
