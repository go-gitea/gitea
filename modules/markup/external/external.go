// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package external

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// RegisterRenderers registers all supported third part renderers according settings
func RegisterRenderers() {
	for _, renderer := range setting.ExternalMarkupRenderers {
		if renderer.Enabled && renderer.Command != "" && len(renderer.FileExtensions) > 0 {
			markup.RegisterRenderer(&Renderer{renderer})
		}
	}
}

// Renderer implements markup.Renderer for external tools
type Renderer struct {
	*setting.MarkupRenderer
}

var (
	_ markup.PostProcessRenderer = (*Renderer)(nil)
	_ markup.ExternalRenderer    = (*Renderer)(nil)
)

// Name returns the external tool name
func (p *Renderer) Name() string {
	return p.MarkupName
}

// NeedPostProcess implements markup.Renderer
func (p *Renderer) NeedPostProcess() bool {
	return p.MarkupRenderer.NeedPostProcess
}

// Extensions returns the supported extensions of the tool
func (p *Renderer) Extensions() []string {
	return p.FileExtensions
}

// SanitizerRules implements markup.Renderer
func (p *Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return p.MarkupSanitizerRules
}

// SanitizerDisabled disabled sanitize if return true
func (p *Renderer) SanitizerDisabled() bool {
	return p.RenderContentMode == setting.RenderContentModeNoSanitizer || p.RenderContentMode == setting.RenderContentModeIframe
}

// DisplayInIFrame represents whether render the content with an iframe
func (p *Renderer) DisplayInIFrame() bool {
	return p.RenderContentMode == setting.RenderContentModeIframe
}

func envMark(envName string) string {
	if runtime.GOOS == "windows" {
		return "%" + envName + "%"
	}
	return "$" + envName
}

// Render renders the data of the document to HTML via the external tool.
func (p *Renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	var (
		command = strings.NewReplacer(
			envMark("GITEA_PREFIX_SRC"), ctx.RenderHelper.ResolveLink("", markup.LinkTypeDefault),
			envMark("GITEA_PREFIX_RAW"), ctx.RenderHelper.ResolveLink("", markup.LinkTypeRaw),
		).Replace(p.Command)
		commands = strings.Fields(command)
		args     = commands[1:]
	)

	if p.IsInputFile {
		// write to temp file
		f, err := os.CreateTemp("", "gitea_input")
		if err != nil {
			return fmt.Errorf("%s create temp file when rendering %s failed: %w", p.Name(), p.Command, err)
		}
		tmpPath := f.Name()
		defer func() {
			if err := util.Remove(tmpPath); err != nil {
				log.Warn("Unable to remove temporary file: %s: Error: %v", tmpPath, err)
			}
		}()

		_, err = io.Copy(f, input)
		if err != nil {
			_ = f.Close()
			return fmt.Errorf("%s write data to temp file when rendering %s failed: %w", p.Name(), p.Command, err)
		}

		err = f.Close()
		if err != nil {
			return fmt.Errorf("%s close temp file when rendering %s failed: %w", p.Name(), p.Command, err)
		}
		args = append(args, f.Name())
	}

	processCtx, _, finished := process.GetManager().AddContext(ctx, fmt.Sprintf("Render [%s] for %s", commands[0], ctx.RenderHelper.ResolveLink("", markup.LinkTypeDefault)))
	defer finished()

	cmd := exec.CommandContext(processCtx, commands[0], args...)
	cmd.Env = append(
		os.Environ(),
		"GITEA_PREFIX_SRC="+ctx.RenderHelper.ResolveLink("", markup.LinkTypeDefault),
		"GITEA_PREFIX_RAW="+ctx.RenderHelper.ResolveLink("", markup.LinkTypeRaw),
	)
	if !p.IsInputFile {
		cmd.Stdin = input
	}
	var stderr bytes.Buffer
	cmd.Stdout = output
	cmd.Stderr = &stderr
	process.SetSysProcAttribute(cmd)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s render run command %s %v failed: %w\nStderr: %s", p.Name(), commands[0], args, err, stderr.String())
	}
	return nil
}
