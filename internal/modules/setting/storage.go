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

// getStorage will find target section and extra special section first and then read override
// items from extra section
func getStorage(rootCfg ConfigProvider, name, typ string, sec ConfigSection) (*Storage, error) {
	if name == "" {
		return nil, errors.New("no name for storage")
	}

	targetSec, tp, err := getStorageTargetSection(rootCfg, name, typ, sec)
	if err != nil {
		return nil, err
	}

	overrideSec := getStorageOverrideSection(rootCfg, targetSec, sec, tp, name)

	targetType := targetSec.Key("STORAGE_TYPE").String()
	switch targetType {
	case string(LocalStorageType):
		return getStorageForLocal(targetSec, overrideSec, tp, name)
	case string(MinioStorageType):
		return getStorageForMinio(targetSec, overrideSec, tp, name)
	default:
		return nil, fmt.Errorf("unsupported storage type %q", targetType)
	}
}

type targetSecType int

const (
	targetSecIsTyp             targetSecType = iota // target section is [storage.type] which the type from parameter
	targetSecIsStorage                              // target section is [storage]
	targetSecIsDefault                              // target section is the default value
	targetSecIsStorageWithName                      // target section is [storage.name]
	targetSecIsSec                                  // target section is from the name seciont [name]
)

func getStorageSectionByType(rootCfg ConfigProvider, typ string) (ConfigSection, targetSecType, error) {
	targetSec, err := rootCfg.GetSection(storageSectionName + "." + typ)
	if err != nil {
		if !IsValidStorageType(StorageType(typ)) {
			return nil, 0, fmt.Errorf("get section via storage type %q failed: %v", typ, err)
		}
		// if typ is a valid storage type, but there is no [storage.local] or [storage.minio] section
		// it's not an error
		return nil, 0, nil
	}

	targetType := targetSec.Key("STORAGE_TYPE").String()
	if targetType == "" {
		if !IsValidStorageType(StorageType(typ)) {
			return nil, 0, fmt.Errorf("unknow storage type %q", typ)
		}
		targetSec.Key("STORAGE_TYPE").SetValue(typ)
	} else if !IsValidStorageType(StorageType(targetType)) {
		return nil, 0, fmt.Errorf("unknow storage type %q for section storage.%v", targetType, typ)
	}

	return targetSec, targetSecIsTyp, nil
}

func getStorageTargetSection(rootCfg ConfigProvider, name, typ string, sec ConfigSection) (ConfigSection, targetSecType, error) {
	// check typ first
	if typ == "" {
		if sec != nil { // check sec's type secondly
			typ = sec.Key("STORAGE_TYPE").String()
			if IsValidStorageType(StorageType(typ)) {
				if targetSec, _ := rootCfg.GetSection(storageSectionName + "." + typ); targetSec == nil {
					return sec, targetSecIsSec, nil
				}
			}
		}
	}

	if typ != "" {
		targetSec, tp, err := getStorageSectionByType(rootCfg, typ)
		if targetSec != nil || err != nil {
			return targetSec, tp, err
		}
	}

	// check stoarge name thirdly
	targetSec, _ := rootCfg.GetSection(storageSectionName + "." + name)
	if targetSec != nil {
		targetType := targetSec.Key("STORAGE_TYPE").String()
		switch {
		case targetType == "":
			if targetSec.Key("PATH").String() == "" { // both storage type and path are empty, use default
				return getDefaultStorageSection(rootCfg), targetSecIsDefault, nil
			}

			targetSec.Key("STORAGE_TYPE").SetValue("local")
		default:
			targetSec, tp, err := getStorageSectionByType(rootCfg, targetType)
			if targetSec != nil || err != nil {
				return targetSec, tp, err
			}
		}

		return targetSec, targetSecIsStorageWithName, nil
	}

	return getDefaultStorageSection(rootCfg), targetSecIsDefault, nil
}

// getStorageOverrideSection override section will be read SERVE_DIRECT, PATH, MINIO_BASE_PATH, MINIO_BUCKET to override the targetsec when possible
func getStorageOverrideSection(rootConfig ConfigProvider, targetSec, sec ConfigSection, targetSecType targetSecType, name string) ConfigSection {
	if targetSecType == targetSecIsSec {
		return nil
	}

	if sec != nil {
		return sec
	}

	if targetSecType != targetSecIsStorageWithName {
		nameSec, _ := rootConfig.GetSection(storageSectionName + "." + name)
		return nameSec
	}
	return nil
}

func getStorageForLocal(targetSec, overrideSec ConfigSection, tp targetSecType, name string) (*Storage, error) {
	storage := Storage{
		Type: StorageType(targetSec.Key("STORAGE_TYPE").String()),
	}

	targetPath := ConfigSectionKeyString(targetSec, "PATH", "")
	var fallbackPath string
	if targetPath == "" { // no path
		fallbackPath = filepath.Join(AppDataPath, name)
	} else {
		if tp == targetSecIsStorage || tp == targetSecIsDefault {
			fallbackPath = filepath.Join(targetPath, name)
		} else {
			fallbackPath = targetPath
		}
		if !filepath.IsAbs(fallbackPath) {
			fallbackPath = filepath.Join(AppDataPath, fallbackPath)
		}
	}

	if overrideSec == nil { // no override section
		storage.Path = fallbackPath
	} else {
		storage.Path = ConfigSectionKeyString(overrideSec, "PATH", "")
		if storage.Path == "" { // overrideSec has no path
			storage.Path = fallbackPath
		} else if !filepath.IsAbs(storage.Path) {
			if targetPath == "" {
				storage.Path = filepath.Join(AppDataPath, storage.Path)
			} else {
				storage.Path = filepath.Join(targetPath, storage.Path)
			}
		}
	}

	return &storage, nil
}

func getStorageForMinio(targetSec, overrideSec ConfigSection, tp targetSecType, name string) (*Storage, error) {
	var storage Storage
	storage.Type = StorageType(targetSec.Key("STORAGE_TYPE").String())
	if err := targetSec.MapTo(&storage.MinioConfig); err != nil {
		return nil, fmt.Errorf("map minio config failed: %v", err)
	}

	if storage.MinioConfig.BasePath == "" {
		storage.MinioConfig.BasePath = name + "/"
	}

	if overrideSec != nil {
		storage.MinioConfig.ServeDirect = ConfigSectionKeyBool(overrideSec, "SERVE_DIRECT", storage.MinioConfig.ServeDirect)
		storage.MinioConfig.BasePath = ConfigSectionKeyString(overrideSec, "MINIO_BASE_PATH", storage.MinioConfig.BasePath)
		storage.MinioConfig.Bucket = ConfigSectionKeyString(overrideSec, "MINIO_BUCKET", storage.MinioConfig.Bucket)
	}
	return &storage, nil
}
