// Go CGO cross compiler
// Copyright (c) 2014 Péter Szilágyi. All rights reserved.
//
// Released under the MIT license.

// Wrapper around the GCO cross compiler docker container.
package main // import "src.techknowlogick.com/xgo"

import (
	"bytes"
	"flag"
	"fmt"
	"go/build"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// Path where to cache external dependencies
var depsCache string

func init() {
	// Initialize the external dependency cache path to a few possible locations
	if home := os.Getenv("HOME"); home != "" {
		depsCache = filepath.Join(home, ".xgo-cache")
		return
	}
	if user, err := user.Current(); user != nil && err == nil && user.HomeDir != "" {
		depsCache = filepath.Join(user.HomeDir, ".xgo-cache")
		return
	}
	depsCache = filepath.Join(os.TempDir(), "xgo-cache")
}

// Cross compilation docker containers
var dockerBase = "techknowlogick/xgo:base"
var dockerDist = "techknowlogick/xgo:"

// Command line arguments to fine tune the compilation
var (
	goVersion   = flag.String("go", "latest", "Go release to use for cross compilation")
	srcPackage  = flag.String("pkg", "", "Sub-package to build if not root import")
	srcRemote   = flag.String("remote", "", "Version control remote repository to build")
	srcBranch   = flag.String("branch", "", "Version control branch to build")
	outPrefix   = flag.String("out", "", "Prefix to use for output naming (empty = package name)")
	outFolder   = flag.String("dest", "", "Destination folder to put binaries in (empty = current)")
	crossDeps   = flag.String("deps", "", "CGO dependencies (configure/make based archives)")
	crossArgs   = flag.String("depsargs", "", "CGO dependency configure arguments")
	targets     = flag.String("targets", "*/*", "Comma separated targets to build for")
	dockerImage = flag.String("image", "", "Use custom docker image instead of official distribution")
)

// ConfigFlags is a simple set of flags to define the environment and dependencies.
type ConfigFlags struct {
	Repository   string   // Root import path to build
	Package      string   // Sub-package to build if not root import
	Prefix       string   // Prefix to use for output naming
	Remote       string   // Version control remote repository to build
	Branch       string   // Version control branch to build
	Dependencies string   // CGO dependencies (configure/make based archives)
	Arguments    string   // CGO dependency configure arguments
	Targets      []string // Targets to build for
}

// Command line arguments to pass to go build
var (
	buildVerbose = flag.Bool("v", false, "Print the names of packages as they are compiled")
	buildSteps   = flag.Bool("x", false, "Print the command as executing the builds")
	buildRace    = flag.Bool("race", false, "Enable data race detection (supported only on amd64)")
	buildTags    = flag.String("tags", "", "List of build tags to consider satisfied during the build")
	buildLdFlags = flag.String("ldflags", "", "Arguments to pass on each go tool link invocation")
	buildMode    = flag.String("buildmode", "default", "Indicates which kind of object file to build")
)

// BuildFlags is a simple collection of flags to fine tune a build.
type BuildFlags struct {
	Verbose bool   // Print the names of packages as they are compiled
	Steps   bool   // Print the command as executing the builds
	Race    bool   // Enable data race detection (supported only on amd64)
	Tags    string // List of build tags to consider satisfied during the build
	LdFlags string // Arguments to pass on each go tool link invocation
	Mode    string // Indicates which kind of object file to build
}

func main() {
	// Retrieve the CLI flags and the execution environment
	flag.Parse()

	xgoInXgo := os.Getenv("XGO_IN_XGO") == "1"
	if xgoInXgo {
		depsCache = "/deps-cache"
	}
	// Only use docker images if we're not already inside out own image
	image := ""

	if !xgoInXgo {
		// Ensure docker is available
		if err := checkDocker(); err != nil {
			log.Fatalf("Failed to check docker installation: %v.", err)
		}
		// Validate the command line arguments
		if len(flag.Args()) != 1 {
			log.Fatalf("Usage: %s [options] <go import path>", os.Args[0])
		}
		// Select the image to use, either official or custom
		image = dockerDist + *goVersion
		if *dockerImage != "" {
			image = *dockerImage
		}
		// Check that all required images are available
		found, err := checkDockerImage(image)
		switch {
		case err != nil:
			log.Fatalf("Failed to check docker image availability: %v.", err)
		case !found:
			fmt.Println("not found!")
			if err := pullDockerImage(image); err != nil {
				log.Fatalf("Failed to pull docker image from the registry: %v.", err)
			}
		default:
			fmt.Println("found.")
		}
	}
	// Cache all external dependencies to prevent always hitting the internet
	if *crossDeps != "" {
		if err := os.MkdirAll(depsCache, 0751); err != nil {
			log.Fatalf("Failed to create dependency cache: %v.", err)
		}
		// Download all missing dependencies
		for _, dep := range strings.Split(*crossDeps, " ") {
			if url := strings.TrimSpace(dep); len(url) > 0 {
				path := filepath.Join(depsCache, filepath.Base(url))

				if _, err := os.Stat(path); err != nil {
					fmt.Printf("Downloading new dependency: %s...\n", url)

					out, err := os.Create(path)
					if err != nil {
						log.Fatalf("Failed to create dependency file: %v.", err)
					}
					res, err := http.Get(url)
					if err != nil {
						log.Fatalf("Failed to retrieve dependency: %v.", err)
					}
					defer res.Body.Close()

					if _, err := io.Copy(out, res.Body); err != nil {
						log.Fatalf("Failed to download dependency: %v", err)
					}
					out.Close()

					fmt.Printf("New dependency cached: %s.\n", path)
				} else {
					fmt.Printf("Dependency already cached: %s.\n", path)
				}
			}
		}
	}
	// Assemble the cross compilation environment and build options
	config := &ConfigFlags{
		Repository:   flag.Args()[0],
		Package:      *srcPackage,
		Remote:       *srcRemote,
		Branch:       *srcBranch,
		Prefix:       *outPrefix,
		Dependencies: *crossDeps,
		Arguments:    *crossArgs,
		Targets:      strings.Split(*targets, ","),
	}
	flags := &BuildFlags{
		Verbose: *buildVerbose,
		Steps:   *buildSteps,
		Race:    *buildRace,
		Tags:    *buildTags,
		LdFlags: *buildLdFlags,
		Mode:    *buildMode,
	}
	folder, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to retrieve the working directory: %v.", err)
	}
	if *outFolder != "" {
		folder, err = filepath.Abs(*outFolder)
		if err != nil {
			log.Fatalf("Failed to resolve destination path (%s): %v.", *outFolder, err)
		}
	}
	// Execute the cross compilation, either in a container or the current system
	if !xgoInXgo {
		err = compile(image, config, flags, folder)
	} else {
		err = compileContained(config, flags, folder)
	}
	if err != nil {
		log.Fatalf("Failed to cross compile package: %v.", err)
	}
}

// Checks whether a docker installation can be found and is functional.
func checkDocker() error {
	fmt.Println("Checking docker installation...")
	if err := run(exec.Command("docker", "version")); err != nil {
		return err
	}
	fmt.Println()
	return nil
}

// Checks whether a required docker image is available locally.
func checkDockerImage(image string) (bool, error) {
	fmt.Printf("Checking for required docker image %s... ", image)
	out, err := exec.Command("docker", "images", "--no-trunc").Output()
	if err != nil {
		return false, err
	}
	return bytes.Contains(out, []byte(image)), nil
}

// Pulls an image from the docker registry.
func pullDockerImage(image string) error {
	fmt.Printf("Pulling %s from docker registry...\n", image)
	return run(exec.Command("docker", "pull", image))
}

// compile cross builds a requested package according to the given build specs
// using a specific docker cross compilation image.
func compile(image string, config *ConfigFlags, flags *BuildFlags, folder string) error {
	// If a local build was requested, find the import path and mount all GOPATH sources
	locals, mounts, paths := []string{}, []string{}, []string{}
	var usesModules bool
	if strings.HasPrefix(config.Repository, string(filepath.Separator)) || strings.HasPrefix(config.Repository, ".") {
		// Resolve the repository import path from the file path
		config.Repository = resolveImportPath(config.Repository)

		// Determine if this is a module-based repository
		var modFile = config.Repository + "/go.mod"
		_, err := os.Stat(modFile)
		usesModules = !os.IsNotExist(err)

		// Iterate over all the local libs and export the mount points
		if os.Getenv("GOPATH") == "" && !usesModules {
			log.Fatalf("No $GOPATH is set or forwarded to xgo")
		}
		if !usesModules {
			for _, gopath := range strings.Split(os.Getenv("GOPATH"), string(os.PathListSeparator)) {
				// Since docker sandboxes volumes, resolve any symlinks manually
				sources := filepath.Join(gopath, "src")
				filepath.Walk(sources, func(path string, info os.FileInfo, err error) error {
					// Skip any folders that errored out
					if err != nil {
						log.Printf("Failed to access GOPATH element %s: %v", path, err)
						return nil
					}
					// Skip anything that's not a symlink
					if info.Mode()&os.ModeSymlink == 0 {
						return nil
					}
					// Resolve the symlink and skip if it's not a folder
					target, err := filepath.EvalSymlinks(path)
					if err != nil {
						return nil
					}
					if info, err = os.Stat(target); err != nil || !info.IsDir() {
						return nil
					}
					// Skip if the symlink points within GOPATH
					if filepath.HasPrefix(target, sources) {
						return nil
					}

					// Folder needs explicit mounting due to docker symlink security
					locals = append(locals, target)
					mounts = append(mounts, filepath.Join("/ext-go", strconv.Itoa(len(locals)), "src", strings.TrimPrefix(path, sources)))
					paths = append(paths, filepath.Join("/ext-go", strconv.Itoa(len(locals))))
					return nil
				})
				// Export the main mount point for this GOPATH entry
				locals = append(locals, sources)
				mounts = append(mounts, filepath.Join("/ext-go", strconv.Itoa(len(locals)), "src"))
				paths = append(paths, filepath.Join("/ext-go", strconv.Itoa(len(locals))))
			}
		}
	}
	// Assemble and run the cross compilation command
	fmt.Printf("Cross compiling %s...\n", config.Repository)

	args := []string{
		"run", "--rm",
		"-v", folder + ":/build",
		"-v", depsCache + ":/deps-cache:ro",
		"-e", "REPO_REMOTE=" + config.Remote,
		"-e", "REPO_BRANCH=" + config.Branch,
		"-e", "PACK=" + config.Package,
		"-e", "DEPS=" + config.Dependencies,
		"-e", "ARGS=" + config.Arguments,
		"-e", "OUT=" + config.Prefix,
		"-e", fmt.Sprintf("FLAG_V=%v", flags.Verbose),
		"-e", fmt.Sprintf("FLAG_X=%v", flags.Steps),
		"-e", fmt.Sprintf("FLAG_RACE=%v", flags.Race),
		"-e", fmt.Sprintf("FLAG_TAGS=%s", flags.Tags),
		"-e", fmt.Sprintf("FLAG_LDFLAGS=%s", flags.LdFlags),
		"-e", fmt.Sprintf("FLAG_BUILDMODE=%s", flags.Mode),
		"-e", "TARGETS=" + strings.Replace(strings.Join(config.Targets, " "), "*", ".", -1),
	}
	if usesModules {
		args = append(args, []string{"-e", "GO111MODULE=on"}...)
		args = append(args, []string{"-v", os.Getenv("GOPATH") + ":/go"}...)

		// Map this repository to the /source folder
		absRepository, err := filepath.Abs(config.Repository)
		if err != nil {
			log.Fatalf("Failed to locate requested module repository: %v.", err)
		}
		args = append(args, []string{"-v", absRepository + ":/source"}...)

		fmt.Printf("Enabled Go module support\n")

		// Check whether it has a vendor folder, and if so, use it
		vendorPath := absRepository + "/vendor"
		vendorfolder, err := os.Stat(vendorPath)
		if !os.IsNotExist(err) && vendorfolder.Mode().IsDir() {
			args = append(args, []string{"-e", "FLAG_MOD=vendor"}...)
			fmt.Printf("Using vendored Go module dependencies\n")
		}
	} else {
		for i := 0; i < len(locals); i++ {
			args = append(args, []string{"-v", fmt.Sprintf("%s:%s:ro", locals[i], mounts[i])}...)
		}
		args = append(args, []string{"-e", "EXT_GOPATH=" + strings.Join(paths, ":")}...)
	}

	args = append(args, []string{image, config.Repository}...)
	return run(exec.Command("docker", args...))
}

// compileContained cross builds a requested package according to the given build
// specs using the current system opposed to running in a container. This is meant
// to be used for cross compilation already from within an xgo image, allowing the
// inheritance and bundling of the root xgo images.
func compileContained(config *ConfigFlags, flags *BuildFlags, folder string) error {
	// If a local build was requested, resolve the import path
	local := strings.HasPrefix(config.Repository, string(filepath.Separator)) || strings.HasPrefix(config.Repository, ".")
	if local {
		config.Repository = resolveImportPath(config.Repository)
	}
	// Fine tune the original environment variables with those required by the build script
	env := []string{
		"REPO_REMOTE=" + config.Remote,
		"REPO_BRANCH=" + config.Branch,
		"PACK=" + config.Package,
		"DEPS=" + config.Dependencies,
		"ARGS=" + config.Arguments,
		"OUT=" + config.Prefix,
		fmt.Sprintf("FLAG_V=%v", flags.Verbose),
		fmt.Sprintf("FLAG_X=%v", flags.Steps),
		fmt.Sprintf("FLAG_RACE=%v", flags.Race),
		fmt.Sprintf("FLAG_TAGS=%s", flags.Tags),
		fmt.Sprintf("FLAG_LDFLAGS=%s", flags.LdFlags),
		fmt.Sprintf("FLAG_BUILDMODE=%s", flags.Mode),
		"TARGETS=" + strings.Replace(strings.Join(config.Targets, " "), "*", ".", -1),
	}
	if local {
		env = append(env, "EXT_GOPATH=/non-existent-path-to-signal-local-build")
	}
	// Assemble and run the local cross compilation command
	fmt.Printf("Cross compiling %s...\n", config.Repository)

	cmd := exec.Command("/build.sh", config.Repository)
	cmd.Env = append(os.Environ(), env...)

	return run(cmd)
}

// resolveImportPath converts a package given by a relative path to a Go import
// path using the local GOPATH environment.
func resolveImportPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("Failed to locate requested package: %v.", err)
	}
	stat, err := os.Stat(abs)
	if err != nil || !stat.IsDir() {
		log.Fatalf("Requested path invalid.")
	}
	pack, err := build.ImportDir(abs, build.FindOnly)
	if err != nil {
		log.Fatalf("Failed to resolve import path: %v.", err)
	}
	return pack.ImportPath
}

// Executes a command synchronously, redirecting its output to stdout.
func run(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
