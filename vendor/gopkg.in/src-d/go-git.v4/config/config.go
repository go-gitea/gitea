// Package config storage is the implementation of git config for go-git
package config

import (
	"errors"
	"fmt"
)

const (
	// DefaultRefSpec is the default refspec used, when none is given
	DefaultRefSpec = "+refs/heads/*:refs/remotes/%s/*"
)

// ConfigStorer generic storage of Config object
type ConfigStorer interface {
	Config() (*Config, error)
	SetConfig(*Config) error
}

var (
	ErrInvalid               = errors.New("config invalid remote")
	ErrRemoteConfigNotFound  = errors.New("remote config not found")
	ErrRemoteConfigEmptyURL  = errors.New("remote config: empty URL")
	ErrRemoteConfigEmptyName = errors.New("remote config: empty name")
)

// Config contains the repository configuration
type Config struct {
	Core struct {
		IsBare bool
	}
	Remotes map[string]*RemoteConfig
}

// NewConfig returns a new empty Config
func NewConfig() *Config {
	return &Config{
		Remotes: make(map[string]*RemoteConfig, 0),
	}
}

// Validate validate the fields and set the default values
func (c *Config) Validate() error {
	for name, r := range c.Remotes {
		if r.Name != name {
			return ErrInvalid
		}

		if err := r.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// RemoteConfig contains the configuration for a given repository
type RemoteConfig struct {
	Name  string
	URL   string
	Fetch []RefSpec
}

// Validate validate the fields and set the default values
func (c *RemoteConfig) Validate() error {
	if c.Name == "" {
		return ErrRemoteConfigEmptyName
	}

	if c.URL == "" {
		return ErrRemoteConfigEmptyURL
	}

	if len(c.Fetch) == 0 {
		c.Fetch = []RefSpec{RefSpec(fmt.Sprintf(DefaultRefSpec, c.Name))}
	}

	return nil
}
