package models

import (
	"fmt"
	"github.com/go-xorm/xorm"
)

// ChangeSeverity changes severity of issue.
func ChangeSeverity(issue *Issue, doer *User) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = changeSeverity(sess, doer, issue); err != nil {
		return err
	}

	if err = sess.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}
	return nil
}

func changeSeverity(e *xorm.Session, doer *User, issue *Issue) error {
	if err := updateIssueCols(e, issue, "severity"); err != nil {
		return err
	}

	return nil
}
