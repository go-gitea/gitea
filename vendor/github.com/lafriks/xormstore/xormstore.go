/*
Package xormstore is a XORM backend for gorilla sessions

Simplest form:

	store, err := xormstore.New(engine, []byte("secret-hash-key"))

All options:

	store, err := xormstore.NewOptions(
		engine, // *xorm.Engine
		xormstore.Options{
			TableName: "sessions",  // "sessions" is default
			SkipCreateTable: false, // false is default
		},
		[]byte("secret-hash-key"),      // 32 or 64 bytes recommended, required
		[]byte("secret-encyption-key")) // nil, 16, 24 or 32 bytes, optional

	if err != nil {
		// xormstore can not be initialized
	}

	// some more settings, see sessions.Options
	store.SessionOpts.Secure = true
	store.SessionOpts.HttpOnly = true
	store.SessionOpts.MaxAge = 60 * 60 * 24 * 60

If you want periodic cleanup of expired sessions:

	quit := make(chan struct{})
	go store.PeriodicCleanup(1*time.Hour, quit)

For more information about the keys see https://github.com/gorilla/securecookie

For API to use in HTTP handlers see https://github.com/gorilla/sessions
*/
package xormstore

import (
	"encoding/base32"
	"net/http"
	"strings"
	"time"

	"github.com/lafriks/xormstore/util"

	"xorm.io/xorm"
	"github.com/gorilla/context"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

const sessionIDLen = 32
const defaultTableName = "sessions"
const defaultMaxAge = 60 * 60 * 24 * 30 // 30 days
const defaultPath = "/"

// Options for xormstore
type Options struct {
	TableName       string
	SkipCreateTable bool
}

// Store represent a xormstore
type Store struct {
	e           *xorm.Engine
	opts        Options
	Codecs      []securecookie.Codec
	SessionOpts *sessions.Options
}

type xormSession struct {
	ID          string         `xorm:"VARCHAR(100) PK NAME 'id'"`
	Data        string         `xorm:"TEXT"`
	CreatedUnix util.TimeStamp `xorm:"created"`
	UpdatedUnix util.TimeStamp `xorm:"updated"`
	ExpiresUnix util.TimeStamp `xorm:"INDEX"`

	tableName string `xorm:"-"` // just to store table name for easier access
}

// Define a type for context keys so that they can't clash with anything else stored in context
type contextKey string

func (xs *xormSession) TableName() string {
	return xs.tableName
}

// New creates a new xormstore session
func New(e *xorm.Engine, keyPairs ...[]byte) (*Store, error) {
	return NewOptions(e, Options{}, keyPairs...)
}

// NewOptions creates a new xormstore session with options
func NewOptions(e *xorm.Engine, opts Options, keyPairs ...[]byte) (*Store, error) {
	st := &Store{
		e:      e,
		opts:   opts,
		Codecs: securecookie.CodecsFromPairs(keyPairs...),
		SessionOpts: &sessions.Options{
			Path:   defaultPath,
			MaxAge: defaultMaxAge,
		},
	}
	if st.opts.TableName == "" {
		st.opts.TableName = defaultTableName
	}

	if !st.opts.SkipCreateTable {
		if err := st.e.Sync2(&xormSession{tableName: st.opts.TableName}); err != nil {
			return nil, err
		}
	}

	return st, nil
}

// Get returns a session for the given name after adding it to the registry.
func (st *Store) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(st, name)
}

// New creates a session with name without adding it to the registry.
func (st *Store) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(st, name)
	opts := *st.SessionOpts
	session.Options = &opts

	st.MaxAge(st.SessionOpts.MaxAge)

	// try fetch from db if there is a cookie
	if cookie, err := r.Cookie(name); err == nil {
		if err := securecookie.DecodeMulti(name, cookie.Value, &session.ID, st.Codecs...); err != nil {
			return session, nil
		}
		s := &xormSession{tableName: st.opts.TableName}
		if has, err := st.e.Where("id = ? AND expires_unix >= ?", session.ID, util.TimeStampNow()).Get(s); !has || err != nil {
			return session, nil
		}
		if err := securecookie.DecodeMulti(session.Name(), s.Data, &session.Values, st.Codecs...); err != nil {
			return session, nil
		}

		context.Set(r, contextKey(name), s)
	}

	return session, nil
}

// Save session and set cookie header
func (st *Store) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	s, _ := context.Get(r, contextKey(session.Name())).(*xormSession)

	// delete if max age is < 0
	if session.Options.MaxAge < 0 {
		if s != nil {
			if _, err := st.e.Delete(&xormSession{
				ID:        session.ID,
				tableName: st.opts.TableName,
			}); err != nil {
				return err
			}
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
		return nil
	}

	data, err := securecookie.EncodeMulti(session.Name(), session.Values, st.Codecs...)
	if err != nil {
		return err
	}
	now := util.TimeStampNow()
	expire := now.AddDuration(time.Second * time.Duration(session.Options.MaxAge))

	if s == nil {
		// generate random session ID key suitable for storage in the db
		session.ID = strings.TrimRight(
			base32.StdEncoding.EncodeToString(
				securecookie.GenerateRandomKey(sessionIDLen)), "=")
		s = &xormSession{
			ID:          session.ID,
			Data:        data,
			CreatedUnix: now,
			UpdatedUnix: now,
			ExpiresUnix: expire,
			tableName:   st.opts.TableName,
		}
		if _, err := st.e.Insert(s); err != nil {
			return err
		}
		context.Set(r, contextKey(session.Name()), s)
	} else {
		s.Data = data
		s.UpdatedUnix = now
		s.ExpiresUnix = expire
		if _, err := st.e.ID(s.ID).Cols("data", "updated_unix", "expires_unix").Update(s); err != nil {
			return err
		}
	}

	// set session id cookie
	id, err := securecookie.EncodeMulti(session.Name(), session.ID, st.Codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), id, session.Options))

	return nil
}

// MaxAge sets the maximum age for the store and the underlying cookie
// implementation. Individual sessions can be deleted by setting
// Options.MaxAge = -1 for that session.
func (st *Store) MaxAge(age int) {
	st.SessionOpts.MaxAge = age
	for _, codec := range st.Codecs {
		if sc, ok := codec.(*securecookie.SecureCookie); ok {
			sc.MaxAge(age)
		}
	}
}

// MaxLength restricts the maximum length of new sessions to l.
// If l is 0 there is no limit to the size of a session, use with caution.
// The default is 4096 (default for securecookie)
func (st *Store) MaxLength(l int) {
	for _, c := range st.Codecs {
		if codec, ok := c.(*securecookie.SecureCookie); ok {
			codec.MaxLength(l)
		}
	}
}

// Cleanup deletes expired sessions
func (st *Store) Cleanup() {
	st.e.Where("expires_unix < ?", util.TimeStampNow()).Delete(&xormSession{tableName: st.opts.TableName})
}

// PeriodicCleanup runs Cleanup every interval. Close quit channel to stop.
func (st *Store) PeriodicCleanup(interval time.Duration, quit <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			st.Cleanup()
		case <-quit:
			return
		}
	}
}
