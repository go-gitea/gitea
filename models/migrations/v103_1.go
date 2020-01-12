package migrations

import (
	api "code.gitea.io/gitea/modules/structs"
	"github.com/go-xorm/xorm"
)

func addStateTypeColumnToIssue(x *xorm.Engine) (err error) {
	type Issue struct {
		StateType api.StateType `xorm:"VARCHAR(11) INDEX DEFAULT 'open'"`
	}

	return x.Sync2(new(Issue))
}
