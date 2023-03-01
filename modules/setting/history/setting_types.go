// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package history

import (
	"strings"
)

type SettingsSource string

const (
	SettingsSourceINI SettingsSource = "ini config file"
	SettingsSourceDB  SettingsSource = "database table 'system setting'"
)

var settingsSources []SettingsSource = []SettingsSource{SettingsSourceINI, SettingsSourceDB}

type setting interface {
	String() string  // Returns a string representation of the given setting that can be found like that in the given settings source
	Section() string //  If a type doesn't support sectioning, use "" as the global scope
	// Returns the un-normalized section this setting belongs to as passed in the init method of this package.
	// There might be settings that don't conform with the normalization (which is why they are replaced)

	Key() string // The un-normalized key of this setting
	IsNormalized() bool
	Normalize()
	Source() SettingsSource // What type is it contained in? app.ini, db, …
}

// A setting that is contained in the app.ini
type iniSetting struct {
	section, key                     string
	normalizedSection, normalizedKey string
	isNormalized                     bool
}

var _ setting = &iniSetting{}

func (s *iniSetting) Normalize() {
	if s.IsNormalized() {
		return
	}
	s.normalizedSection = strings.ToLower(s.section)
	s.normalizedKey = strings.ToUpper(s.key)
	s.isNormalized = true
}

func (s *iniSetting) IsNormalized() bool {
	return s.isNormalized
}

func (s *iniSetting) String() string {
	s.Normalize()
	return "[" + s.normalizedSection + "]." + s.normalizedKey
}

func (s *iniSetting) Section() string {
	s.Normalize()
	return s.section
}

func (s *iniSetting) Key() string {
	s.Normalize()
	return s.key
}

func (_ *iniSetting) Source() SettingsSource {
	return SettingsSourceINI
}

// A setting that is stored in the database
type dbSetting struct {
	section, key                     string
	normalizedSection, normalizedKey string
	isNormalized                     bool
}

var _ setting = &dbSetting{}

func (s *dbSetting) Normalize() {
	if s.IsNormalized() {
		return
	}
	s.normalizedSection = strings.ToLower(s.section)
	s.normalizedKey = strings.ToLower(s.key)
	s.isNormalized = true
}

func (s *dbSetting) IsNormalized() bool {
	return s.isNormalized
}

func (s *dbSetting) String() string {
	s.Normalize()
	return s.section + "." + s.key
}

func (s *dbSetting) Section() string {
	s.Normalize()
	return s.section
}

func (s *dbSetting) Key() string {
	s.Normalize()
	return s.key
}

func (_ *dbSetting) Source() SettingsSource {
	return SettingsSourceDB
}
