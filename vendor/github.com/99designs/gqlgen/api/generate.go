package api

import (
	"syscall"

	"github.com/99designs/gqlgen/codegen"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin"
	"github.com/99designs/gqlgen/plugin/federation"
	"github.com/99designs/gqlgen/plugin/modelgen"
	"github.com/99designs/gqlgen/plugin/resolvergen"
	"github.com/pkg/errors"
)

func Generate(cfg *config.Config, option ...Option) error {
	_ = syscall.Unlink(cfg.Exec.Filename)
	if cfg.Model.IsDefined() {
		_ = syscall.Unlink(cfg.Model.Filename)
	}

	plugins := []plugin.Plugin{}
	if cfg.Model.IsDefined() {
		plugins = append(plugins, modelgen.New())
	}
	plugins = append(plugins, resolvergen.New())
	if cfg.Federation.IsDefined() {
		plugins = append([]plugin.Plugin{federation.New()}, plugins...)
	}

	for _, o := range option {
		o(cfg, &plugins)
	}

	for _, p := range plugins {
		if inj, ok := p.(plugin.EarlySourceInjector); ok {
			if s := inj.InjectSourceEarly(); s != nil {
				cfg.Sources = append(cfg.Sources, s)
			}
		}
	}

	if err := cfg.LoadSchema(); err != nil {
		return errors.Wrap(err, "failed to load schema")
	}

	for _, p := range plugins {
		if inj, ok := p.(plugin.LateSourceInjector); ok {
			if s := inj.InjectSourceLate(cfg.Schema); s != nil {
				cfg.Sources = append(cfg.Sources, s)
			}
		}
	}

	// LoadSchema again now we have everything
	if err := cfg.LoadSchema(); err != nil {
		return errors.Wrap(err, "failed to load schema")
	}

	if err := cfg.Init(); err != nil {
		return errors.Wrap(err, "generating core failed")
	}

	for _, p := range plugins {
		if mut, ok := p.(plugin.ConfigMutator); ok {
			err := mut.MutateConfig(cfg)
			if err != nil {
				return errors.Wrap(err, p.Name())
			}
		}
	}
	// Merge again now that the generated models have been injected into the typemap
	data, err := codegen.BuildData(cfg)
	if err != nil {
		return errors.Wrap(err, "merging type systems failed")
	}

	if err = codegen.GenerateCode(data); err != nil {
		return errors.Wrap(err, "generating core failed")
	}

	for _, p := range plugins {
		if mut, ok := p.(plugin.CodeGenerator); ok {
			err := mut.GenerateCode(data)
			if err != nil {
				return errors.Wrap(err, p.Name())
			}
		}
	}

	if err = codegen.GenerateCode(data); err != nil {
		return errors.Wrap(err, "generating core failed")
	}

	if !cfg.SkipValidation {
		if err := validate(cfg); err != nil {
			return errors.Wrap(err, "validation failed")
		}
	}

	return nil
}

func validate(cfg *config.Config) error {
	roots := []string{cfg.Exec.ImportPath()}
	if cfg.Model.IsDefined() {
		roots = append(roots, cfg.Model.ImportPath())
	}

	if cfg.Resolver.IsDefined() {
		roots = append(roots, cfg.Resolver.ImportPath())
	}

	cfg.Packages.LoadAll(roots...)
	errs := cfg.Packages.Errors()
	if len(errs) > 0 {
		return errs
	}
	return nil
}
