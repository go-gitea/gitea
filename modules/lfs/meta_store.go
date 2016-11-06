package main

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"strings"
	"time"

	"github.com/boltdb/bolt"
)

// MetaStore implements a metadata storage. It stores user credentials and Meta information
// for objects. The storage is handled by boltdb.
type MetaStore struct {
	db *bolt.DB
}

var (
	errNoBucket       = errors.New("Bucket not found")
	errObjectNotFound = errors.New("Object not found")
)

var (
	usersBucket   = []byte("users")
	objectsBucket = []byte("objects")
)

// NewMetaStore creates a new MetaStore using the boltdb database at dbFile.
func NewMetaStore(dbFile string) (*MetaStore, error) {
	db, err := bolt.Open(dbFile, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}

	db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(usersBucket); err != nil {
			return err
		}

		if _, err := tx.CreateBucketIfNotExists(objectsBucket); err != nil {
			return err
		}

		return nil
	})

	return &MetaStore{db: db}, nil
}

// Get retrieves the Meta information for an object given information in
// RequestVars
func (s *MetaStore) Get(v *RequestVars) (*MetaObject, error) {
	if !s.authenticate(v.Authorization) {
		return nil, newAuthError()
	}
	meta, error := s.UnsafeGet(v)
	return meta, error
}

// Get retrieves the Meta information for an object given information in
// RequestVars
// DO NOT CHECK authentication, as it is supposed to have been done before
func (s *MetaStore) UnsafeGet(v *RequestVars) (*MetaObject, error) {
	var meta MetaObject

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(objectsBucket)
		if bucket == nil {
			return errNoBucket
		}

		value := bucket.Get([]byte(v.Oid))
		if len(value) == 0 {
			return errObjectNotFound
		}

		dec := gob.NewDecoder(bytes.NewBuffer(value))
		return dec.Decode(&meta)
	})

	if err != nil {
		return nil, err
	}

	return &meta, nil
}

// Put writes meta information from RequestVars to the store.
func (s *MetaStore) Put(v *RequestVars) (*MetaObject, error) {
	if !s.authenticate(v.Authorization) {
		return nil, newAuthError()
	}

	// Check if it exists first
	if meta, err := s.Get(v); err == nil {
		meta.Existing = true
		return meta, nil
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	meta := MetaObject{Oid: v.Oid, Size: v.Size}
	err := enc.Encode(meta)
	if err != nil {
		return nil, err
	}

	err = s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(objectsBucket)
		if bucket == nil {
			return errNoBucket
		}

		err = bucket.Put([]byte(v.Oid), buf.Bytes())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &meta, nil
}

// Delete removes the meta information from RequestVars to the store.
func (s *MetaStore) Delete(v *RequestVars) error {
	if !s.authenticate(v.Authorization) {
		return newAuthError()
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(objectsBucket)
		if bucket == nil {
			return errNoBucket
		}

		err := bucket.Delete([]byte(v.Oid))
		if err != nil {
			return err
		}

		return nil
	})

	return err
}

// Close closes the underlying boltdb.
func (s *MetaStore) Close() {
	s.db.Close()
}

// AddUser adds user credentials to the meta store.
func (s *MetaStore) AddUser(user, pass string) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(usersBucket)
		if bucket == nil {
			return errNoBucket
		}

		err := bucket.Put([]byte(user), []byte(pass))
		if err != nil {
			return err
		}
		return nil
	})

	return err
}

// DeleteUser removes user credentials from the meta store.
func (s *MetaStore) DeleteUser(user string) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(usersBucket)
		if bucket == nil {
			return errNoBucket
		}

		err := bucket.Delete([]byte(user))
		return err
	})

	return err
}

// MetaUser encapsulates information about a meta store user
type MetaUser struct {
	Name string
}

// Users returns all MetaUsers in the meta store
func (s *MetaStore) Users() ([]*MetaUser, error) {
	var users []*MetaUser

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(usersBucket)
		if bucket == nil {
			return errNoBucket
		}

		bucket.ForEach(func(k, v []byte) error {
			users = append(users, &MetaUser{string(k)})
			return nil
		})
		return nil
	})

	return users, err
}

// Objects returns all MetaObjects in the meta store
func (s *MetaStore) Objects() ([]*MetaObject, error) {
	var objects []*MetaObject

	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(objectsBucket)
		if bucket == nil {
			return errNoBucket
		}

		bucket.ForEach(func(k, v []byte) error {
			var meta MetaObject
			dec := gob.NewDecoder(bytes.NewBuffer(v))
			err := dec.Decode(&meta)
			if err != nil {
				return err
			}
			objects = append(objects, &meta)
			return nil
		})
		return nil
	})

	return objects, err
}

// authenticate uses the authorization string to determine whether
// or not to proceed. This server assumes an HTTP Basic auth format.
func (s *MetaStore) authenticate(authorization string) bool {
	if Config.IsPublic() {
		return true
	}

	if authorization == "" {
		return false
	}

	if !strings.HasPrefix(authorization, "Basic ") {
		return false
	}

	c, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authorization, "Basic "))
	if err != nil {
		return false
	}
	cs := string(c)
	i := strings.IndexByte(cs, ':')
	if i < 0 {
		return false
	}
	user, password := cs[:i], cs[i+1:]

	// check Basic Authentication (admin)
	ok := checkBasicAuth(user, password, true)
	if ok {
		return true
	}

	value := ""

	s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(usersBucket)
		if bucket == nil {
			return errNoBucket
		}

		value = string(bucket.Get([]byte(user)))
		return nil
	})

	if value != "" && value == password {
		return true
	}
	return false
}

type authError struct {
	error
}

func (e authError) AuthError() bool {
	return true
}

func newAuthError() error {
	return authError{errors.New("Forbidden")}
}
