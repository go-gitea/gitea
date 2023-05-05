// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
)

// Storage represents configuration of storages
type Storage struct {
	Type        string
	Path        string
	Section     ConfigSection
	ServeDirect bool
}

// MapTo implements the Mappable interface
func (s *Storage) MapTo(v interface{}) error {
	pathValue := reflect.ValueOf(v).Elem().FieldByName("Path")
	if pathValue.IsValid() && pathValue.Kind() == reflect.String {
		pathValue.SetString(s.Path)
	}
	if s.Section != nil {
		return s.Section.MapTo(v)
	}
	return nil
}

const storageSectionName = "storage"

func getStorageSection(rootCfg ConfigProvider) ConfigSection {
	storageSec := rootCfg.Section(storageSectionName)
	// Global Defaults
	storageSec.Key("MINIO_ENDPOINT").MustString("localhost:9000")
	storageSec.Key("MINIO_ACCESS_KEY_ID").MustString("")
	storageSec.Key("MINIO_SECRET_ACCESS_KEY").MustString("")
	storageSec.Key("MINIO_BUCKET").MustString("gitea")
	storageSec.Key("MINIO_LOCATION").MustString("us-east-1")
	storageSec.Key("MINIO_USE_SSL").MustBool(false)
	storageSec.Key("MINIO_INSECURE_SKIP_VERIFY").MustBool(false)
	storageSec.Key("MINIO_CHECKSUM_ALGORITHM").MustString("default")
	return storageSec
}

// getStorage will read storage configurations from 4 possible ways
// 1 read configurations from [$name] if the setting keys exist (eg: name="attachments")
// 2 read configurations from [storage.$name] if the keys exist
// 3 read configurations from [storage.$type] if the keys exist (eg: type="local" or "minio")
// 4 read configurations from [storage] if the keys exist
// The keys in earlier section have higher priority.
func getStorage(rootCfg ConfigProvider, startSec ConfigSection, name, typ string) (*Storage, error) {
	if name == "" {
		return nil, errors.New("getStorage: name cannot be empty")
	}

	targetSec := startSec
	if targetSec == nil {
		targetSec, _ = rootCfg.GetSection(storageSectionName + "." + name)
	} else {
		if targetSec.Key("STORAGE_TYPE").String() == "" {
			targetSec = nil
		}
	}

	if targetSec == nil && typ != "" {
		targetSec, _ = rootCfg.GetSection(storageSectionName + "." + typ)
	}
	if targetSec == nil {
		targetSec, _ = rootCfg.GetSection(storageSectionName)
	}
	if targetSec == nil { // finally fallback
		targetSec = startSec
		if targetSec == nil {
			var err error
			targetSec, err = rootCfg.NewSection(storageSectionName + "." + name)
			if err != nil {
				return nil, err
			}
		}
	}

	storageType := targetSec.Key("STORAGE_TYPE").MustString("local")
	if storageType != "local" && storageType != "minio" {
		var err error
		targetSec, err = rootCfg.GetSection(storageSectionName + "." + storageType)
		if err != nil {
			return nil, fmt.Errorf("unknown storage type: %s", storageType)
		}
		storageType = targetSec.Key("STORAGE_TYPE").String()
	}

	storage := Storage{
		Section:     targetSec,
		Type:        storageType,
		ServeDirect: targetSec.Key("SERVE_DIRECT").MustBool(false),
		Path:        targetSec.Key("PATH").MustString(filepath.Join(AppDataPath, name)),
	}

	// Specific defaults
	if !filepath.IsAbs(storage.Path) {
		storage.Path = filepath.Join(AppWorkPath, storage.Path)
		storage.Section.Key("PATH").SetValue(storage.Path)
	}
	storage.Section.Key("MINIO_BASE_PATH").MustString(name + "/")

	return &storage, nil
}
