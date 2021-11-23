package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
)

// OAuth Login Source
type OAuth struct {
	ID   int64 `xorm:"pk autoincr"`
	Name string
}

func init() {
	db.RegisterModel(new(OAuth))
}

func getOAuthByID(e db.Engine, id int64) (*OAuth, error) {
	o := new(OAuth)
	has, err := e.ID(id).Get(o)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrUserNotExist{id, "", 0}
	}
	return o, nil
}

// GetOAuthByID returns the oauth object by given ID if exists.
func GetOAuthByID(id int64) (*OAuth, error) {
	return getOAuthByID(db.GetEngine(db.DefaultContext), id)
}

// GetOAuthByName returns oauth by given name.
func GetOAuthByName(name string) (*OAuth, error) {
	return getOAuthByName(db.GetEngine(db.DefaultContext), name)
}

func getOAuthByName(e db.Engine, name string) (*OAuth, error) {
	if len(name) == 0 {
		return nil, ErrUserNotExist{0, name, 0}
	}
	o := &OAuth{Name: strings.ToLower(name)}
	has, err := e.Get(o)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrUserNotExist{0, name, 0}
	}
	return o, nil
}

// CreateOAuth creates record of a new oauth.
func CreateOAuth(o *OAuth) (err error) {
	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Insert(o); err != nil {
		return err
	}

	return sess.Commit()
}

func validateOAuth(o *OAuth) error {

	return nil
}

func updateOAuth(e db.Engine, o *OAuth) error {
	if err := validateOAuth(o); err != nil {
		return err
	}

	_, err := e.ID(o.ID).AllCols().Update(o)
	return err
}

// UpdateOAuth updates oauth's information.
func UpdateOAuth(o *OAuth) error {
	return updateOAuth(db.GetEngine(db.DefaultContext), o)
}

// UpdateOAuthCols update user according special columns
func UpdateOAuthCols(o *OAuth, cols ...string) error {
	return updateOAuthCols(db.GetEngine(db.DefaultContext), o, cols...)
}

func updateOAuthCols(e db.Engine, o *OAuth, cols ...string) error {
	if err := validateOAuth(o); err != nil {
		return err
	}

	_, err := e.ID(o.ID).Cols(cols...).Update(o)
	return err
}

func deleteOAuth(e db.Engine, o *OAuth) error {
	if _, err := e.ID(o.ID).Delete(new(OAuth)); err != nil {
		return fmt.Errorf("Delete: %v", err)
	}

	return nil
}

// DeleteOAuth deletes the record of oauth
func DeleteOAuth(o *OAuth) (err error) {
	sess := db.NewSession(db.DefaultContext)
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = deleteOAuth(sess, o); err != nil {
		// Note: don't wrapper error here.
		return err
	}

	return sess.Commit()
}
