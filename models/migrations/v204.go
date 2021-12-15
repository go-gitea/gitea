// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addResolveReasonForComment(x *xorm.Engine) error {
	// ResolveReason represents a reason for why a comment was resolved.
	type ResolveReason int

	type Comment struct {
		ResolveReason ResolveReason
	}

	if err := x.Sync2(new(Comment)); err != nil {
		return fmt.Errorf("sync2: %v", err)
	}

	if _, err := x.Exec("UPDATE `comment` set resolve_reason = 1 WHERE `comment`.resolve_doer_id > 0"); err != nil {
		return err
	}

	if _, err := x.Exec("UPDATE `comment` set resolve_reason = 0 WHERE `comment`.resolve_doer_id == 0"); err != nil {
		return err
	}

	return nil
}
