// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_24 //nolint

import (
	"fmt"
	"math"
	"strconv"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"xorm.io/xorm"
)

const keyRevision = "revision"

type Setting struct {
	ID           int64              `xorm:"pk autoincr"`
	SettingKey   string             `xorm:"varchar(255) unique"` // key should be lowercase
	SettingValue string             `xorm:"text"`
	Version      int                `xorm:"version"`
	Created      timeutil.TimeStamp `xorm:"created"`
	Updated      timeutil.TimeStamp `xorm:"updated"`
}

// TableName sets the table name for the settings struct
func (s *Setting) TableName() string {
	return "system_setting"
}

func MigrateIniToDatabase(x *xorm.Engine) error {
	uiMap := make(map[string]string)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("EXPLORE_PAGING_NUM"))] = strconv.Itoa(setting.ExplorePagingNum)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("SITEMAP_PAGING_NUM"))] = strconv.Itoa(setting.SitemapPagingNum)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("ISSUE_PAGING_NUM"))] = strconv.Itoa(setting.IssuePagingNum)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("REPO_SEARCH_PAGING_NUM"))] = strconv.Itoa(setting.RepoSearchPagingNum)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("MEMBERS_PAGING_NUM"))] = strconv.Itoa(setting.MembersPagingNum)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("FEED_MAX_COMMIT_NUM"))] = strconv.Itoa(setting.FeedMaxCommitNum)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("FEED_PAGING_NUM"))] = strconv.Itoa(setting.FeedPagingNum)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("PACKAGES_PAGING_NUM"))] = strconv.Itoa(setting.PackagesPagingNum)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("CODE_COMMENT_LINES"))] = strconv.Itoa(setting.CodeCommentLines)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("SHOW_USER_EMAIL"))] = strconv.FormatBool(setting.ShowUserEmail)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("SEARCH_REPO_DESCRIPTION"))] = strconv.FormatBool(setting.SearchRepoDescription)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("ONLY_SHOW_RELEVANT_REPOS"))] = strconv.FormatBool(setting.OnlyShowRelevantRepos)
	uiMap[fmt.Sprintf("ui.%s", util.ToSnakeCase("EXPLORE_PAGING_DEFAULT_SORT"))] = fmt.Sprintf("\"%s\"", setting.ExploreDefaultSort)

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if err := sess.Sync(new(Setting)); err != nil {
		return err
	}

	_ = getRevision(sess) // prepare the "revision" key ahead

	if _, err := sess.Exec("UPDATE system_setting SET version=version+1 WHERE setting_key=?", keyRevision); err != nil {
		return err
	}
	for k, v := range uiMap {
		res, err := sess.Exec("UPDATE system_setting SET version=version+1, setting_value=? WHERE setting_key=?", v, k)
		if err != nil {
			return err
		}
		rows, _ := res.RowsAffected()
		if rows == 0 { // if no existing row, insert a new row
			if _, err = sess.Insert(&Setting{SettingKey: k, SettingValue: v}); err != nil {
				return err
			}
		}
	}

	return sess.Commit()
}

func getRevision(sess *xorm.Session) int {
	revision := &Setting{}
	exist, err := sess.Where("setting_key = ?", keyRevision).Get(revision)
	if err != nil {
		return 0
	} else if !exist {
		_, err = sess.Insert(&Setting{SettingKey: keyRevision, Version: 1})
		if err != nil {
			return 0
		}
		return 1
	}

	if revision.Version <= 0 || revision.Version >= math.MaxInt-1 {
		_, err = sess.Exec("UPDATE system_setting SET version=1 WHERE setting_key=?", keyRevision)
		if err != nil {
			return 0
		}
		return 1
	}
	return revision.Version
}
