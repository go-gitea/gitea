// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"path/filepath"
)

// StorageType is a type of Storage
type StorageType string

// LocalStorageType is the type descriptor for local storage
const LocalStorageType StorageType = "local"

// MinioStorageType is the type descriptor for minio storage
const MinioStorageType StorageType = "minio"

// MinioStorageConfig represents the configuration for a minio storage
type MinioStorageConfig struct {
	Endpoint           string `ini:"MINIO_ENDPOINT"`
	AccessKeyID        string `ini:"MINIO_ACCESS_KEY_ID"`
	SecretAccessKey    string `ini:"MINIO_SECRET_ACCESS_KEY"`
	Bucket             string `ini:"MINIO_BUCKET"`
	Location           string `ini:"MINIO_LOCATION"`
	BasePath           string `ini:"MINIO_BASE_PATH"`
	UseSSL             bool   `ini:"MINIO_USE_SSL"`
	InsecureSkipVerify bool   `ini:"MINIO_INSECURE_SKIP_VERIFY"`
	ChecksumAlgorithm  string `ini:"MINIO_CHECKSUM_ALGORITHM"`
	ServeDirect        bool   `ini:"SERVE_DIRECT"`
}

// Storage represents configuration of storages
type Storage struct {
	Type        StorageType        // local or minio
	Path        string             // for local type
	MinioConfig MinioStorageConfig // for minio type
}

const storageSectionName = "storage"

func getDefaultStorageSection(rootCfg ConfigProvider) ConfigSection {
	storageSec := rootCfg.Section(storageSectionName)
	// Global Defaults
	storageSec.Key("STORAGE_TYPE").MustString("local")
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

func getStorage(rootCfg ConfigProvider, name, typ string, sec ConfigSection) (*Storage, error) {
	if name == "" {
		return nil, errors.New("no name for storage")
	}

	var targetSec ConfigSection = nil
	if typ != "" {
		var err error
		targetSec, err = rootCfg.GetSection(storageSectionName + "." + typ)
		if err != nil {
			if typ != "local" && typ != "minio" {
				return nil, fmt.Errorf("get section via storage type %q failed: %v", typ, err)
			}
		}
		if targetSec != nil {
			targetType := targetSec.Key("STORAGE_TYPE").String()
			if targetType == "" {
				if typ != "local" && typ != "minio" {
					return nil, fmt.Errorf("unknow storage type %q", typ)
				}
				targetSec.Key("STORAGE_TYPE").SetValue(typ)
			} else if targetType != "local" && targetType != "minio" {
				return nil, fmt.Errorf("unknow storage type %q for section storage.%v", targetType, typ)
			}
		}
	}

	packageNameSec, _ := rootCfg.GetSection(storageSectionName + "." + name)

	if targetSec == nil {
		targetSec = sec
	}
	if targetSec == nil {
		targetSec = packageNameSec
	}
	if targetSec == nil {
		targetSec = getDefaultStorageSection(rootCfg)
	} else {
		targetType := targetSec.Key("STORAGE_TYPE").String()
		switch {
		case targetType == "":
			targetSec = getDefaultStorageSection(rootCfg)
		default:
			newTargetSec, _ := rootCfg.GetSection(storageSectionName + "." + targetType)
			if newTargetSec == nil {
				if targetType != "local" && targetType != "minio" {
					return nil, fmt.Errorf("invalid storage section %s.%q", storageSectionName, targetType)
				}
			} else {
				targetSec = newTargetSec
				if targetType == "local" || targetType == "minio" {
					tp := targetSec.Key("STORAGE_TYPE").String()
					if tp == "" {
						targetSec.Key("STORAGE_TYPE").SetValue(targetType)
					}
				}
			}
		}
	}

	targetType := targetSec.Key("STORAGE_TYPE").String()
	if targetType != "local" && targetType != "minio" {
		return nil, fmt.Errorf("invalid storage type %q", targetType)
	}

	var storage Storage
	storage.Type = StorageType(targetType)

	switch targetType {
	case string(LocalStorageType):
		storage.Path = targetSec.Key("PATH").MustString(filepath.Join(AppDataPath, name))
		if !filepath.IsAbs(storage.Path) {
			storage.Path = filepath.Join(AppWorkPath, storage.Path)
			targetSec.Key("PATH").SetValue(storage.Path)
		}
	case string(MinioStorageType):
		storage.MinioConfig.BasePath = name + "/"

		if err := targetSec.MapTo(&storage.MinioConfig); err != nil {
			return nil, fmt.Errorf("map minio config failed: %v", err)
		}
		// extra config section will be read SERVE_DIRECT, PATH, MINIO_BASE_PATH to override the targetsec
		extraConfigSec := sec
		if extraConfigSec == nil {
			extraConfigSec = packageNameSec
		}

		if extraConfigSec != nil {
			storage.MinioConfig.ServeDirect = MustSectionKeyBool(extraConfigSec, "SERVE_DIRECT", storage.MinioConfig.ServeDirect)
			storage.MinioConfig.BasePath = MustSectionKeyString(extraConfigSec, "MINIO_BASE_PATH", storage.MinioConfig.BasePath)
			storage.MinioConfig.Bucket = MustSectionKeyString(extraConfigSec, "MINIO_BUCKET", storage.MinioConfig.Bucket)
		}
	}

	return &storage, nil
}
