// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package driver

import (
	"fmt"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/util"
	user_service "code.gitea.io/gitea/services/user"

	"lab.forgefriends.org/friendlyforgeformat/gof3/format"
	f3_util "lab.forgefriends.org/friendlyforgeformat/gof3/util"
)

type User struct {
	user_model.User
}

func UserConverter(f *user_model.User) *User {
	return &User{
		User: *f,
	}
}

func (o User) GetID() int64 {
	return o.ID
}

func (o *User) SetID(id int64) {
	o.ID = id
}

func (o *User) IsNil() bool {
	return o.ID == 0
}

func (o *User) Equals(other *User) bool {
	return (o.Name == other.Name)
}

func (o *User) ToFormat() *format.User {
	return &format.User{
		Common:   format.Common{Index: o.ID},
		UserName: o.Name,
		Name:     o.FullName,
		Email:    o.Email,
		Password: o.Passwd,
	}
}

func (o *User) FromFormat(user *format.User) {
	*o = User{
		User: user_model.User{
			ID:       user.Index,
			Name:     user.UserName,
			FullName: user.Name,
			Email:    user.Email,
			Passwd:   user.Password,
		},
	}
}

type UserProvider struct {
	g *Gitea
}

func (o *UserProvider) ToFormat(user *User) *format.User {
	return user.ToFormat()
}

func (o *UserProvider) FromFormat(p *format.User) *User {
	var user User
	user.FromFormat(p)
	return &user
}

func (o *UserProvider) GetObjects(page int) []*User {
	users, _, err := user_model.SearchUsers(&user_model.SearchUserOptions{
		Actor:       o.g.GetDoer(),
		Type:        user_model.UserTypeIndividual,
		ListOptions: db.ListOptions{Page: page, PageSize: o.g.perPage},
	})
	if err != nil {
		panic(fmt.Errorf("error while listing users: %v", err))
	}
	return f3_util.ConvertMap[*user_model.User, *User](users, UserConverter)
}

func (o *UserProvider) ProcessObject(user *User) {
}

func (o *UserProvider) Get(exemplar *User) *User {
	var user *user_model.User
	var err error
	if exemplar.GetID() > 0 {
		user, err = user_model.GetUserByIDCtx(o.g.ctx, exemplar.GetID())
	} else if exemplar.Name != "" {
		user, err = user_model.GetUserByName(o.g.ctx, exemplar.Name)
	} else {
		panic("GetID() == 0 and UserName == \"\"")
	}
	if user_model.IsErrUserNotExist(err) {
		return &User{}
	}
	if err != nil {
		panic(fmt.Errorf("user %v %w", exemplar, err))
	}
	return UserConverter(user)
}

func (o *UserProvider) Put(user *User) *User {
	overwriteDefault := &user_model.CreateUserOverwriteOptions{
		IsActive: util.OptionalBoolTrue,
	}
	u := user.User
	err := user_model.CreateUser(&u, overwriteDefault)
	if err != nil {
		panic(err)
	}
	return o.Get(UserConverter(&u))
}

func (o *UserProvider) Delete(user *User) *User {
	u := o.Get(user)
	if !u.IsNil() {
		if err := user_service.DeleteUser(o.g.ctx, &user.User, true); err != nil {
			panic(err)
		}
	}
	return u
}
