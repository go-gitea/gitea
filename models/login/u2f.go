// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package login

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/tstranex/u2f"
)

// ____ ________________________________              .__          __                 __  .__
// |    |   \_____  \_   _____/\______   \ ____   ____ |__| _______/  |_____________ _/  |_|__| ____   ____
// |    |   //  ____/|    __)   |       _// __ \ / ___\|  |/  ___/\   __\_  __ \__  \\   __\  |/  _ \ /    \
// |    |  //       \|     \    |    |   \  ___// /_/  >  |\___ \  |  |  |  | \// __ \|  | |  (  <_> )   |  \
// |______/ \_______ \___  /    |____|_  /\___  >___  /|__/____  > |__|  |__|  (____  /__| |__|\____/|___|  /
// \/   \/            \/     \/_____/         \/                   \/                    \/

// ErrU2FRegistrationNotExist represents a "ErrU2FRegistrationNotExist" kind of error.
type ErrU2FRegistrationNotExist struct {
	ID int64
}

func (err ErrU2FRegistrationNotExist) Error() string {
	return fmt.Sprintf("U2F registration does not exist [id: %d]", err.ID)
}

// IsErrU2FRegistrationNotExist checks if an error is a ErrU2FRegistrationNotExist.
func IsErrU2FRegistrationNotExist(err error) bool {
	_, ok := err.(ErrU2FRegistrationNotExist)
	return ok
}

// U2FRegistration represents the registration data and counter of a security key
type U2FRegistration struct {
	ID          int64 `xorm:"pk autoincr"`
	Name        string
	UserID      int64 `xorm:"INDEX"`
	Raw         []byte
	Counter     uint32             `xorm:"BIGINT"`
	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func init() {
	db.RegisterModel(new(U2FRegistration))
}

// TableName returns a better table name for U2FRegistration
func (reg U2FRegistration) TableName() string {
	return "u2f_registration"
}

// Parse will convert the db entry U2FRegistration to an u2f.Registration struct
func (reg *U2FRegistration) Parse() (*u2f.Registration, error) {
	r := new(u2f.Registration)
	return r, r.UnmarshalBinary(reg.Raw)
}

func (reg *U2FRegistration) updateCounter(e db.Engine) error {
	_, err := e.ID(reg.ID).Cols("counter").Update(reg)
	return err
}

// UpdateCounter will update the database value of counter
func (reg *U2FRegistration) UpdateCounter() error {
	return reg.updateCounter(db.GetEngine(db.DefaultContext))
}

// U2FRegistrationList is a list of *U2FRegistration
type U2FRegistrationList []*U2FRegistration

// ToRegistrations will convert all U2FRegistrations to u2f.Registrations
func (list U2FRegistrationList) ToRegistrations() []u2f.Registration {
	regs := make([]u2f.Registration, 0, len(list))
	for _, reg := range list {
		r, err := reg.Parse()
		if err != nil {
			log.Fatal("parsing u2f registration: %v", err)
			continue
		}
		regs = append(regs, *r)
	}

	return regs
}

func getU2FRegistrationsByUID(e db.Engine, uid int64) (U2FRegistrationList, error) {
	regs := make(U2FRegistrationList, 0)
	return regs, e.Where("user_id = ?", uid).Find(&regs)
}

// GetU2FRegistrationByID returns U2F registration by id
func GetU2FRegistrationByID(id int64) (*U2FRegistration, error) {
	return getU2FRegistrationByID(db.GetEngine(db.DefaultContext), id)
}

func getU2FRegistrationByID(e db.Engine, id int64) (*U2FRegistration, error) {
	reg := new(U2FRegistration)
	if found, err := e.ID(id).Get(reg); err != nil {
		return nil, err
	} else if !found {
		return nil, ErrU2FRegistrationNotExist{ID: id}
	}
	return reg, nil
}

// GetU2FRegistrationsByUID returns all U2F registrations of the given user
func GetU2FRegistrationsByUID(uid int64) (U2FRegistrationList, error) {
	return getU2FRegistrationsByUID(db.GetEngine(db.DefaultContext), uid)
}

func createRegistration(e db.Engine, userID int64, name string, reg *u2f.Registration) (*U2FRegistration, error) {
	raw, err := reg.MarshalBinary()
	if err != nil {
		return nil, err
	}
	r := &U2FRegistration{
		UserID:  userID,
		Name:    name,
		Counter: 0,
		Raw:     raw,
	}
	_, err = e.InsertOne(r)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// CreateRegistration will create a new U2FRegistration from the given Registration
func CreateRegistration(userID int64, name string, reg *u2f.Registration) (*U2FRegistration, error) {
	return createRegistration(db.GetEngine(db.DefaultContext), userID, name, reg)
}

// DeleteRegistration will delete U2FRegistration
func DeleteRegistration(reg *U2FRegistration) error {
	return deleteRegistration(db.GetEngine(db.DefaultContext), reg)
}

func deleteRegistration(e db.Engine, reg *U2FRegistration) error {
	_, err := e.Delete(reg)
	return err
}
