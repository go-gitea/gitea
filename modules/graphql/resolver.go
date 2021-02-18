// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
//go:generate go run -mod=vendor github.com/99designs/gqlgen genrate

package graphql

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// Resolver resolver type define
type Resolver struct{}

// Query get query port
func (r *Resolver) Query() QueryResolver {
	return &Query{}
}

// Mutation get mutation port
func (r *Resolver) Mutation() MutationResolver {
	return &Mutation{}
}
