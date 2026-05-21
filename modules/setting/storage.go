// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// StorageType is a type of Storage
type StorageType string

const (
	// LocalStorageType is the type descriptor for local storage
	LocalStorageType StorageType = "local"
	// S3StorageType is the type descriptor for S3-compatible storage
	S3StorageType StorageType = "s3"
	// AzureBlobStorageType is the type descriptor for azure blob storage
	AzureBlobStorageType StorageType = "azureblob"
	// legacyMinioStorageType is the deprecated value of S3StorageType,
	// accepted on read and rewritten to S3StorageType in
	// migrateDeprecatedStorageConfig.
	legacyMinioStorageType StorageType = "minio"
)

var storageTypes = []StorageType{
	LocalStorageType,
	S3StorageType,
	AzureBlobStorageType,
	legacyMinioStorageType, // remove together with the rest of the minio fallback in MinioToS3RemovalVersion
}

// IsValidStorageType returns true if the given storage type is valid
func IsValidStorageType(storageType StorageType) bool {
	return slices.Contains(storageTypes, storageType)
}

// S3StorageConfig represents the configuration for an S3-compatible storage
type S3StorageConfig struct {
	Endpoint           string `ini:"S3_ENDPOINT" json:",omitempty"`
	AccessKeyID        string `ini:"S3_ACCESS_KEY_ID" json:",omitempty"`
	SecretAccessKey    string `ini:"S3_SECRET_ACCESS_KEY" json:",omitempty"`
	IamEndpoint        string `ini:"S3_IAM_ENDPOINT" json:",omitempty"`
	Bucket             string `ini:"S3_BUCKET" json:",omitempty"`
	Location           string `ini:"S3_LOCATION" json:",omitempty"`
	BasePath           string `ini:"S3_BASE_PATH" json:",omitempty"`
	UseSSL             bool   `ini:"S3_USE_SSL"`
	InsecureSkipVerify bool   `ini:"S3_INSECURE_SKIP_VERIFY"`
	ChecksumAlgorithm  string `ini:"S3_CHECKSUM_ALGORITHM" json:",omitempty"`
	ServeDirect        bool   `ini:"SERVE_DIRECT"`
	BucketLookUpType   string `ini:"S3_BUCKET_LOOKUP_TYPE" json:",omitempty"`
}

// minioToS3KeyRenames lists deprecated MINIO_* ini keys and their S3_*
// replacements, ordered for stable startup-log output.
var minioToS3KeyRenames = []struct{ oldKey, newKey string }{
	{"MINIO_ACCESS_KEY_ID", "S3_ACCESS_KEY_ID"},
	{"MINIO_BASE_PATH", "S3_BASE_PATH"},
	{"MINIO_BUCKET", "S3_BUCKET"},
	{"MINIO_BUCKET_LOOKUP_TYPE", "S3_BUCKET_LOOKUP_TYPE"},
	{"MINIO_CHECKSUM_ALGORITHM", "S3_CHECKSUM_ALGORITHM"},
	{"MINIO_ENDPOINT", "S3_ENDPOINT"},
	{"MINIO_IAM_ENDPOINT", "S3_IAM_ENDPOINT"},
	{"MINIO_INSECURE_SKIP_VERIFY", "S3_INSECURE_SKIP_VERIFY"},
	{"MINIO_LOCATION", "S3_LOCATION"},
	{"MINIO_SECRET_ACCESS_KEY", "S3_SECRET_ACCESS_KEY"},
	{"MINIO_USE_SSL", "S3_USE_SSL"},
}

const MinioToS3RemovalVersion = "v1.27.0"

func migrateDeprecatedStorageConfig(sec ConfigSection) {
	if sec == nil {
		return
	}
	if sec.HasKey("STORAGE_TYPE") && sec.Key("STORAGE_TYPE").String() == string(legacyMinioStorageType) {
		LogStartupProblem(1, log.WARN, "Deprecation: config option `[%s].STORAGE_TYPE = %s` is deprecated, please use `%s` instead because this fallback will be removed in %s", sec.Name(), legacyMinioStorageType, S3StorageType, MinioToS3RemovalVersion)
		sec.Key("STORAGE_TYPE").SetValue(string(S3StorageType))
	}
	for _, r := range minioToS3KeyRenames {
		if !sec.HasKey(r.oldKey) {
			continue
		}
		LogStartupProblem(1, log.WARN, "Deprecation: config option `[%s].%s` present, please use `[%s].%s` instead because this fallback will be removed in %s", sec.Name(), r.oldKey, sec.Name(), r.newKey, MinioToS3RemovalVersion)
		if !sec.HasKey(r.newKey) {
			sec.Key(r.newKey).SetValue(sec.Key(r.oldKey).String())
		}
		sec.DeleteKey(r.oldKey)
	}
}

func (cfg *S3StorageConfig) ToShadow() {
	if cfg.AccessKeyID != "" {
		cfg.AccessKeyID = "******"
	}
	if cfg.SecretAccessKey != "" {
		cfg.SecretAccessKey = "******"
	}
}

// AzureBlobStorageConfig represents the configuration for an Azure Blob storage
type AzureBlobStorageConfig struct {
	Endpoint    string `ini:"AZURE_BLOB_ENDPOINT" json:",omitempty"`
	AccountName string `ini:"AZURE_BLOB_ACCOUNT_NAME" json:",omitempty"`
	AccountKey  string `ini:"AZURE_BLOB_ACCOUNT_KEY" json:",omitempty"`
	Container   string `ini:"AZURE_BLOB_CONTAINER" json:",omitempty"`
	BasePath    string `ini:"AZURE_BLOB_BASE_PATH" json:",omitempty"`
	ServeDirect bool   `ini:"SERVE_DIRECT"`
}

func (cfg *AzureBlobStorageConfig) ToShadow() {
	if cfg.AccountKey != "" {
		cfg.AccountKey = "******"
	}
	if cfg.AccountName != "" {
		cfg.AccountName = "******"
	}
}

// Storage represents configuration of storages
type Storage struct {
	Type            StorageType            // local or s3 or azureblob
	Path            string                 `json:",omitempty"` // for local type
	TemporaryPath   string                 `json:",omitempty"`
	S3Config        S3StorageConfig        // for s3 type
	AzureBlobConfig AzureBlobStorageConfig // for azureblob type
}

func (storage *Storage) ToShadowCopy() Storage {
	shadowStorage := *storage
	shadowStorage.S3Config.ToShadow()
	shadowStorage.AzureBlobConfig.ToShadow()
	return shadowStorage
}

func (storage *Storage) ServeDirect() bool {
	return (storage.Type == S3StorageType && storage.S3Config.ServeDirect) ||
		(storage.Type == AzureBlobStorageType && storage.AzureBlobConfig.ServeDirect)
}

const storageSectionName = "storage"

func getDefaultStorageSection(rootCfg ConfigProvider) ConfigSection {
	storageSec := rootCfg.Section(storageSectionName)
	migrateDeprecatedStorageConfig(storageSec)
	// Global Defaults
	storageSec.Key("STORAGE_TYPE").MustString("local")
	storageSec.Key("S3_ENDPOINT").MustString("localhost:9000")
	storageSec.Key("S3_ACCESS_KEY_ID").MustString("")
	storageSec.Key("S3_SECRET_ACCESS_KEY").MustString("")
	storageSec.Key("S3_BUCKET").MustString("gitea")
	storageSec.Key("S3_LOCATION").MustString("us-east-1")
	storageSec.Key("S3_USE_SSL").MustBool(false)
	storageSec.Key("S3_INSECURE_SKIP_VERIFY").MustBool(false)
	storageSec.Key("S3_CHECKSUM_ALGORITHM").MustString("default")
	storageSec.Key("S3_BUCKET_LOOKUP_TYPE").MustString("auto")
	storageSec.Key("AZURE_BLOB_ENDPOINT").MustString("")
	storageSec.Key("AZURE_BLOB_ACCOUNT_NAME").MustString("")
	storageSec.Key("AZURE_BLOB_ACCOUNT_KEY").MustString("")
	storageSec.Key("AZURE_BLOB_CONTAINER").MustString("gitea")
	return storageSec
}

// getStorage will find target section and extra special section first and then read override
// items from extra section
func getStorage(rootCfg ConfigProvider, name, typ string, sec ConfigSection) (*Storage, error) {
	if name == "" {
		return nil, errors.New("no name for storage")
	}

	migrateDeprecatedStorageConfig(sec)
	targetSec, tp, err := getStorageTargetSection(rootCfg, name, typ, sec)
	if err != nil {
		return nil, err
	}

	overrideSec := getStorageOverrideSection(rootCfg, sec, tp, name)
	migrateDeprecatedStorageConfig(targetSec)
	migrateDeprecatedStorageConfig(overrideSec)

	targetType := targetSec.Key("STORAGE_TYPE").String()
	switch targetType {
	case string(LocalStorageType):
		return getStorageForLocal(targetSec, overrideSec, tp, name)
	case string(S3StorageType):
		return getStorageForS3(targetSec, overrideSec, tp, name)
	case string(AzureBlobStorageType):
		return getStorageForAzureBlob(targetSec, overrideSec, tp, name)
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

func getStorageSectionByType(rootCfg ConfigProvider, typ string) (ConfigSection, targetSecType, error) { //nolint:unparam // FIXME: targetSecType is always 0, wrong design?
	targetSec, err := rootCfg.GetSection(storageSectionName + "." + typ)
	if err != nil && typ == string(S3StorageType) {
		// fall back to the legacy section name [storage.minio]
		if legacySec, legacyErr := rootCfg.GetSection(storageSectionName + "." + string(legacyMinioStorageType)); legacyErr == nil {
			LogStartupProblem(0, log.WARN, "Deprecation: storage section `[%s.%s]` is deprecated, please rename it to `[%s.%s]` because this fallback will be removed in %s", storageSectionName, legacyMinioStorageType, storageSectionName, S3StorageType, MinioToS3RemovalVersion)
			targetSec, err = legacySec, nil
		}
	}
	if err != nil {
		if !IsValidStorageType(StorageType(typ)) {
			return nil, 0, fmt.Errorf("get section via storage type %q failed: %v", typ, err)
		}
		// if typ is a valid storage type, but there is no [storage.local] or [storage.s3] section
		// it's not an error
		return nil, 0, nil
	}
	migrateDeprecatedStorageConfig(targetSec)

	targetType := targetSec.Key("STORAGE_TYPE").String()
	if targetType == "" {
		if !IsValidStorageType(StorageType(typ)) {
			return nil, 0, fmt.Errorf("unknown storage type %q", typ)
		}
		targetSec.Key("STORAGE_TYPE").SetValue(typ)
	} else if !IsValidStorageType(StorageType(targetType)) {
		return nil, 0, fmt.Errorf("unknown storage type %q for section storage.%v", targetType, typ)
	}

	return targetSec, targetSecIsTyp, nil
}

func getStorageTargetSection(rootCfg ConfigProvider, name, typ string, sec ConfigSection) (ConfigSection, targetSecType, error) {
	// check typ first
	if typ == "" {
		if sec != nil { // check sec's type secondly
			typ = sec.Key("STORAGE_TYPE").String()
			if IsValidStorageType(StorageType(typ)) {
				if targetSec, _, _ := getStorageSectionByType(rootCfg, typ); targetSec == nil {
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

	// check storage name thirdly
	targetSec, _ := rootCfg.GetSection(storageSectionName + "." + name)
	if targetSec != nil {
		targetType := targetSec.Key("STORAGE_TYPE").String()
		switch targetType {
		case "":
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

// getStorageOverrideSection override section will be read SERVE_DIRECT, PATH, S3_BASE_PATH, S3_BUCKET to override the targetsec when possible
func getStorageOverrideSection(rootConfig ConfigProvider, sec ConfigSection, targetSecType targetSecType, name string) ConfigSection {
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

	checkOverlappedPath("[storage."+name+"].PATH", storage.Path)

	return &storage, nil
}

func getStorageForS3(targetSec, overrideSec ConfigSection, tp targetSecType, name string) (*Storage, error) {
	storage := Storage{Type: S3StorageType}
	if err := targetSec.MapTo(&storage.S3Config); err != nil {
		return nil, fmt.Errorf("map S3 config failed: %v", err)
	}

	var defaultPath string
	if storage.S3Config.BasePath != "" {
		if tp == targetSecIsStorage || tp == targetSecIsDefault {
			defaultPath = strings.TrimSuffix(storage.S3Config.BasePath, "/") + "/" + name + "/"
		} else {
			defaultPath = storage.S3Config.BasePath
		}
	}
	if defaultPath == "" {
		defaultPath = name + "/"
	}

	if overrideSec != nil {
		storage.S3Config.ServeDirect = ConfigSectionKeyBool(overrideSec, "SERVE_DIRECT", storage.S3Config.ServeDirect)
		storage.S3Config.BasePath = ConfigSectionKeyString(overrideSec, "S3_BASE_PATH", defaultPath)
		storage.S3Config.Bucket = ConfigSectionKeyString(overrideSec, "S3_BUCKET", storage.S3Config.Bucket)
	} else {
		storage.S3Config.BasePath = defaultPath
	}
	return &storage, nil
}

func getStorageForAzureBlob(targetSec, overrideSec ConfigSection, tp targetSecType, name string) (*Storage, error) {
	var storage Storage
	storage.Type = StorageType(targetSec.Key("STORAGE_TYPE").String())
	if err := targetSec.MapTo(&storage.AzureBlobConfig); err != nil {
		return nil, fmt.Errorf("map azure blob config failed: %v", err)
	}

	var defaultPath string
	if storage.AzureBlobConfig.BasePath != "" {
		if tp == targetSecIsStorage || tp == targetSecIsDefault {
			defaultPath = strings.TrimSuffix(storage.AzureBlobConfig.BasePath, "/") + "/" + name + "/"
		} else {
			defaultPath = storage.AzureBlobConfig.BasePath
		}
	}
	if defaultPath == "" {
		defaultPath = name + "/"
	}

	if overrideSec != nil {
		storage.AzureBlobConfig.ServeDirect = ConfigSectionKeyBool(overrideSec, "SERVE_DIRECT", storage.AzureBlobConfig.ServeDirect)
		storage.AzureBlobConfig.BasePath = ConfigSectionKeyString(overrideSec, "AZURE_BLOB_BASE_PATH", defaultPath)
		storage.AzureBlobConfig.Container = ConfigSectionKeyString(overrideSec, "AZURE_BLOB_CONTAINER", storage.AzureBlobConfig.Container)
	} else {
		storage.AzureBlobConfig.BasePath = defaultPath
	}
	return &storage, nil
}
