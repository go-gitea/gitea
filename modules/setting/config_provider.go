// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	ini "gopkg.in/ini.v1"
)

type ConfigSection interface {
	Name() string
	MapTo(interface{}) error
	HasKey(key string) bool
	NewKey(name, value string) (*ini.Key, error)
	Key(key string) *ini.Key
	Keys() []*ini.Key
	ChildSections() []*ini.Section
}

// ConfigProvider represents a config provider
type ConfigProvider interface {
	Section(section string) ConfigSection
	NewSection(name string) (ConfigSection, error)
	GetSection(name string) (ConfigSection, error)
	DeleteSection(name string) error
	Save() error
}

type iniFileConfigProvider struct {
	*ini.File
	filepath   string // the ini file path
	newFile    bool   // whether the file has not existed previously
	allowEmpty bool   // whether not finding configuration files is allowed (only true for the tests)
}

// NewEmptyConfigProvider create a new empty config provider
func NewEmptyConfigProvider() ConfigProvider {
	cp, _ := newConfigProviderFromData("")
	return cp
}

// newConfigProviderFromData this function is only for testing
func newConfigProviderFromData(configContent string) (ConfigProvider, error) {
	var cfg *ini.File
	var err error
	if configContent == "" {
		cfg = ini.Empty()
	} else {
		cfg, err = ini.Load(strings.NewReader(configContent))
		if err != nil {
			return nil, err
		}
	}
	cfg.NameMapper = ini.SnackCase
	return &iniFileConfigProvider{
		File:    cfg,
		newFile: true,
	}, nil
}

// newConfigProviderFromFile load configuration from file.
// NOTE: do not print any log except error.
func newConfigProviderFromFile(customConf string, allowEmpty bool, extraConfig string) (*iniFileConfigProvider, error) {
	cfg := ini.Empty()
	newFile := true

	if customConf != "" {
		isFile, err := util.IsFile(customConf)
		if err != nil {
			return nil, fmt.Errorf("unable to check if %s is a file. Error: %v", customConf, err)
		}
		if isFile {
			if err := cfg.Append(customConf); err != nil {
				return nil, fmt.Errorf("failed to load custom conf '%s': %v", customConf, err)
			}
			newFile = false
		}
	}

	if newFile && !allowEmpty {
		return nil, fmt.Errorf("unable to find configuration file: %q, please ensure you are running in the correct environment or set the correct configuration file with -c", CustomConf)
	}

	if extraConfig != "" {
		if err := cfg.Append([]byte(extraConfig)); err != nil {
			return nil, fmt.Errorf("unable to append more config: %v", err)
		}
	}

	cfg.NameMapper = ini.SnackCase
	return &iniFileConfigProvider{
		File:       cfg,
		filepath:   customConf,
		newFile:    newFile,
		allowEmpty: allowEmpty,
	}, nil
}

func (p *iniFileConfigProvider) Section(section string) ConfigSection {
	return p.File.Section(section)
}

func (p *iniFileConfigProvider) NewSection(name string) (ConfigSection, error) {
	return p.File.NewSection(name)
}

func (p *iniFileConfigProvider) GetSection(name string) (ConfigSection, error) {
	return p.File.GetSection(name)
}

func (p *iniFileConfigProvider) DeleteSection(name string) error {
	p.File.DeleteSection(name)
	return nil
}

// Save save the content into file
func (p *iniFileConfigProvider) Save() error {
	if p.filepath == "" {
		if !p.allowEmpty {
			return fmt.Errorf("custom config path must not be empty")
		}
		return nil
	}

	if p.newFile {
		if err := os.MkdirAll(filepath.Dir(CustomConf), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create '%s': %v", CustomConf, err)
		}
	}
	if err := p.SaveTo(p.filepath); err != nil {
		return fmt.Errorf("failed to save '%s': %v", p.filepath, err)
	}

	// Change permissions to be more restrictive
	fi, err := os.Stat(CustomConf)
	if err != nil {
		return fmt.Errorf("failed to determine current conf file permissions: %v", err)
	}

	if fi.Mode().Perm() > 0o600 {
		if err = os.Chmod(CustomConf, 0o600); err != nil {
			log.Warn("Failed changing conf file permissions to -rw-------. Consider changing them manually.")
		}
	}
	return nil
}

// a file is an implementation ConfigProvider and other implementations are possible, i.e. from docker, k8s, …
var _ ConfigProvider = &iniFileConfigProvider{}

func mustMapSetting(rootCfg ConfigProvider, sectionName string, setting interface{}) {
	if err := rootCfg.Section(sectionName).MapTo(setting); err != nil {
		log.Fatal("Failed to map %s settings: %v", sectionName, err)
	}
}

func deprecatedSetting(rootCfg ConfigProvider, oldSection, oldKey, newSection, newKey, version string) {
	if rootCfg.Section(oldSection).HasKey(oldKey) {
		log.Error("Deprecated fallback `[%s]` `%s` present. Use `[%s]` `%s` instead. This fallback will be/has been removed in %s", oldSection, oldKey, newSection, newKey, version)
	}
}

// deprecatedSettingDB add a hint that the configuration has been moved to database but still kept in app.ini
func deprecatedSettingDB(rootCfg ConfigProvider, oldSection, oldKey string) {
	if rootCfg.Section(oldSection).HasKey(oldKey) {
		log.Error("Deprecated `[%s]` `%s` present which has been copied to database table sys_setting", oldSection, oldKey)
	}
}
