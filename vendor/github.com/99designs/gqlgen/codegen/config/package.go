package config

import (
	"fmt"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/99designs/gqlgen/internal/code"
)

type PackageConfig struct {
	Filename string `yaml:"filename,omitempty"`
	Package  string `yaml:"package,omitempty"`
}

func (c *PackageConfig) ImportPath() string {
	if !c.IsDefined() {
		return ""
	}
	return code.ImportPathForDir(c.Dir())
}

func (c *PackageConfig) Dir() string {
	if !c.IsDefined() {
		return ""
	}
	return filepath.Dir(c.Filename)
}

func (c *PackageConfig) Pkg() *types.Package {
	if !c.IsDefined() {
		return nil
	}
	return types.NewPackage(c.ImportPath(), c.Package)
}

func (c *PackageConfig) IsDefined() bool {
	return c.Filename != ""
}

func (c *PackageConfig) Check() error {
	if strings.ContainsAny(c.Package, "./\\") {
		return fmt.Errorf("package should be the output package name only, do not include the output filename")
	}
	if c.Filename == "" {
		return fmt.Errorf("filename must be specified")
	}
	if !strings.HasSuffix(c.Filename, ".go") {
		return fmt.Errorf("filename should be path to a go source file")
	}

	c.Filename = abs(c.Filename)

	// If Package is not set, first attempt to load the package at the output dir. If that fails
	// fallback to just the base dir name of the output filename.
	if c.Package == "" {
		c.Package = code.NameForDir(c.Dir())
	}

	return nil
}
