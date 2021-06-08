// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addHideFromExplorePageUserColumn(x *xorm.Engine) error {
	type User struct {
		HideFromExplorePage bool `xorm:"NOT NULL DEFAULT false"`
	}
	if err := x.Sync2(new(User)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
