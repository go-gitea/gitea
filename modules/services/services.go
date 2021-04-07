// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package services

import "fmt"

// Service represents a service
type Service interface {
	Init() error
}

var (
	services []struct {
		Name string
		Service
	}
)

// ServiceFunc is a wrap to make a function as a Service interface
type ServiceFunc func() error

// Init run the service init function
func (h ServiceFunc) Init() error {
	return h()
}

// RegisterService register a service
func RegisterService(name string, svr ServiceFunc) {
	services = append(services, struct {
		Name string
		Service
	}{
		Name:    name,
		Service: svr,
	})
}

// Init initializes services according the sequence
func Init() error {
	for _, service := range services {
		if err := service.Init(); err != nil {
			return fmt.Errorf("Init service %s failed: %v", service.Name, err)
		}
	}
	return nil
}
