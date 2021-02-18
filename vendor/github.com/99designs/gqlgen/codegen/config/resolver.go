package config

import (
	"fmt"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/99designs/gqlgen/internal/code"
)

type ResolverConfig struct {
	Filename         string         `yaml:"filename,omitempty"`
	FilenameTemplate string         `yaml:"filename_template,omitempty"`
	Package          string         `yaml:"package,omitempty"`
	Type             string         `yaml:"type,omitempty"`
	Layout           ResolverLayout `yaml:"layout,omitempty"`
	DirName          string         `yaml:"dir"`
}

type ResolverLayout string

var (
	LayoutSingleFile   ResolverLayout = "single-file"
	LayoutFollowSchema ResolverLayout = "follow-schema"
)

func (r *ResolverConfig) Check() error {
	if r.Layout == "" {
		r.Layout = LayoutSingleFile
	}
	if r.Type == "" {
		r.Type = "Resolver"
	}

	switch r.Layout {
	case LayoutSingleFile:
		if r.Filename == "" {
			return fmt.Errorf("filename must be specified with layout=%s", r.Layout)
		}
		if !strings.HasSuffix(r.Filename, ".go") {
			return fmt.Errorf("filename should be path to a go source file with layout=%s", r.Layout)
		}
		r.Filename = abs(r.Filename)
	case LayoutFollowSchema:
		if r.DirName == "" {
			return fmt.Errorf("dirname must be specified with layout=%s", r.Layout)
		}
		r.DirName = abs(r.DirName)
		if r.Filename == "" {
			r.Filename = filepath.Join(r.DirName, "resolver.go")
		} else {
			r.Filename = abs(r.Filename)
		}
	default:
		return fmt.Errorf("invalid layout %s. must be %s or %s", r.Layout, LayoutSingleFile, LayoutFollowSchema)
	}

	if strings.ContainsAny(r.Package, "./\\") {
		return fmt.Errorf("package should be the output package name only, do not include the output filename")
	}

	if r.Package == "" && r.Dir() != "" {
		r.Package = code.NameForDir(r.Dir())
	}

	return nil
}

func (r *ResolverConfig) ImportPath() string {
	if r.Dir() == "" {
		return ""
	}
	return code.ImportPathForDir(r.Dir())
}

func (r *ResolverConfig) Dir() string {
	switch r.Layout {
	case LayoutSingleFile:
		if r.Filename == "" {
			return ""
		}
		return filepath.Dir(r.Filename)
	case LayoutFollowSchema:
		return r.DirName
	default:
		panic("invalid layout " + r.Layout)
	}
}

func (r *ResolverConfig) Pkg() *types.Package {
	if r.Dir() == "" {
		return nil
	}
	return types.NewPackage(r.ImportPath(), r.Package)
}

func (r *ResolverConfig) IsDefined() bool {
	return r.Filename != "" || r.DirName != ""
}
