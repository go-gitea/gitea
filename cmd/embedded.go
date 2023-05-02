// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/assetfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"

	"github.com/gobwas/glob"
	"github.com/urfave/cli"
)

// Cmdembedded represents the available extract sub-command.
var (
	Cmdembedded = cli.Command{
		Name:        "embedded",
		Usage:       "Extract embedded resources",
		Description: "A command for extracting embedded resources, like templates and images",
		Subcommands: []cli.Command{
			subcmdList,
			subcmdView,
			subcmdExtract,
		},
	}

	subcmdList = cli.Command{
		Name:   "list",
		Usage:  "List files matching the given pattern",
		Action: runList,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "include-vendored,vendor",
				Usage: "Include files under public/vendor as well",
			},
		},
	}

	subcmdView = cli.Command{
		Name:   "view",
		Usage:  "View a file matching the given pattern",
		Action: runView,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "include-vendored,vendor",
				Usage: "Include files under public/vendor as well",
			},
		},
	}

	subcmdExtract = cli.Command{
		Name:   "extract",
		Usage:  "Extract resources",
		Action: runExtract,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "include-vendored,vendor",
				Usage: "Include files under public/vendor as well",
			},
			cli.BoolFlag{
				Name:  "overwrite",
				Usage: "Overwrite files if they already exist",
			},
			cli.BoolFlag{
				Name:  "rename",
				Usage: "Rename files as {name}.bak if they already exist (overwrites previous .bak)",
			},
			cli.BoolFlag{
				Name:  "custom",
				Usage: "Extract to the 'custom' directory as per app.ini",
			},
			cli.StringFlag{
				Name:  "destination,dest-dir",
				Usage: "Extract to the specified directory",
			},
		},
	}

	matchedAssetFiles []assetFile
)

type assetFile struct {
	fs   *assetfs.LayeredFS
	name string
	path string
}

func initEmbeddedExtractor(c *cli.Context) error {
	// FIXME: there is a bug, if the user runs `gitea embedded` with a different user or root,
	// The setting.Init (loadRunModeFrom) will fail and do log.Fatal
	// But the console logger has been deleted, so nothing is printed, the user sees nothing and Gitea just exits.

	// Silence the console logger
	log.DelNamedLogger("console")
	log.DelNamedLogger(log.DEFAULT)

	// Read configuration file
	setting.Init(&setting.Options{
		AllowEmpty: true,
	})

	patterns, err := compileCollectPatterns(c.Args())
	if err != nil {
		return err
	}

	collectAssetFilesByPattern(c, patterns, "options", options.BuiltinAssets())
	collectAssetFilesByPattern(c, patterns, "public", public.BuiltinAssets())
	collectAssetFilesByPattern(c, patterns, "templates", templates.BuiltinAssets())

	return nil
}

func runList(c *cli.Context) error {
	if err := runListDo(c); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return err
	}
	return nil
}

func runView(c *cli.Context) error {
	if err := runViewDo(c); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return err
	}
	return nil
}

func runExtract(c *cli.Context) error {
	if err := runExtractDo(c); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return err
	}
	return nil
}

func runListDo(c *cli.Context) error {
	if err := initEmbeddedExtractor(c); err != nil {
		return err
	}

	for _, a := range matchedAssetFiles {
		fmt.Println(a.path)
	}

	return nil
}

func runViewDo(c *cli.Context) error {
	if err := initEmbeddedExtractor(c); err != nil {
		return err
	}

	if len(matchedAssetFiles) == 0 {
		return fmt.Errorf("no files matched the given pattern")
	} else if len(matchedAssetFiles) > 1 {
		return fmt.Errorf("too many files matched the given pattern, try to be more specific")
	}

	data, err := matchedAssetFiles[0].fs.ReadFile(matchedAssetFiles[0].name)
	if err != nil {
		return fmt.Errorf("%s: %w", matchedAssetFiles[0].path, err)
	}

	if _, err = os.Stdout.Write(data); err != nil {
		return fmt.Errorf("%s: %w", matchedAssetFiles[0].path, err)
	}

	return nil
}

func runExtractDo(c *cli.Context) error {
	if err := initEmbeddedExtractor(c); err != nil {
		return err
	}

	if len(c.Args()) == 0 {
		return fmt.Errorf("a list of pattern of files to extract is mandatory (e.g. '**' for all)")
	}

	destdir := "."

	if c.IsSet("destination") {
		destdir = c.String("destination")
	} else if c.Bool("custom") {
		destdir = setting.CustomPath
		fmt.Println("Using app.ini at", setting.CustomConf)
	}

	fi, err := os.Stat(destdir)
	if errors.Is(err, os.ErrNotExist) {
		// In case Windows users attempt to provide a forward-slash path
		wdestdir := filepath.FromSlash(destdir)
		if wfi, werr := os.Stat(wdestdir); werr == nil {
			destdir = wdestdir
			fi = wfi
			err = nil
		}
	}
	if err != nil {
		return fmt.Errorf("%s: %s", destdir, err)
	} else if !fi.IsDir() {
		return fmt.Errorf("destination %q is not a directory", destdir)
	}

	fmt.Printf("Extracting to %s:\n", destdir)

	overwrite := c.Bool("overwrite")
	rename := c.Bool("rename")

	for _, a := range matchedAssetFiles {
		if err := extractAsset(destdir, a, overwrite, rename); err != nil {
			// Non-fatal error
			fmt.Fprintf(os.Stderr, "%s: %v", a.path, err)
		}
	}

	return nil
}

func extractAsset(d string, a assetFile, overwrite, rename bool) error {
	dest := filepath.Join(d, filepath.FromSlash(a.path))
	dir := filepath.Dir(dest)

	data, err := a.fs.ReadFile(a.name)
	if err != nil {
		return fmt.Errorf("%s: %w", a.path, err)
	}

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}

	perms := os.ModePerm & 0o666

	fi, err := os.Lstat(dest)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%s: %w", dest, err)
		}
	} else if !overwrite && !rename {
		fmt.Printf("%s already exists; skipped.\n", dest)
		return nil
	} else if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s already exists, but it's not a regular file", dest)
	} else if rename {
		if err := util.Rename(dest, dest+".bak"); err != nil {
			return fmt.Errorf("error creating backup for %s: %w", dest, err)
		}
		// Attempt to respect file permissions mask (even if user:group will be set anew)
		perms = fi.Mode()
	}

	file, err := os.OpenFile(dest, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, perms)
	if err != nil {
		return fmt.Errorf("%s: %w", dest, err)
	}
	defer file.Close()

	if _, err = file.Write(data); err != nil {
		return fmt.Errorf("%s: %w", dest, err)
	}

	fmt.Println(dest)

	return nil
}

func collectAssetFilesByPattern(c *cli.Context, globs []glob.Glob, path string, layer *assetfs.Layer) {
	fs := assetfs.Layered(layer)
	files, err := fs.ListAllFiles(".", true)
	if err != nil {
		log.Error("Error listing files in %q: %v", path, err)
		return
	}
	for _, name := range files {
		if path == "public" &&
			strings.HasPrefix(name, "vendor/") &&
			!c.Bool("include-vendored") {
			continue
		}
		matchName := path + "/" + name
		for _, g := range globs {
			if g.Match(matchName) {
				matchedAssetFiles = append(matchedAssetFiles, assetFile{fs: fs, name: name, path: path + "/" + name})
				break
			}
		}
	}
}

func compileCollectPatterns(args []string) ([]glob.Glob, error) {
	if len(args) == 0 {
		args = []string{"**"}
	}
	pat := make([]glob.Glob, len(args))
	for i := range args {
		if g, err := glob.Compile(args[i], '/'); err != nil {
			return nil, fmt.Errorf("'%s': Invalid glob pattern: %w", args[i], err)
		} else { //nolint:revive
			pat[i] = g
		}
	}
	return pat, nil
}
