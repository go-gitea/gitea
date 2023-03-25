// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package initiator

import (
	"context"
	"fmt"
	"testing"
)

var serviceA = &ServiceConfig{
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

var serviceB = &ServiceConfig{
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
	RegisterService(serviceA)
	RegisterService(serviceB)
	if err := Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := ShutdownService(context.Background()); err != nil {
		t.Fatal(err)
	}
}
