// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//nolint:forbidigo
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/google/go-github/v53/github"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v3"
)

const defaultVersion = "v1.18" // to backport to

func main() {
	app := cli.NewApp()
	app.Name = "backport"
	app.Usage = "Backport provided PR-number on to the current or previous released version"
	app.Description = `Backport will look-up the PR in Gitea's git log and attempt to cherry-pick it on the current version`
	app.ArgsUsage = "<PR-to-backport>"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "version",
			Usage: "Version branch to backport on to",
		},
		cli.StringFlag{
			Name:  "upstream",
			Value: "origin",
			Usage: "Upstream remote for the Gitea upstream",
		},
		cli.StringFlag{
			Name:  "release-branch",
			Value: "",
			Usage: "Release branch to backport on. Will default to release/<version>",
		},
		cli.StringFlag{
			Name:  "cherry-pick",
			Usage: "SHA to cherry-pick as backport",
		},
		cli.StringFlag{
			Name:  "backport-branch",
			Usage: "Backport branch to backport on to (default: backport-<pr>-<version>",
		},
		cli.StringFlag{
			Name:  "remote",
			Value: "",
			Usage: "Remote for your fork of the Gitea upstream",
		},
		cli.StringFlag{
			Name:  "fork-user",
			Value: "",
			Usage: "Forked user name on Github",
		},
		cli.BoolFlag{
			Name:  "no-fetch",
			Usage: "Set this flag to prevent fetch of remote branches",
		},
		cli.BoolFlag{
			Name:  "no-amend-message",
			Usage: "Set this flag to prevent automatic amendment of the commit message",
		},
		cli.BoolFlag{
			Name:  "no-push",
			Usage: "Set this flag to prevent pushing the backport up to your fork",
		},
		cli.BoolFlag{
			Name:  "no-xdg-open",
			Usage: "Set this flag to not use xdg-open to open the PR URL",
		},
		cli.BoolFlag{
			Name:  "continue",
			Usage: "Set this flag to continue from a git cherry-pick that has broken",
		},
	}
	cli.AppHelpTemplate = `NAME:
	{{.Name}} - {{.Usage}}
USAGE:
	{{.HelpName}} {{if .VisibleFlags}}[options]{{end}} {{if .ArgsUsage}}{{.ArgsUsage}}{{else}}[arguments...]{{end}}
	{{if len .Authors}}
AUTHOR:
	{{range .Authors}}{{ . }}{{end}}
	{{end}}{{if .Commands}}
OPTIONS:
	{{range .VisibleFlags}}{{.}}
	{{end}}{{end}}
`

	app.Action = runBackport

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to backport: %v\n", err)
	}
}

func runBackport(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	continuing := c.Bool("continue")

	var pr string

	version := c.String("version")
	if version == "" && continuing {
		// determine version from current branch name
		var err error
		pr, version, err = readCurrentBranch(ctx)
		if err != nil {
			return err
		}
	}
	if version == "" {
		version = readVersion()
	}
	if version == "" {
		version = defaultVersion
	}

	upstream := c.String("upstream")
	if upstream == "" {
		upstream = "origin"
	}

	forkUser := c.String("fork-user")
	remote := c.String("remote")
	if remote == "" && !c.Bool("--no-push") {
		var err error
		remote, forkUser, err = determineRemote(ctx, forkUser)
		if err != nil {
			return err
		}
	}

	upstreamReleaseBranch := c.String("release-branch")
	if upstreamReleaseBranch == "" {
		upstreamReleaseBranch = path.Join("release", version)
	}

	localReleaseBranch := path.Join(upstream, upstreamReleaseBranch)

	args := c.Args()
	if len(args) == 0 && pr == "" {
		return fmt.Errorf("no PR number provided\nProvide a PR number to backport")
	} else if len(args) != 1 && pr == "" {
		return fmt.Errorf("multiple PRs provided %v\nOnly a single PR can be backported at a time", args)
	}
	if pr == "" {
		pr = args[0]
	}

	backportBranch := c.String("backport-branch")
	if backportBranch == "" {
		backportBranch = "backport-" + pr + "-" + version
	}

	fmt.Printf("* Backporting %s to %s as %s\n", pr, localReleaseBranch, backportBranch)

	sha := c.String("cherry-pick")
	if sha == "" {
		var err error
		sha, err = determineSHAforPR(ctx, pr)
		if err != nil {
			return err
		}
	}
	if sha == "" {
		return fmt.Errorf("unable to determine sha for cherry-pick of %s", pr)
	}

	if !c.Bool("no-fetch") {
		if err := fetchRemoteAndMain(ctx, upstream, upstreamReleaseBranch); err != nil {
			return err
		}
	}

	if !continuing {
		if err := checkoutBackportBranch(ctx, backportBranch, localReleaseBranch); err != nil {
			return err
		}
	}

	if err := cherrypick(ctx, sha); err != nil {
		return err
	}

	if !c.Bool("no-amend-message") {
		if err := amendCommit(ctx, pr); err != nil {
			return err
		}
	}

	if !c.Bool("no-push") {
		url := "https://github.com/go-gitea/gitea/compare/" + upstreamReleaseBranch + "..." + forkUser + ":" + backportBranch

		if err := gitPushUp(ctx, remote, backportBranch); err != nil {
			return err
		}

		if !c.Bool("no-xdg-open") {
			if err := xdgOpen(ctx, url); err != nil {
				return err
			}
		} else {
			fmt.Printf("* Navigate to %s to open PR\n", url)
		}
	}
	return nil
}

func xdgOpen(ctx context.Context, url string) error {
	fmt.Printf("* `xdg-open %s`\n", url)
	out, err := exec.CommandContext(ctx, "xdg-open", url).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", string(out))
		return fmt.Errorf("unable to xdg-open to %s: %w", url, err)
	}
	return nil
}

func gitPushUp(ctx context.Context, remote, backportBranch string) error {
	fmt.Printf("* `git push -u %s %s`\n", remote, backportBranch)
	out, err := exec.CommandContext(ctx, "git", "push", "-u", remote, backportBranch).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", string(out))
		return fmt.Errorf("unable to push up to %s: %w", remote, err)
	}
	return nil
}

func amendCommit(ctx context.Context, pr string) error {
	fmt.Printf("* Amending commit to prepend `Backport #%s` to body\n", pr)
	out, err := exec.CommandContext(ctx, "git", "log", "-1", "--pretty=format:%B").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", string(out))
		return fmt.Errorf("unable to get last log message: %w", err)
	}

	parts := strings.SplitN(string(out), "\n", 2)

	if len(parts) != 2 {
		return fmt.Errorf("unable to interpret log message:\n%s", string(out))
	}
	subject, body := parts[0], parts[1]
	if !strings.HasSuffix(subject, " (#"+pr+")") {
		subject = subject + " (#" + pr + ")"
	}

	out, err = exec.CommandContext(ctx, "git", "commit", "--amend", "-m", subject+"\n\nBackport #"+pr+"\n"+body).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", string(out))
		return fmt.Errorf("unable to amend last log message: %w", err)
	}
	return nil
}

func cherrypick(ctx context.Context, sha string) error {
	// Check if a CHERRY_PICK_HEAD exists
	if _, err := os.Stat(".git/CHERRY_PICK_HEAD"); err == nil {
		// Assume that we are in the middle of cherry-pick - continue it
		fmt.Println("* Attempting git cherry-pick --continue")
		out, err := exec.CommandContext(ctx, "git", "cherry-pick", "--continue").Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "git cherry-pick --continue failed:\n%s\n", string(out))
			return fmt.Errorf("unable to continue cherry-pick: %w", err)
		}
		return nil
	}

	fmt.Printf("* Attempting git cherry-pick %s\n", sha)
	out, err := exec.CommandContext(ctx, "git", "cherry-pick", sha).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "git cherry-pick %s failed:\n%s\n", sha, string(out))
		return fmt.Errorf("git cherry-pick %s failed: %w", sha, err)
	}
	return nil
}

func checkoutBackportBranch(ctx context.Context, backportBranch, releaseBranch string) error {
	out, err := exec.CommandContext(ctx, "git", "branch", "--show-current").Output()
	if err != nil {
		return fmt.Errorf("unable to check current branch %w", err)
	}

	currentBranch := strings.TrimSpace(string(out))
	fmt.Printf("* Current branch is %s\n", currentBranch)
	if currentBranch == backportBranch {
		fmt.Printf("* Current branch is %s - not checking out\n", currentBranch)
		return nil
	}

	if _, err := exec.CommandContext(ctx, "git", "rev-list", "-1", backportBranch).Output(); err == nil {
		fmt.Printf("* Branch %s already exists. Checking it out...\n", backportBranch)
		return exec.CommandContext(ctx, "git", "checkout", "-f", backportBranch).Run()
	}

	fmt.Printf("* `git checkout -b %s %s`\n", backportBranch, releaseBranch)
	return exec.CommandContext(ctx, "git", "checkout", "-b", backportBranch, releaseBranch).Run()
}

func fetchRemoteAndMain(ctx context.Context, remote, releaseBranch string) error {
	fmt.Printf("* `git fetch %s main`\n", remote)
	out, err := exec.CommandContext(ctx, "git", "fetch", remote, "main").Output()
	if err != nil {
		fmt.Println(string(out))
		return fmt.Errorf("unable to fetch %s from %s: %w", "main", remote, err)
	}
	fmt.Println(string(out))

	fmt.Printf("* `git fetch %s %s`\n", remote, releaseBranch)
	out, err = exec.CommandContext(ctx, "git", "fetch", remote, releaseBranch).Output()
	if err != nil {
		fmt.Println(string(out))
		return fmt.Errorf("unable to fetch %s from %s: %w", releaseBranch, remote, err)
	}
	fmt.Println(string(out))

	return nil
}

func determineRemote(ctx context.Context, forkUser string) (string, string, error) {
	out, err := exec.CommandContext(ctx, "git", "remote", "-v").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to list git remotes:\n%s\n", string(out))
		return "", "", fmt.Errorf("unable to determine forked remote: %w", err)
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Split(line, "\t")
		name, remote := fields[0], fields[1]
		// only look at pushers
		if !strings.HasSuffix(remote, " (push)") {
			continue
		}
		// only look at github.com pushes
		if !strings.Contains(remote, "github.com") {
			continue
		}
		// ignore go-gitea/gitea
		if strings.Contains(remote, "go-gitea/gitea") {
			continue
		}
		if !strings.Contains(remote, forkUser) {
			continue
		}
		if strings.HasPrefix(remote, "git@github.com:") {
			forkUser = strings.TrimPrefix(remote, "git@github.com:")
		} else if strings.HasPrefix(remote, "https://github.com/") {
			forkUser = strings.TrimPrefix(remote, "https://github.com/")
		} else if strings.HasPrefix(remote, "https://www.github.com/") {
			forkUser = strings.TrimPrefix(remote, "https://www.github.com/")
		} else if forkUser == "" {
			return "", "", fmt.Errorf("unable to extract forkUser from remote %s: %s", name, remote)
		}
		idx := strings.Index(forkUser, "/")
		if idx >= 0 {
			forkUser = forkUser[:idx]
		}
		return name, forkUser, nil
	}
	return "", "", fmt.Errorf("unable to find appropriate remote in:\n%s", string(out))
}

func readCurrentBranch(ctx context.Context) (pr, version string, err error) {
	out, err := exec.CommandContext(ctx, "git", "branch", "--show-current").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read current git branch:\n%s\n", string(out))
		return "", "", fmt.Errorf("unable to read current git branch: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "-")

	if len(parts) != 3 || parts[0] != "backport" {
		fmt.Fprintf(os.Stderr, "Unable to continue from git branch:\n%s\n", string(out))
		return "", "", fmt.Errorf("unable to continue from git branch:\n%s", string(out))
	}

	return parts[1], parts[2], nil
}

func readVersion() string {
	bs, err := os.ReadFile("docs/config.yaml")
	if err != nil {
		if err == os.ErrNotExist {
			log.Println("`docs/config.yaml` not present")
			return ""
		}
		fmt.Fprintf(os.Stderr, "Unable to read `docs/config.yaml`: %v\n", err)
		return ""
	}

	type params struct {
		Version string
	}
	type docConfig struct {
		Params params
	}
	dc := &docConfig{}
	if err := yaml.Unmarshal(bs, dc); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read `docs/config.yaml`: %v\n", err)
		return ""
	}

	if dc.Params.Version == "" {
		fmt.Fprintf(os.Stderr, "No version in `docs/config.yaml`")
		return ""
	}

	version := dc.Params.Version
	if version[0] != 'v' {
		version = "v" + version
	}

	split := strings.SplitN(version, ".", 3)

	return strings.Join(split[:2], ".")
}

func determineSHAforPR(ctx context.Context, prStr string) (string, error) {
	prNum, err := strconv.Atoi(prStr)
	if err != nil {
		return "", err
	}

	client := github.NewClient(http.DefaultClient)

	pr, _, err := client.PullRequests.Get(ctx, "go-gitea", "gitea", prNum)
	if err != nil {
		return "", err
	}

	if pr.Merged == nil || !*pr.Merged {
		return "", fmt.Errorf("PR #%d is not yet merged - cannot determine sha to backport", prNum)
	}

	if pr.MergeCommitSHA != nil {
		return *pr.MergeCommitSHA, nil
	}

	return "", nil
}

func installSignals() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// install notify
		signalChannel := make(chan os.Signal, 1)

		signal.Notify(
			signalChannel,
			syscall.SIGINT,
			syscall.SIGTERM,
		)
		select {
		case <-signalChannel:
		case <-ctx.Done():
		}
		cancel()
		signal.Reset()
	}()

	return ctx, cancel
}
