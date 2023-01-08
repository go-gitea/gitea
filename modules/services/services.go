// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package services

import (
	"context"

	"code.gitea.io/gitea/modules/log"
)

type Config struct {
	Name         string
	Init         func(ctx context.Context) error
	Shutdown     func(ctx context.Context) error
	Dependencies []string
}

type service struct {
	initialized bool
	cfg         *Config
	dependents  []string
}

var services = make(map[string]*service)

func Register(serviceCfg *Config) {
	if serviceCfg.Name == "" {
		log.Fatal("Service configuration %#v has no name", serviceCfg)
	}
	if _, ok := services[serviceCfg.Name]; ok {
		log.Fatal("A service named %s exist", serviceCfg.Name)
	}

	services[serviceCfg.Name] = &service{
		cfg: serviceCfg,
	}
}

func initService(ctx context.Context, service *service) error {
	if service.initialized {
		return nil
	}
	for _, dep := range service.cfg.Dependencies {
		if err := initService(ctx, services[dep]); err != nil {
			return err
		}
		services[dep].dependents = append(services[dep].dependents, service.cfg.Name)
	}
	log.Trace("Initializing service: %s", service.cfg.Name)
	if err := service.cfg.Init(ctx); err != nil {
		return err
	}
	service.initialized = true
	return nil
}

func Init(ctx context.Context) error {
	for _, service := range services {
		if err := initService(ctx, service); err != nil {
			return err
		}
	}
	return nil
}

func shutdownService(ctx context.Context, service *service) error {
	if !service.initialized {
		return nil
	}
	for _, dep := range service.dependents {
		if err := shutdownService(ctx, services[dep]); err != nil {
			return err
		}
	}
	log.Trace("Shuting down service: %s", service.cfg.Name)
	if err := service.cfg.Shutdown(ctx); err != nil {
		return err
	}
	service.initialized = false
	return nil
}

func Shutdown(ctx context.Context) error {
	for _, service := range services {
		if err := shutdownService(ctx, service); err != nil {
			return err
		}
	}
	return nil
}
