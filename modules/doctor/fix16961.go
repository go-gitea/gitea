// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"bytes"
	"fmt"
	"strconv"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/builder"
)

// #16831 revealed that the dump command that was broken in 1.14.3-1.14.6 and 1.15.0 (#15885).
// This led to repo_unit and login_source cfg not being converted to JSON in the dump
// Unfortunately although it was hoped that there were only a few users affected it
// appears that many users are affected.

// We therefore need to provide a doctor command to fix this repeated issue #16961

func fixBrokenRepoUnits(logger log.Logger, autofix bool) error {
	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		ID          int64
		RepoID      int64
		Type        models.UnitType
		Config      []byte
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX CREATED"`
	}

	count := 0

	err := models.Iterate(
		models.DefaultDBContext(),
		new(RepoUnit),
		builder.Eq{"1": "1"},
		func(idx int, bean interface{}) error {
			unit := bean.(*RepoUnit)

			bs := unit.Config
			repoUnit := &models.RepoUnit{
				ID:          unit.ID,
				RepoID:      unit.RepoID,
				Type:        unit.Type,
				CreatedUnix: unit.CreatedUnix,
			}

			switch models.UnitType(unit.Type) {
			case models.UnitTypeCode, models.UnitTypeReleases, models.UnitTypeWiki, models.UnitTypeProjects:
				cfg := &models.UnitConfig{}
				repoUnit.Config = cfg

				err := models.JSONUnmarshalHandleDoubleEncode(bs, &cfg)
				if err == nil {
					return nil
				}

				// Handle #16961
				if string(bs) != "&{}" {
					return err
				}
			case models.UnitTypeExternalWiki:
				cfg := &models.ExternalWikiConfig{}
				repoUnit.Config = cfg
				err := models.JSONUnmarshalHandleDoubleEncode(bs, &cfg)
				if err == nil {
					return nil
				}

				if len(bs) < 3 {
					return err
				}
				if bs[0] != '&' || bs[1] != '{' || bs[len(bs)-1] != '}' {
					return err
				}
				cfg.ExternalWikiURL = string(bs[2 : len(bs)-1])
			case models.UnitTypeExternalTracker:
				cfg := &models.ExternalTrackerConfig{}
				repoUnit.Config = cfg
				err := models.JSONUnmarshalHandleDoubleEncode(bs, &cfg)
				if err == nil {
					return nil
				}
				// Handle #16961
				if len(bs) < 3 {
					return err
				}

				if bs[0] != '&' || bs[1] != '{' || bs[len(bs)-1] != '}' {
					return err
				}

				parts := bytes.Split(bs[2:len(bs)-1], []byte{' '})
				if len(parts) != 3 {
					return err
				}

				cfg.ExternalTrackerURL = string(bytes.Join(parts[:len(parts)-2], []byte{' '}))
				cfg.ExternalTrackerFormat = string(parts[len(parts)-2])
				cfg.ExternalTrackerStyle = string(parts[len(parts)-1])
			case models.UnitTypePullRequests:
				cfg := &models.PullRequestsConfig{}
				repoUnit.Config = cfg
				err := models.JSONUnmarshalHandleDoubleEncode(bs, &cfg)
				if err == nil {
					return nil
				}

				// Handle #16961
				if len(bs) < 3 {
					return err
				}

				if bs[0] != '&' || bs[1] != '{' || bs[len(bs)-1] != '}' {
					return err
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
					return err
				}

				var parseErr error
				cfg.IgnoreWhitespaceConflicts, parseErr = strconv.ParseBool(string(parts[0]))
				if parseErr != nil {
					return err
				}
				cfg.AllowMerge, parseErr = strconv.ParseBool(string(parts[1]))
				if parseErr != nil {
					return err
				}
				cfg.AllowRebase, parseErr = strconv.ParseBool(string(parts[2]))
				if parseErr != nil {
					return err
				}
				cfg.AllowRebaseMerge, parseErr = strconv.ParseBool(string(parts[3]))
				if parseErr != nil {
					return err
				}
				cfg.AllowSquash, parseErr = strconv.ParseBool(string(parts[4]))
				if parseErr != nil {
					return err
				}
				cfg.AllowManualMerge, parseErr = strconv.ParseBool(string(parts[5]))
				if parseErr != nil {
					return err
				}
				cfg.AutodetectManualMerge, parseErr = strconv.ParseBool(string(parts[6]))
				if parseErr != nil {
					return err
				}

				// 1.14 unit
				if len(parts) == 7 {
					count++
					if !autofix {
						return nil
					}
					return models.UpdateRepoUnit(repoUnit)
				}

				if len(parts) < 9 {
					return err
				}

				cfg.DefaultDeleteBranchAfterMerge, parseErr = strconv.ParseBool(string(parts[7]))
				if parseErr != nil {
					return err
				}

				cfg.DefaultMergeStyle = models.MergeStyle(string(bytes.Join(parts[8:], []byte{' '})))
			case models.UnitTypeIssues:
				cfg := &models.IssuesConfig{}
				repoUnit.Config = cfg
				err := models.JSONUnmarshalHandleDoubleEncode(bs, &cfg)
				if err == nil {
					return nil
				}

				// Handle #16961
				if len(bs) < 3 {
					return err
				}

				if bs[0] != '&' || bs[1] != '{' || bs[len(bs)-1] != '}' {
					return err
				}

				parts := bytes.Split(bs[2:len(bs)-1], []byte{' '})
				if len(parts) != 3 {
					return err
				}
				var parseErr error
				cfg.EnableTimetracker, parseErr = strconv.ParseBool(string(parts[0]))
				if parseErr != nil {
					return err
				}
				cfg.AllowOnlyContributorsToTrackTime, parseErr = strconv.ParseBool(string(parts[1]))
				if parseErr != nil {
					return err
				}
				cfg.EnableDependencies, parseErr = strconv.ParseBool(string(parts[2]))
				if parseErr != nil {
					return err
				}
			default:
				panic(fmt.Sprintf("unrecognized repo unit type: %v", unit.Type))
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
		Run:       fixBrokenRepoUnits,
		Priority:  7,
	})
}
