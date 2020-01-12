package migrations

import (
	"code.gitea.io/gitea/modules/log"
	"github.com/go-xorm/xorm"
)

func updateStateTypeColumnFromIssue(x *xorm.Engine) (err error) {
	if _, err := x.Exec("UPDATE issue SET state_type = 'closed' WHERE is_closed = true;"); err != nil {
		log.Warn("UPDATE state_type: %v", err)
	}
	if _, err := x.Exec("UPDATE issue SET state_type = 'open' WHERE is_closed = false;"); err != nil {
		log.Warn("UPDATE state_type: %v", err)
	}

	return nil
}
