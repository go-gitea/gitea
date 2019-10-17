// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"net/url"
	"strings"
	"time"

	"xorm.io/xorm"
)

func updateMigrationServiceTypes(x *xorm.Engine) error {
	type Repository struct {
		ID                  int64
		OriginalServiceType int    `xorm:"index default(0)"`
		OriginalURL         string `xorm:"VARCHAR(2048)"`
	}

	if err := x.Sync2(new(Repository)); err != nil {
		return err
	}

	var last int
	const batchSize = 50
	for {
		var results = make([]Repository, 0, batchSize)
		err := x.Where("original_url <> '' AND original_url IS NOT NULL").
			And("original_service_type = 0 OR original_service_type IS NULL").
			OrderBy("id").
			Limit(batchSize, last).
			Find(&results)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			break
		}
		last += len(results)

		const PlainGitService = 1 // 1 plain git service
		const GithubService = 2   // 2 github.com

		for _, res := range results {
			u, err := url.Parse(res.OriginalURL)
			if err != nil {
				return err
			}
			var serviceType = PlainGitService
			if strings.EqualFold(u.Host, "github.com") {
				serviceType = GithubService
			}
			_, err = x.Exec("UPDATE repository SET original_service_type = ? WHERE id = ?", serviceType, res.ID)
			if err != nil {
				return err
			}
		}
	}

	type ExternalLoginUser struct {
		ExternalID        string                 `xorm:"pk NOT NULL"`
		UserID            int64                  `xorm:"INDEX NOT NULL"`
		LoginSourceID     int64                  `xorm:"pk NOT NULL"`
		RawData           map[string]interface{} `xorm:"TEXT JSON"`
		Provider          string                 `xorm:"index VARCHAR(25)"`
		Email             string
		Name              string
		FirstName         string
		LastName          string
		NickName          string
		Description       string
		AvatarURL         string
		Location          string
		AccessToken       string
		AccessTokenSecret string
		RefreshToken      string
		ExpiresAt         time.Time
	}

	return x.Sync2(new(ExternalLoginUser))
}
