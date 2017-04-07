// +build ignore

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

const (
	unrelDir  = changelogDir + "/unreleased"
	relDir    = changelogDir + "/v"
	relDirFmt = relDir + "%s"

	prUrlFormat  = "https://github.com/go-gitea/gitea/pull/%d"
	tagUrlFormat = "https://github.com/go-gitea/gitea/releases/tags/v%s"
)

func main() {
	var (
		changelogDir = flag.String("changelogDir", "changelog", "Path to changelogs")
		version      = flag.String("version", "", "Version for changelog")
		outputFile   = flag.String("outputFile", "CHANGELOG2.md", "File to which the changelog is written")
		force        = flag.Bool("force", false, "Force update of changelog even if version already exists. DON'T USE THIS!")
	)

	flag.Parse()

	if version == nil || *version == "" {
		fmt.Printf("Error: You did not give a version\n\nAvailable flags:\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	verDir := fmt.Sprintf(relDirFmt, *version)

	if _, err := os.Stat(verDir); err == nil {
		if !(*force) {
			log.Println("Error: Version already released! Aborting!")
			os.Exit(1)
		}
		log.Println("Warning: Version already released! Continuing anyhow!")
	}

	log.Printf("Creating new changelog directory: %q\n", verDir)
	if err := os.Mkdir(verDir, 0755); err != nil && !(*force) {
		log.Printf("Error: creating changelog directory failed: %q: %v\n", verDir, err)
		os.Exit(1)
	}

	log.Printf("Copying unreleased changelogs\n")
	if err := filepath.Walk(unrelDir, copyFileWalker(unrelDir, verDir)); err != nil {
		log.Printf("Error: copying files: %v\n", err)
		os.Exit(1)
	}

	log.Printf("Generating new changelog\n")
	cl := newChangelog()
	if err := filepath.Walk(changelogDir, cl.Populate(unrelDir)); err != nil {
		fmt.Printf("Error: generating changelog: %v\n", err)
		os.Exit(1)
	}
	if err := cl.Write(*outputFile); err != nil {
		fmt.Printf("Error: writing changelog: %v\n", err)
		os.Exit(1)
	}

	log.Printf("Removing released changelogs\n")
	if err := filepath.Walk(unrelDir, removeFileWalker(unrelDir)); err != nil {
		log.Printf("Error: removing files: %v\n", err)
		os.Exit(1)
	}
}

func removeFileWalker(dir string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove file: %q: %s", path, err)
		}
		log.Printf(" => Changelog removed: %q\n", path)
		return nil
	}
}

func copyFileWalker(oldDir, newDir string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// NOTE: Since the generator loads the entire file into memory.
		//       Don't include files that are larger than 1kB
		if info.Size() >= 1024 {
			log.Printf("Warning: file to large, skipping: %q %d\n", path, info.Size())
			return nil
		}

		newPath := strings.Replace(path, oldDir, newDir, 1)
		oldFile, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file: %q", path)
		}
		defer oldFile.Close()

		newFile, err := os.Create(newPath)
		if err != nil {
			return fmt.Errorf("failed to open file: %q", newPath)
		}
		defer newFile.Close()

		if _, err := io.Copy(newFile, oldFile); err != nil {
			return fmt.Errorf("failed to copy file: %q => %q: %s", path, newPath, err)
		}
		log.Printf(" => Changelog copied: %q\n", newPath)
		return nil
	}
}

type changelog struct {
	Version map[string]*changelogVersion
}

func (c *changelog) Populate(skipDir string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || strings.HasPrefix(path, skipDir) || path == changelogDir {
			return err
		}

		ver := strings.TrimPrefix(path, relDir)
		if len(ver) < 1 {
			return fmt.Errorf("invalid version path: %q: %q", path, ver)
		}

		log.Printf(" => seen %q in %q\n", ver, path)

		cl := &changelogVersion{}
		if err := filepath.Walk(changelogDir, cl.Populate(unrelDir)); err != nil {
			return fmt.Errorf("error populating changelog version: %q: %v\n", ver, err)
		}

		c.Version[ver] = cl

		return nil
	}
}

func newChangelog() *changelog {
	return &changelog{Version: make(map[string]*changelogVersion)}
}

func (c *changelog) Write(outFile string) error {
	f, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("writing changelog failed: %q: %v", err)
	}
	defer f.Close()

	for ver, clv := range c.Version {
		fmt.Fprintf(f, "## [%s](%s)\n", ver, fmt.Sprintf(tagUrlFormat, ver))
		clv.Write(f)
	}

	return nil
}

type changelogVersion struct {
	Breaking    []*changelogEntry
	Feature     []*changelogEntry
	Bugfix      []*changelogEntry
	Enhancement []*changelogEntry
	Misc        []*changelogEntry
}

func (c *changelogVersion) Populate(skipDir string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || strings.HasPrefix(path, skipDir) || !strings.HasSuffix(path, ".yml") {
			return err
		}
		cle, err := loadChangelogEntry(path)
		if err != nil {
			return fmt.Errorf("failed to load changelog entry: %v", err)
		}
		switch cle.Kind {
		case "breaking":
			c.Breaking = append(c.Breaking, cle)
		case "feature":
			c.Feature = append(c.Feature, cle)
		case "bugfix":
			c.Bugfix = append(c.Bugfix, cle)
		case "enhancement":
			c.Enhancement = append(c.Enhancement, cle)
		default:
			c.Misc = append(c.Misc, cle)
		}
		return nil
	}
}

func (c *changelogVersion) Write(f io.Writer) error {
	if len(c.Breaking) > 0 {
		f.Write([]byte("* BREAKING\n"))
		for _, cle := range c.Breaking {
			cle.Write(f)
		}
	}
	if len(c.Bugfix) > 0 {
		f.Write([]byte("* BUGFIXES\n"))
		for _, cle := range c.Bugfix {
			cle.Write(f)
		}
	}
	if len(c.Feature) > 0 {
		f.Write([]byte("* FEATURES\n"))
		for _, cle := range c.Feature {
			cle.Write(f)
		}
	}
	if len(c.Enhancement) > 0 {
		f.Write([]byte("* ENHANCEMENT\n"))
		for _, cle := range c.Enhancement {
			cle.Write(f)
		}
	}
	if len(c.Misc) > 0 {
		f.Write([]byte("* MISC\n"))
		for _, cle := range c.Misc {
			cle.Write(f)
		}
	}

	return nil
}

type changelogEntry struct {
	Kind        string  `yaml:"kind"`
	Title       string  `yaml:"title"`
	PullRequest int     `yaml:"pull_request"`
	Issue       *int    `yaml:"issue"`
	Author      *string `yaml:"author"`
}

func loadChangelogEntry(file string) (*changelogEntry, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("can't read changelog entry into memory: %q: %s", file, err)
	}

	newCL := &changelogEntry{}
	if err := yaml.Unmarshal(data, newCL); err != nil {
		return nil, fmt.Errorf("can't unmarshal changelog entry: %q: %s", file, err)
	}

	return newCL, nil
}

func (c *changelogEntry) Write(w io.Writer) (int, error) {
	url := fmt.Sprintf(prUrlFormat, c.PullRequest)
	str := fmt.Sprintf("  * %s [%d](%s)\n", c.Title, c.PullRequest, url)
	return w.Write([]byte(str))
}
