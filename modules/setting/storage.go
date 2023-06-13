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

const (
	// LocalStorageType is the type descriptor for local storage
	LocalStorageType StorageType = "local"
	// MinioStorageType is the type descriptor for minio storage
	MinioStorageType StorageType = "minio"
)

var storageTypes = []StorageType{
	LocalStorageType,
	MinioStorageType,
}

// IsValidStorageType returns true if the given storage type is valid
func IsValidStorageType(storageType StorageType) bool {
	for _, t := range storageTypes {
		if t == storageType {
			return true
		}
	}
	return false
}

// MinioStorageConfig represents the configuration for a minio storage
type MinioStorageConfig struct {
	Endpoint           string `ini:"MINIO_ENDPOINT" json:",omitempty"`
	AccessKeyID        string `ini:"MINIO_ACCESS_KEY_ID" json:",omitempty"`
	SecretAccessKey    string `ini:"MINIO_SECRET_ACCESS_KEY" json:",omitempty"`
	Bucket             string `ini:"MINIO_BUCKET" json:",omitempty"`
	Location           string `ini:"MINIO_LOCATION" json:",omitempty"`
	BasePath           string `ini:"MINIO_BASE_PATH" json:",omitempty"`
	UseSSL             bool   `ini:"MINIO_USE_SSL"`
	InsecureSkipVerify bool   `ini:"MINIO_INSECURE_SKIP_VERIFY"`
	ChecksumAlgorithm  string `ini:"MINIO_CHECKSUM_ALGORITHM" json:",omitempty"`
	ServeDirect        bool   `ini:"SERVE_DIRECT"`
}

// Storage represents configuration of storages
type Storage struct {
	Type          StorageType        // local or minio
	Path          string             `json:",omitempty"` // for local type
	TemporaryPath string             `json:",omitempty"`
	MinioConfig   MinioStorageConfig // for minio type
}

func (storage *Storage) ToShadowCopy() Storage {
	shadowStorage := *storage
	if shadowStorage.MinioConfig.AccessKeyID != "" {
		shadowStorage.MinioConfig.AccessKeyID = "******"
	}
	if shadowStorage.MinioConfig.SecretAccessKey != "" {
		shadowStorage.MinioConfig.SecretAccessKey = "******"
	}
	return shadowStorage
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

	var targetSec ConfigSection
	if typ != "" {
		var err error
		targetSec, err = rootCfg.GetSection(storageSectionName + "." + typ)
		if err != nil {
			if !IsValidStorageType(StorageType(typ)) {
				return nil, fmt.Errorf("get section via storage type %q failed: %v", typ, err)
			}
		}
		if targetSec != nil {
			targetType := targetSec.Key("STORAGE_TYPE").String()
			if targetType == "" {
				if !IsValidStorageType(StorageType(typ)) {
					return nil, fmt.Errorf("unknow storage type %q", typ)
				}
				targetSec.Key("STORAGE_TYPE").SetValue(typ)
			} else if !IsValidStorageType(StorageType(targetType)) {
				return nil, fmt.Errorf("unknow storage type %q for section storage.%v", targetType, typ)
			}
		}
	}

	storageNameSec, _ := rootCfg.GetSection(storageSectionName + "." + name)

	if targetSec == nil {
		targetSec = sec
	}
	if targetSec == nil {
		targetSec = storageNameSec
	}
	if targetSec == nil {
		targetSec = getDefaultStorageSection(rootCfg)
	} else {
		targetType := targetSec.Key("STORAGE_TYPE").String()
		switch {
		case targetType == "":
			if targetSec.Key("PATH").String() == "" {
				targetSec = getDefaultStorageSection(rootCfg)
			} else {
				targetSec.Key("STORAGE_TYPE").SetValue("local")
			}
		default:
			newTargetSec, _ := rootCfg.GetSection(storageSectionName + "." + targetType)
			if newTargetSec == nil {
				if !IsValidStorageType(StorageType(targetType)) {
					return nil, fmt.Errorf("invalid storage section %s.%q", storageSectionName, targetType)
				}
			} else {
				targetSec = newTargetSec
				if IsValidStorageType(StorageType(targetType)) {
					tp := targetSec.Key("STORAGE_TYPE").String()
					if tp == "" {
						targetSec.Key("STORAGE_TYPE").SetValue(targetType)
					}
				}
			}
		}
	}

	targetType := targetSec.Key("STORAGE_TYPE").String()
	if !IsValidStorageType(StorageType(targetType)) {
		return nil, fmt.Errorf("invalid storage type %q", targetType)
	}

	var storage Storage
	storage.Type = StorageType(targetType)

	switch targetType {
	case string(LocalStorageType):
		storage.Path = ConfigSectionKeyString(targetSec, "PATH", filepath.Join(AppDataPath, name))
		if !filepath.IsAbs(storage.Path) {
			storage.Path = filepath.Join(AppWorkPath, storage.Path)
		}
	case string(MinioStorageType):
		storage.MinioConfig.BasePath = name + "/"

		if err := targetSec.MapTo(&storage.MinioConfig); err != nil {
			return nil, fmt.Errorf("map minio config failed: %v", err)
		}
		// extra config section will be read SERVE_DIRECT, PATH, MINIO_BASE_PATH to override the targetsec
		extraConfigSec := sec
		if extraConfigSec == nil {
			extraConfigSec = storageNameSec
		}

		if extraConfigSec != nil {
			storage.MinioConfig.ServeDirect = ConfigSectionKeyBool(extraConfigSec, "SERVE_DIRECT", storage.MinioConfig.ServeDirect)
			storage.MinioConfig.BasePath = ConfigSectionKeyString(extraConfigSec, "MINIO_BASE_PATH", storage.MinioConfig.BasePath)
			storage.MinioConfig.Bucket = ConfigSectionKeyString(extraConfigSec, "MINIO_BUCKET", storage.MinioConfig.Bucket)
		}
	}

	return &storage, nil
}
