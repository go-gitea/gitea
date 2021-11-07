// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"bytes"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/builder"
)

// #16831 revealed that the dump command that was broken in 1.14.3-1.14.6 and 1.15.0 (#15885).
// This led to repo_unit and login_source cfg not being converted to JSON in the dump
// Unfortunately although it was hoped that there were only a few users affected it
// appears that many users are affected.

// We therefore need to provide a doctor command to fix this repeated issue #16961

func parseBool16961(bs []byte) (bool, error) {
	if bytes.EqualFold(bs, []byte("%!s(bool=false)")) {
		return false, nil
	}

	if bytes.EqualFold(bs, []byte("%!s(bool=true)")) {
		return true, nil
	}

	return false, fmt.Errorf("unexpected bool format: %s", string(bs))
}

func fixUnitConfig16961(bs []byte, cfg *models.UnitConfig) (fixed bool, err error) {
	err = models.JSONUnmarshalHandleDoubleEncode(bs, &cfg)
	if err == nil {
		return
	}

	// Handle #16961
	if string(bs) != "&{}" && len(bs) != 0 {
		return
	}

	return true, nil
}

func fixExternalWikiConfig16961(bs []byte, cfg *models.ExternalWikiConfig) (fixed bool, err error) {
	err = models.JSONUnmarshalHandleDoubleEncode(bs, &cfg)
	if err == nil {
		return
	}

	if len(bs) < 3 {
		return
	}
	if bs[0] != '&' || bs[1] != '{' || bs[len(bs)-1] != '}' {
		return
	}
	cfg.ExternalWikiURL = string(bs[2 : len(bs)-1])
	return true, nil
}

func fixExternalTrackerConfig16961(bs []byte, cfg *models.ExternalTrackerConfig) (fixed bool, err error) {
	err = models.JSONUnmarshalHandleDoubleEncode(bs, &cfg)
	if err == nil {
		return
	}
	// Handle #16961
	if len(bs) < 3 {
		return
	}

	if bs[0] != '&' || bs[1] != '{' || bs[len(bs)-1] != '}' {
		return
	}

	parts := bytes.Split(bs[2:len(bs)-1], []byte{' '})
	if len(parts) != 3 {
		return
	}

	cfg.ExternalTrackerURL = string(bytes.Join(parts[:len(parts)-2], []byte{' '}))
	cfg.ExternalTrackerFormat = string(parts[len(parts)-2])
	cfg.ExternalTrackerStyle = string(parts[len(parts)-1])
	return true, nil
}

func fixPullRequestsConfig16961(bs []byte, cfg *models.PullRequestsConfig) (fixed bool, err error) {
	err = models.JSONUnmarshalHandleDoubleEncode(bs, &cfg)
	if err == nil {
		return
	}

	// Handle #16961
	if len(bs) < 3 {
		return
	}

	if bs[0] != '&' || bs[1] != '{' || bs[len(bs)-1] != '}' {
		return
	}

	// PullRequestsConfig was the following in 1.14
	// type PullRequestsConfig struct {
	// 	IgnoreWhitespaceConflicts bool
	// 	AllowMerge                bool
	// 	AllowRebase               bool
	// 	AllowRebaseMerge          bool
	// 	AllowSquash               bool
	// 	AllowManualMerge          bool
	// 	AutodetectManualMerge     bool
	// }
	//
	// 1.15 added in addition:
	// DefaultDeleteBranchAfterMerge bool
	// DefaultMergeStyle             MergeStyle
	parts := bytes.Split(bs[2:len(bs)-1], []byte{' '})
	if len(parts) < 7 {
		return
	}

	var parseErr error
	cfg.IgnoreWhitespaceConflicts, parseErr = parseBool16961(parts[0])
	if parseErr != nil {
		return
	}
	cfg.AllowMerge, parseErr = parseBool16961(parts[1])
	if parseErr != nil {
		return
	}
	cfg.AllowRebase, parseErr = parseBool16961(parts[2])
	if parseErr != nil {
		return
	}
	cfg.AllowRebaseMerge, parseErr = parseBool16961(parts[3])
	if parseErr != nil {
		return
	}
	cfg.AllowSquash, parseErr = parseBool16961(parts[4])
	if parseErr != nil {
		return
	}
	cfg.AllowManualMerge, parseErr = parseBool16961(parts[5])
	if parseErr != nil {
		return
	}
	cfg.AutodetectManualMerge, parseErr = parseBool16961(parts[6])
	if parseErr != nil {
		return
	}

	// 1.14 unit
	if len(parts) == 7 {
		return true, nil
	}

	if len(parts) < 9 {
		return
	}

	cfg.DefaultDeleteBranchAfterMerge, parseErr = parseBool16961(parts[7])
	if parseErr != nil {
		return
	}

	cfg.DefaultMergeStyle = models.MergeStyle(string(bytes.Join(parts[8:], []byte{' '})))
	return true, nil
}

func fixIssuesConfig16961(bs []byte, cfg *models.IssuesConfig) (fixed bool, err error) {
	err = models.JSONUnmarshalHandleDoubleEncode(bs, &cfg)
	if err == nil {
		return
	}

	// Handle #16961
	if len(bs) < 3 {
		return
	}

	if bs[0] != '&' || bs[1] != '{' || bs[len(bs)-1] != '}' {
		return
	}

	parts := bytes.Split(bs[2:len(bs)-1], []byte{' '})
	if len(parts) != 3 {
		return
	}
	var parseErr error
	cfg.EnableTimetracker, parseErr = parseBool16961(parts[0])
	if parseErr != nil {
		return
	}
	cfg.AllowOnlyContributorsToTrackTime, parseErr = parseBool16961(parts[1])
	if parseErr != nil {
		return
	}
	cfg.EnableDependencies, parseErr = parseBool16961(parts[2])
	if parseErr != nil {
		return
	}
	return true, nil
}

func fixBrokenRepoUnit16961(repoUnit *models.RepoUnit, bs []byte) (fixed bool, err error) {
	// Shortcut empty or null values
	if len(bs) == 0 {
		return false, nil
	}

	switch unit.Type(repoUnit.Type) {
	case unit.TypeCode, unit.TypeReleases, unit.TypeWiki, unit.TypeProjects:
		cfg := &models.UnitConfig{}
		repoUnit.Config = cfg
		if fixed, err := fixUnitConfig16961(bs, cfg); !fixed {
			return false, err
		}
	case unit.TypeExternalWiki:
		cfg := &models.ExternalWikiConfig{}
		repoUnit.Config = cfg

		if fixed, err := fixExternalWikiConfig16961(bs, cfg); !fixed {
			return false, err
		}
	case unit.TypeExternalTracker:
		cfg := &models.ExternalTrackerConfig{}
		repoUnit.Config = cfg
		if fixed, err := fixExternalTrackerConfig16961(bs, cfg); !fixed {
			return false, err
		}
	case unit.TypePullRequests:
		cfg := &models.PullRequestsConfig{}
		repoUnit.Config = cfg

		if fixed, err := fixPullRequestsConfig16961(bs, cfg); !fixed {
			return false, err
		}
	case unit.TypeIssues:
		cfg := &models.IssuesConfig{}
		repoUnit.Config = cfg
		if fixed, err := fixIssuesConfig16961(bs, cfg); !fixed {
			return false, err
		}
	default:
		panic(fmt.Sprintf("unrecognized repo unit type: %v", repoUnit.Type))
	}
	return true, nil
}

func fixBrokenRepoUnits16961(logger log.Logger, autofix bool) error {
	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		ID          int64
		RepoID      int64
		Type        unit.Type
		Config      []byte
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX CREATED"`
	}

	count := 0

	err := db.Iterate(
		db.DefaultContext,
		new(RepoUnit),
		builder.Gt{
			"id": 0,
		},
		func(idx int, bean interface{}) error {
			unit := bean.(*RepoUnit)

			bs := unit.Config
			repoUnit := &models.RepoUnit{
				ID:          unit.ID,
				RepoID:      unit.RepoID,
				Type:        unit.Type,
				CreatedUnix: unit.CreatedUnix,
			}

			if fixed, err := fixBrokenRepoUnit16961(repoUnit, bs); !fixed {
				return err
			}

			count++
			if !autofix {
				return nil
			}

			return models.UpdateRepoUnit(repoUnit)
		},
	)

	if err != nil {
		logger.Critical("Unable to iterate acrosss repounits to fix the broken units: Error %v", err)
		return err
	}

	if !autofix {
		logger.Warn("Found %d broken repo_units", count)
		return nil
	}
	logger.Info("Fixed %d broken repo_units", count)

	return nil
}

func init() {
	Register(&Check{
		Title:     "Check for incorrectly dumped repo_units (See #16961)",
		Name:      "fix-broken-repo-units",
		IsDefault: false,
		Run:       fixBrokenRepoUnits16961,
		Priority:  7,
	})
}
