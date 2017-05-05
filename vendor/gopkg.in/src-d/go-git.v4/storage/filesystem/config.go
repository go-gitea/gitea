package filesystem

import (
	"fmt"
	"os"

	"gopkg.in/src-d/go-git.v4/config"
	gitconfig "gopkg.in/src-d/go-git.v4/plumbing/format/config"
	"gopkg.in/src-d/go-git.v4/storage/filesystem/internal/dotgit"
)

const (
	remoteSection = "remote"
	coreSection   = "core"
	fetchKey      = "fetch"
	urlKey        = "url"
	bareKey       = "bare"
)

type ConfigStorage struct {
	dir *dotgit.DotGit
}

func (c *ConfigStorage) Config() (*config.Config, error) {
	cfg := config.NewConfig()

	ini, err := c.unmarshal()
	if err != nil {
		return nil, err
	}

	c.unmarshalCore(cfg, ini)
	c.unmarshalRemotes(cfg, ini)

	return cfg, nil
}

func (c *ConfigStorage) unmarshal() (*gitconfig.Config, error) {
	cfg := gitconfig.New()

	f, err := c.dir.Config()
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}

		return nil, err
	}

	defer f.Close()

	d := gitconfig.NewDecoder(f)
	if err := d.Decode(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *ConfigStorage) unmarshalCore(cfg *config.Config, ini *gitconfig.Config) {
	s := ini.Section(coreSection)
	if s.Options.Get(bareKey) == "true" {
		cfg.Core.IsBare = true
	}
}

func (c *ConfigStorage) unmarshalRemotes(cfg *config.Config, ini *gitconfig.Config) {
	s := ini.Section(remoteSection)
	for _, sub := range s.Subsections {
		r := c.unmarshalRemote(sub)
		cfg.Remotes[r.Name] = r
	}
}

func (c *ConfigStorage) unmarshalRemote(s *gitconfig.Subsection) *config.RemoteConfig {
	fetch := []config.RefSpec{}
	for _, f := range s.Options.GetAll(fetchKey) {
		rs := config.RefSpec(f)
		if rs.IsValid() {
			fetch = append(fetch, rs)
		}
	}

	return &config.RemoteConfig{
		Name:  s.Name,
		URL:   s.Option(urlKey),
		Fetch: fetch,
	}
}

func (c *ConfigStorage) SetConfig(cfg *config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	ini, err := c.unmarshal()
	if err != nil {
		return err
	}

	c.marshalCore(cfg, ini)
	c.marshalRemotes(cfg, ini)
	return c.marshal(ini)
}

func (c *ConfigStorage) marshalCore(cfg *config.Config, ini *gitconfig.Config) {
	s := ini.Section(coreSection)
	s.AddOption(bareKey, fmt.Sprintf("%t", cfg.Core.IsBare))
}

func (c *ConfigStorage) marshalRemotes(cfg *config.Config, ini *gitconfig.Config) {
	s := ini.Section(remoteSection)
	s.Subsections = make(gitconfig.Subsections, len(cfg.Remotes))

	var i int
	for _, r := range cfg.Remotes {
		s.Subsections[i] = c.marshalRemote(r)
		i++
	}
}

func (c *ConfigStorage) marshalRemote(r *config.RemoteConfig) *gitconfig.Subsection {
	s := &gitconfig.Subsection{Name: r.Name}
	s.AddOption(urlKey, r.URL)
	for _, rs := range r.Fetch {
		s.AddOption(fetchKey, rs.String())
	}

	return s
}

func (c *ConfigStorage) marshal(ini *gitconfig.Config) error {
	f, err := c.dir.ConfigWriter()
	if err != nil {
		return err
	}

	defer f.Close()

	e := gitconfig.NewEncoder(f)
	return e.Encode(ini)
}
