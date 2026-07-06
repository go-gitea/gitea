// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gitea.dev/modules/setting"
	"gitea.dev/services/aireview"

	"github.com/urfave/cli/v3"
)

var CmdAIReview = &cli.Command{
	Name:        "ai-review",
	Usage:       "Run AI code review on a local diff",
	Description: "Reads a git diff from stdin and runs AI code review on it.",
	Action:      runAIReview,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "diff",
			Usage: "Path to diff file (default: read from stdin)",
		},
		&cli.StringFlag{
			Name:  "file",
			Usage: "Single file to review",
		},
	},
}

func runAIReview(ctx context.Context, c *cli.Command) error {
	var diff string
	if diffPath := c.String("diff"); diffPath != "" {
		data, err := os.ReadFile(diffPath)
		if err != nil {
			return fmt.Errorf("read diff file: %w", err)
		}
		diff = string(data)
	} else if filePath := c.String("file"); filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		diff = fmt.Sprintf("--- a/%s\n+++ b/%s\n@@ -1,%d +1,%d @@\n%s", filePath, filePath, len(strings.Split(string(data), "\n")), len(strings.Split(string(data), "\n")), string(data))
	} else {
		cmd := exec.Command("git", "diff")
		stdout, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("git diff: %w", err)
		}
		diff = string(stdout)
	}

	if strings.TrimSpace(diff) == "" {
		return errors.New("no diff provided")
	}

	provider, err := aireview.GetProvider(setting.AIRreview.Provider)
	if err != nil {
		return fmt.Errorf("get provider: %w", err)
	}

	resp, err := provider.ReviewCode(context.Background(), &aireview.ReviewRequest{
		Diff: diff,
	})
	if err != nil {
		return fmt.Errorf("review failed: %w", err)
	}

	fmt.Println("## AI Code Review Results")
	if resp.Summary != "" {
		fmt.Printf("### Summary\n%s\n\n", resp.Summary)
	}
	if len(resp.Comments) > 0 {
		fmt.Println("### Comments")
		for _, c := range resp.Comments {
			loc := ""
			if c.File != "" {
				loc = fmt.Sprintf("%s:%d ", c.File, c.Line)
			}
			fmt.Printf("- %s[%s] %s\n", loc, c.Severity, c.Body)
		}
	}

	return nil
}
