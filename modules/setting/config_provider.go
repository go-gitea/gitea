// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"gopkg.in/ini.v1" //nolint:depguard
)

type ConfigKey interface {
	Name() string
	Value() string
	SetValue(v string)

	In(defaultVal string, candidates []string) string
	String() string
	Strings(delim string) []string

	MustString(defaultVal string) string
	MustBool(defaultVal ...bool) bool
	MustInt(defaultVal ...int) int
	MustInt64(defaultVal ...int64) int64
	MustDuration(defaultVal ...time.Duration) time.Duration
}

type ConfigSection interface {
	Name() string
	MapTo(any) error
	HasKey(key string) bool
	NewKey(name, value string) (ConfigKey, error)
	Key(key string) ConfigKey
	Keys() []ConfigKey
	ChildSections() []ConfigSection
}

// ConfigProvider represents a config provider
type ConfigProvider interface {
	Section(section string) ConfigSection
	Sections() []ConfigSection
	NewSection(name string) (ConfigSection, error)
	GetSection(name string) (ConfigSection, error)
	Save() error
	SaveTo(filename string) error

	DisableSaving()
	PrepareSaving() (ConfigProvider, error)
	IsLoadedFromEmpty() bool
}

type iniConfigProvider struct {
	file string
	ini  *ini.File

	disableSaving   bool // disable the "Save" method because the config options could be polluted
	loadedFromEmpty bool // whether the file has not existed previously
}

type iniConfigSection struct {
	sec *ini.Section
}

var (
	_ ConfigProvider = (*iniConfigProvider)(nil)
	_ ConfigSection  = (*iniConfigSection)(nil)
	_ ConfigKey      = (*ini.Key)(nil)
)

// ConfigSectionKey only searches the keys in the given section, but it is O(n).
// ini package has a special behavior:  with "[sec] a=1" and an empty "[sec.sub]",
// then in "[sec.sub]", Key()/HasKey() can always see "a=1" because it always tries parent sections.
// It returns nil if the key doesn't exist.
func ConfigSectionKey(sec ConfigSection, key string) ConfigKey {
	if sec == nil {
		return nil
	}
	for _, k := range sec.Keys() {
		if k.Name() == key {
			return k
		}
	}
	return nil
}

func ConfigSectionKeyString(sec ConfigSection, key string, def ...string) string {
	k := ConfigSectionKey(sec, key)
	if k != nil && k.String() != "" {
		return k.String()
	}
	if len(def) > 0 {
		return def[0]
	}
	return ""
}

func ConfigSectionKeyBool(sec ConfigSection, key string, def ...bool) bool {
	k := ConfigSectionKey(sec, key)
	if k != nil && k.String() != "" {
		b, _ := strconv.ParseBool(k.String())
		return b
	}
	if len(def) > 0 {
		return def[0]
	}
	return false
}

// ConfigInheritedKey works like ini.Section.Key(), but it always returns a new key instance, it is O(n) because NewKey is O(n)
// and the returned key is safe to be used with "MustXxx", it doesn't change the parent's values.
// Otherwise, ini.Section.Key().MustXxx would pollute the parent section's keys.
// It never returns nil.
func ConfigInheritedKey(sec ConfigSection, key string) ConfigKey {
	k := sec.Key(key)
	if k != nil && k.String() != "" {
		newKey, _ := sec.NewKey(k.Name(), k.String())
		return newKey
	}
	newKey, _ := sec.NewKey(key, "")
	return newKey
}

func ConfigInheritedKeyString(sec ConfigSection, key string, def ...string) string {
	k := sec.Key(key)
	if k != nil && k.String() != "" {
		return k.String()
	}
	if len(def) > 0 {
		return def[0]
	}
	return ""
}

func (s *iniConfigSection) Name() string {
	return s.sec.Name()
}

func (s *iniConfigSection) MapTo(v any) error {
	return s.sec.MapTo(v)
}

func (s *iniConfigSection) HasKey(key string) bool {
	return s.sec.HasKey(key)
}

func (s *iniConfigSection) NewKey(name, value string) (ConfigKey, error) {
	return s.sec.NewKey(name, value)
}

func (s *iniConfigSection) Key(key string) ConfigKey {
	return s.sec.Key(key)
}

func (s *iniConfigSection) Keys() (keys []ConfigKey) {
	for _, k := range s.sec.Keys() {
		keys = append(keys, k)
	}
	return keys
}

func (s *iniConfigSection) ChildSections() (sections []ConfigSection) {
	for _, s := range s.sec.ChildSections() {
		sections = append(sections, &iniConfigSection{s})
	}
	return sections
}

func configProviderLoadOptions() ini.LoadOptions {
	return ini.LoadOptions{
		KeyValueDelimiterOnWrite: " = ",
		IgnoreContinuation:       true,
	}
}

// NewConfigProviderFromData this function is mainly for testing purpose
func NewConfigProviderFromData(configContent string) (ConfigProvider, error) {
	cfg, err := ini.LoadSources(configProviderLoadOptions(), strings.NewReader(configContent))
	if err != nil {
		return nil, err
	}
	cfg.NameMapper = ini.SnackCase
	return &iniConfigProvider{
		ini:             cfg,
		loadedFromEmpty: true,
	}, nil
}

// NewConfigProviderFromFile load configuration from file.
// NOTE: do not print any log except error.
func NewConfigProviderFromFile(file string, extraConfigs ...string) (ConfigProvider, error) {
	cfg := ini.Empty(configProviderLoadOptions())
	loadedFromEmpty := true

	if file != "" {
		isFile, err := util.IsFile(file)
		if err != nil {
			return nil, fmt.Errorf("unable to check if %q is a file. Error: %v", file, err)
		}
		if isFile {
			if err = cfg.Append(file); err != nil {
				return nil, fmt.Errorf("failed to load config file %q: %v", file, err)
			}
			loadedFromEmpty = false
		}
	}

	for _, s := range extraConfigs {
		if err := cfg.Append([]byte(s)); err != nil {
			return nil, fmt.Errorf("unable to append more config: %v", err)
		}
	}

	cfg.NameMapper = ini.SnackCase
	return &iniConfigProvider{
		file:            file,
		ini:             cfg,
		loadedFromEmpty: loadedFromEmpty,
	}, nil
}

func (p *iniConfigProvider) Section(section string) ConfigSection {
	return &iniConfigSection{sec: p.ini.Section(section)}
}

func (p *iniConfigProvider) Sections() (sections []ConfigSection) {
	for _, s := range p.ini.Sections() {
		sections = append(sections, &iniConfigSection{s})
	}
	return sections
}

func (p *iniConfigProvider) NewSection(name string) (ConfigSection, error) {
	sec, err := p.ini.NewSection(name)
	if err != nil {
		return nil, err
	}
	return &iniConfigSection{sec: sec}, nil
}

func (p *iniConfigProvider) GetSection(name string) (ConfigSection, error) {
	sec, err := p.ini.GetSection(name)
	if err != nil {
		return nil, err
	}
	return &iniConfigSection{sec: sec}, nil
}

var errDisableSaving = errors.New("this config can't be saved, developers should prepare a new config to save")

// Save saves the content into file
func (p *iniConfigProvider) Save() error {
	if p.disableSaving {
		return errDisableSaving
	}
	filename := p.file
	if filename == "" {
		return fmt.Errorf("config file path must not be empty")
	}
	if p.loadedFromEmpty {
		if err := os.MkdirAll(filepath.Dir(filename), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create %q: %v", filename, err)
		}
	}
	if err := p.ini.SaveTo(filename); err != nil {
		return fmt.Errorf("failed to save %q: %v", filename, err)
	}

	// Change permissions to be more restrictive
	fi, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("failed to determine current conf file permissions: %v", err)
	}

	if fi.Mode().Perm() > 0o600 {
		if err = os.Chmod(filename, 0o600); err != nil {
			log.Warn("Failed changing conf file permissions to -rw-------. Consider changing them manually.")
		}
	}
	return nil
}

func (p *iniConfigProvider) SaveTo(filename string) error {
	if p.disableSaving {
		return errDisableSaving
	}
	return p.ini.SaveTo(filename)
}

// DisableSaving disables the saving function, use PrepareSaving to get clear config options.
func (p *iniConfigProvider) DisableSaving() {
	p.disableSaving = true
}

// PrepareSaving loads the ini from file again to get clear config options.
// Otherwise, the "MustXxx" calls would have polluted the current config provider,
// it makes the "Save" outputs a lot of garbage options
// After the INI package gets refactored, no "MustXxx" pollution, this workaround can be dropped.
func (p *iniConfigProvider) PrepareSaving() (ConfigProvider, error) {
	if p.file == "" {
		return nil, errors.New("no config file to save")
	}
	return NewConfigProviderFromFile(p.file)
}

func (p *iniConfigProvider) IsLoadedFromEmpty() bool {
	return p.loadedFromEmpty
}

func mustMapSetting(rootCfg ConfigProvider, sectionName string, setting any) {
	if err := rootCfg.Section(sectionName).MapTo(setting); err != nil {
		log.Fatal("Failed to map %s settings: %v", sectionName, err)
	}
}

// DeprecatedWarnings contains the warning message for various deprecations, including: setting option, file/folder, etc
var DeprecatedWarnings []string

func deprecatedSetting(rootCfg ConfigProvider, oldSection, oldKey, newSection, newKey, version string) {
	if rootCfg.Section(oldSection).HasKey(oldKey) {
		msg := fmt.Sprintf("Deprecated config option `[%s]` `%s` present. Use `[%s]` `%s` instead. This fallback will be/has been removed in %s", oldSection, oldKey, newSection, newKey, version)
		log.Error("%v", msg)
		DeprecatedWarnings = append(DeprecatedWarnings, msg)
	}
}

// deprecatedSettingDB add a hint that the configuration has been moved to database but still kept in app.ini
func deprecatedSettingDB(rootCfg ConfigProvider, oldSection, oldKey string) {
	if rootCfg.Section(oldSection).HasKey(oldKey) {
		log.Error("Deprecated `[%s]` `%s` present which has been copied to database table sys_setting", oldSection, oldKey)
	}
}

// NewConfigProviderForLocale loads locale configuration from source and others. "string" if for a local file path, "[]byte" is for INI content
func NewConfigProviderForLocale(source any, others ...any) (ConfigProvider, error) {
	iniFile, err := ini.LoadSources(ini.LoadOptions{
		IgnoreInlineComment:         true,
		UnescapeValueCommentSymbols: true,
		IgnoreContinuation:          true,
	}, source, others...)
	if err != nil {
		return nil, fmt.Errorf("unable to load locale ini: %w", err)
	}
	iniFile.BlockMode = false
	return &iniConfigProvider{
		ini:             iniFile,
		loadedFromEmpty: true,
	}, nil
}

func init() {
	ini.PrettyFormat = false
}
