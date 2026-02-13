// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11

import (
	"net/url"

	"xorm.io/xorm"
)

func SanitizeOriginalURL(x *xorm.Engine) error {
	type Repository struct {
		ID          int64
		OriginalURL string `xorm:"VARCHAR(2048)"`
	}

	var last int
	const batchSize = 50
	for {
		results := make([]Repository, 0, batchSize)
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

		for _, res := range results {
			u, err := url.Parse(res.OriginalURL)
			if err != nil {
				// it is ok to continue here, we only care about fixing URLs that we can read
				continue
			}
			u.User = nil
			originalURL := u.String()
			_, err = x.Exec("UPDATE repository SET original_url = ? WHERE id = ?", originalURL, res.ID)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
