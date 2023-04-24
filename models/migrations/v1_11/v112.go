// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_11 //nolint

import (
	"fmt"
	"path/filepath"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
	"xorm.io/xorm"
)

func RemoveAttachmentMissedRepo(x *xorm.Engine) error {
	type Attachment struct {
		UUID string `xorm:"uuid"`
	}
	var start int
	attachments := make([]*Attachment, 0, 50)
	for {
		err := x.Select("uuid").Where(builder.NotIn("release_id", builder.Select("id").From("`release`"))).
			And("release_id > 0").
			OrderBy("id").Limit(50, start).Find(&attachments)
		if err != nil {
			return err
		}

		for i := 0; i < len(attachments); i++ {
			uuid := attachments[i].UUID
			if err = util.RemoveAll(filepath.Join(setting.Attachment.Path, uuid[0:1], uuid[1:2], uuid)); err != nil {
				fmt.Printf("Error: %v", err) //nolint:forbidigo
			}
		}

		if len(attachments) < 50 {
			break
		}
		start += 50
		attachments = attachments[:0]
	}

	_, err := x.Exec("DELETE FROM attachment WHERE release_id > 0 AND release_id NOT IN (SELECT id FROM `release`)")
	return err
}
