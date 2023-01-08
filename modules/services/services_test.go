// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package services

import (
	"context"
	"fmt"
	"testing"
)

var serviceA = &Config{
	Name: "serviceA",
	Init: func(ctx context.Context) error {
		fmt.Println("serviceA init")
		return nil
	},
	Shutdown: func(ctx context.Context) error {
		fmt.Println("serviceA shutdown")
		return nil
	},
}

var serviceB = &Config{
	Name: "serviceB",
	Init: func(ctx context.Context) error {
		fmt.Println("serviceB init")
		return nil
	},
	Shutdown: func(ctx context.Context) error {
		fmt.Println("serviceB shutdown")
		return nil
	},
	Dependencies: []string{"serviceA"},
}

func TestServices(t *testing.T) {
	Register(serviceA)
	Register(serviceB)
	if err := Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
