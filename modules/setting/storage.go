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
// 1 read configurations from [storage.$name] if the keys exist
// 2 read configurations from given section if it's not nil
// 3 read configurations from [storage.$type] if the keys exist (eg: type="local" or "minio")
// 4 read configurations from [storage] if the keys exist
// The keys in earlier section have higher priority.
func getStorage(rootCfg ConfigProvider, name string, startSec ConfigSection, typ string) (*Storage, error) {
	if name == "" {
		return nil, errors.New("getStorage: name cannot be empty")
	}

	nameSec, _ := rootCfg.GetSection(storageSectionName + "." + name)

	targetSec := nameSec
	storageType := ""
	if targetSec == nil {
		targetSec = startSec
		if targetSec != nil {
			storageType = targetSec.Key("STORAGE_TYPE").String()
			if storageType == "" {
				targetSec = nil // startSec's STORAGE_TYPE could be ignored
			} else {
				storageSec, _ := rootCfg.GetSection(storageSectionName + "." + storageType)
				if storageSec != nil {
					targetSec = storageSec
					if storageType != "local" && storageType != "minio" {
						storageType = targetSec.Key("STORAGE_TYPE").MustString("local")
					}
				} else if storageType != "local" && storageType != "minio" {
					return nil, fmt.Errorf("unknown storage type: %s", storageType)
				}
			}
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
			targetSec = getStorageSection(rootCfg)
		}
	}

	storageType = targetSec.Key("STORAGE_TYPE").MustString("local")
	if storageType != "local" && storageType != "minio" {
		var err error
		targetSec, err = rootCfg.GetSection(storageSectionName + "." + storageType)
		if err != nil {
			return nil, fmt.Errorf("unknown storage type: %s", storageType)
		}
		storageType = targetSec.Key("STORAGE_TYPE").String()
		if storageType != "local" && storageType != "minio" {
			return nil, fmt.Errorf("%s should have STORAGE_TYPE as local or minio", storageSectionName+"."+storageType)
		}
	}

	// just nameSec and startSec could contain override configurations
	overrideSec := nameSec
	if overrideSec == nil {
		overrideSec = startSec
	}

	serveDirect := false
	path := filepath.Join(AppDataPath, name)
	minioBucket := targetSec.Key("MINIO_BUCKET").String()
	minioBasePath := name + "/"
	if overrideSec != nil {
		serveDirect = overrideSec.Key("SERVE_DIRECT").MustBool(false)
		path = overrideSec.Key("PATH").MustString(path)
		minioBucket = overrideSec.Key("MINIO_BUCKET").MustString(minioBucket)
		minioBasePath = overrideSec.Key("MINIO_BASE_PATH").MustString(minioBasePath)
	}

	storage := Storage{
		Section:     targetSec,
		Type:        storageType,
		ServeDirect: serveDirect,
		Path:        path,
	}

	// Specific defaults
	if !filepath.IsAbs(storage.Path) {
		storage.Path = filepath.Join(AppWorkPath, storage.Path)
		storage.Section.Key("PATH").SetValue(storage.Path)
	}
	storage.Section.Key("MINIO_BUCKET").SetValue(minioBucket)
	storage.Section.Key("MINIO_BASE_PATH").SetValue(minioBasePath)

	return &storage, nil
}
