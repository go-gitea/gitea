// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//nolint:forbidigo
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/google/go-github/v53/github"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Name = "backport"
	app.Usage = "Backport provided PR-number on to the current or previous released version"
	app.Description = `Backport will look-up the PR in Gitea's git log and attempt to cherry-pick it on the current version`
	app.ArgsUsage = "<PR-to-backport>"

	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "version",
			Usage: "Version branch to backport on to",
		},
		&cli.StringFlag{
			Name:  "upstream",
			Value: "origin",
			Usage: "Upstream remote for the Gitea upstream",
		},
		&cli.StringFlag{
			Name:  "release-branch",
			Value: "",
			Usage: "Release branch to backport on. Will default to release/<version>",
		},
		&cli.StringFlag{
			Name:  "cherry-pick",
			Usage: "SHA to cherry-pick as backport",
		},
		&cli.StringFlag{
			Name:  "backport-branch",
			Usage: "Backport branch to backport on to (default: backport-<pr>-<version>",
		},
		&cli.BoolFlag{
			Name:  "no-fetch",
			Usage: "Set this flag to prevent fetch of remote branches",
		},
		&cli.BoolFlag{
			Name:  "no-amend-message",
			Usage: "Set this flag to prevent automatic amendment of the commit message",
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
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
}

func runBackport(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	version := c.String("version")
	if version == "" {
		return fmt.Errorf("Provide a version to backport to")
	}

	upstream := c.String("upstream")
	if upstream == "" {
		upstream = "origin"
	}

	upstreamReleaseBranch := c.String("release-branch")
	if upstreamReleaseBranch == "" {
		upstreamReleaseBranch = path.Join("release", version)
	}

	localReleaseBranch := path.Join(upstream, upstreamReleaseBranch)

	args := c.Args().Slice()
	if len(args) == 0 {
		return fmt.Errorf("Provide a PR number to backport")
	} else if len(args) != 1 {
		return fmt.Errorf("Only a single PR can be backported at a time")
	}
	pr := args[0]

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

	if err := checkoutBackportBranch(ctx, backportBranch, localReleaseBranch); err != nil {
		return err
	}

	if err := cherrypick(ctx, sha); err != nil {
		return err
	}

	if !c.Bool("no-amend-message") {
		if err := amendCommit(ctx, pr); err != nil {
			return err
		}
	}

	fmt.Printf("Backport done! You can now push it with `git push <your remote> %s`\n", backportBranch)

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
	fmt.Printf("* Attempting git cherry-pick %s\n", sha)
	out, err := exec.CommandContext(ctx, "git", "cherry-pick", sha).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "git cherry-pick %s failed:\n%s\n", sha, string(out))
		return fmt.Errorf("git cherry-pick %s failed: %w", sha, err)
	}
	return nil
}

func checkoutBackportBranch(ctx context.Context, backportBranch, releaseBranch string) error {
	fmt.Printf("* `git branch -D %s`\n", backportBranch)
	_ = exec.CommandContext(ctx, "git", "branch", "-D", backportBranch).Run()

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

	fmt.Printf("* `git fetch %s %s`\n", remote, releaseBranch)
	out, err = exec.CommandContext(ctx, "git", "fetch", remote, releaseBranch).Output()
	if err != nil {
		fmt.Println(string(out))
		return fmt.Errorf("unable to fetch %s from %s: %w", releaseBranch, remote, err)
	}

	return nil
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
