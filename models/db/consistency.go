// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import "xorm.io/builder"

// CountOrphanedObjects count subjects with have no existing refobject anymore
func CountOrphanedObjects(subject, refobject, joinCond string) (int64, error) {
	return GetEngine(DefaultContext).Table("`"+subject+"`").
		Join("LEFT", "`"+refobject+"`", joinCond).
		Where(builder.IsNull{"`" + refobject + "`.id"}).
		Select("COUNT(`" + subject + "`.`id`)").
		Count()
}

// DeleteOrphanedObjects delete subjects with have no existing refobject anymore
func DeleteOrphanedObjects(subject, refobject, joinCond string) error {
	subQuery := builder.Select("`"+subject+"`.id").
		From("`"+subject+"`").
		Join("LEFT", "`"+refobject+"`", joinCond).
		Where(builder.IsNull{"`" + refobject + "`.id"})
	b := builder.Delete(builder.In("id", subQuery)).From("`" + subject + "`")
	_, err := GetEngine(DefaultContext).Exec(b)
	return err
}
