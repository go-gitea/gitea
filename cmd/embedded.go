// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build bindata

package cmd

import (
	"fmt"
//	"os"
//	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"

	"github.com/gobwas/glob"
	"github.com/urfave/cli"
)

// Cmdembedded represents the available extract sub-command.
var (
	Cmdembedded = cli.Command{
		Name:        "embedded",
		Usage:       "Extract embedded resources",
		Description: "A command for extracting embedded resources, like templates and images.",
		Subcommands: []cli.Command{
			subcmdList,
			subcmdExtract,
		},
		Flags: []cli.Flag{
			/*
			cli.StringFlag{
				Name:  "name, n",
				Value: "**",
				Usage: "glob pattern used to match files",
			},
			*/
			cli.BoolFlag{
				Name:  "include-vendored,vendor",
				Usage: "Include files under public/vendor as well",
			},
		},
	}

	subcmdList = cli.Command{
		Name:   "list",
		Usage:  "List files matching the given pattern",
		Action: runList,
	}

	subcmdExtract = cli.Command{
		Name:   "extract",
		Usage:  "Extract resources",
		Action: runExtract,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "overwrite",
				Usage: "Overwrite files if they already exist",
			},
			cli.BoolFlag{
				Name:  "custom",
				Usage: "Extract to the 'custom' directory",
			},
			cli.StringFlag{
				Name:  "destination",
				Usage: "Destination for the extracted files",
			},
		},
	}

	sections	map[string]*section
	assets		[]asset
)

type section struct {
	Path		string
	Names		func() []string
	IsDir		func(string) (bool, error)
}

type asset struct {
	Section		*section
	Name		string
}

func initEmbeddedExtractor(c *cli.Context) error {

	// Silence the console logger
	log.DelNamedLogger("console")

	// Read configuration file
	setting.NewContext()

	pats, err := getPatterns(c.Args())
	if err != nil {
		return err
	}
	sections := make(map[string]*section,3)

	sections["public"] = &section{ Path: "public", Names: public.AssetNames, IsDir: public.AssetIsDir }
	sections["options"] = &section{ Path: "options", Names: options.AssetNames, IsDir: options.AssetIsDir }
	sections["templates"] = &section{ Path: "templates", Names: templates.AssetNames, IsDir: templates.AssetIsDir }

	for _, sec := range sections {
		assets = append(assets, buildAssetList(sec, pats, c)...)
	}

	return nil
}

func runList(ctx *cli.Context) error {
	if err := initEmbeddedExtractor(ctx); err != nil {
		return err
	}
	// fmt.Println("Using app.ini at", setting.CustomConf)

	for _, asset := range assets {
		fmt.Printf("- [%s] [%s]\n", asset.Section.Path, asset.Name)
	}
	fmt.Println("End of list.")
	return nil
}

func runExtract(ctx *cli.Context) error {
	fmt.Println("Not implemented")
	return nil
}

func buildAssetList(sec *section, globs []glob.Glob, c *cli.Context) []asset {
	var results = make([]asset, 0, 64)
	for _, name := range sec.Names() {
		if isdir, err := sec.IsDir(name); !isdir && err == nil {
			if sec.Path == "public" &&
				strings.HasPrefix(name, "vendor/") &&
				!c.Bool("include-vendored") {
				continue
			}
			matchName := "/"+sec.Path+"/"+name
			for _, g := range globs {
				if g.Match(matchName) {
					results = append(results, asset{Section: sec, Name: name})
					break
				}
			}
		}
	}
	return results
}

func getPatterns(args []string) ([]glob.Glob, error) {
	if len(args) == 0 {
		args = []string{"**"}
	}
	pat := make([]glob.Glob,len(args))
	for i := range args {
		if g, err := glob.Compile(args[i], '.', '/'); err != nil {
			return nil, fmt.Errorf("'%s': Invalid glob pattern: %v", args[i], err)
		} else {
			pat[i] = g
		}
	}
	return pat, nil
}