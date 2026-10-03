// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package external

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"gitea.dev/modules/markup"
	"gitea.dev/modules/process"
	"gitea.dev/modules/setting"

	"github.com/kballard/go-shellquote"
)

// RegisterRenderers registers all supported third part renderers according settings
func RegisterRenderers() {
	markup.RegisterRenderer(&frontendRenderer{
		name: "openapi-swagger",
		patterns: []string{
			"openapi.yaml",
			"openapi.yml",
			"openapi.json",
			"swagger.yaml",
			"swagger.yml",
			"swagger.json",
		},
	})

	markup.RegisterRenderer(&frontendRenderer{
		name: "viewer-3d",
		patterns: []string{
			// It needs more logic to make it overall right (render a text 3D model automatically):
			// we need to distinguish the ambiguous filename extensions.
			// For example: "*.amf, *.obj, *.off, *.step" might be or not be a 3D model file.
			// So when it is a text file, we can't assume that "we only render it by 3D plugin",
			// otherwise the end users would be impossible to view its real content when the file is not a 3D model.
			"*.3dm", "*.3ds", "*.3mf", "*.amf", "*.bim", "*.brep",
			"*.dae", "*.fbx", "*.fcstd", "*.glb", "*.gltf",
			"*.ifc", "*.igs", "*.iges", "*.stp", "*.step",
			"*.stl", "*.obj", "*.off", "*.ply", "*.wrl",
		},
	})

	markup.RegisterRenderer(&frontendRenderer{
		name:     "asciicast",
		patterns: []string{"*.cast"},
	})

	for _, renderer := range setting.ExternalMarkupRenderers {
		markup.RegisterRenderer(&Renderer{renderer})
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

func (p *Renderer) Name() string {
	return p.MarkupName
}

func (p *Renderer) NeedPostProcess() bool {
	return p.MarkupRenderer.NeedPostProcess
}

func (p *Renderer) FileNamePatterns() []string {
	return p.FilePatterns
}

func (p *Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return p.MarkupSanitizerRules
}

func (p *Renderer) GetExternalRendererOptions() (ret markup.ExternalRendererOptions) {
	ret.SanitizerDisabled = p.RenderContentMode == setting.RenderContentModeNoSanitizer || p.RenderContentMode == setting.RenderContentModeIframe
	ret.DisplayInIframe = p.RenderContentMode == setting.RenderContentModeIframe
	ret.ContentSandbox = p.RenderContentSandbox
	return ret
}

func sanitizeCliArgUrl(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	isSafeShellChar := func(c byte) bool {
		return '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' ||
			c == '-' || c == '_' || c == '.' || c == '~' ||
			c == '+' || c == '@' || c == '%' || c == ':' || c == '[' || c == ']' ||
			c >= 128
	}
	for i := 0; i < len(u.Host); i++ {
		if !isSafeShellChar(u.Host[i]) {
			return ""
		}
	}
	const hex = "0123456789ABCDEF"
	pathFields := strings.Split(u.EscapedPath(), "/")
	for idx, pathField := range pathFields {
		pos := 0
		for ; pos < len(pathField); pos++ {
			if !isSafeShellChar(pathField[pos]) {
				break
			}
		}
		if pos == len(pathField) {
			continue
		}
		b := make([]byte, pos, len(pathField)+10)
		copy(b, pathField[:pos])
		for ; pos < len(pathField); pos++ {
			if isSafeShellChar(pathField[pos]) {
				b = append(b, pathField[pos])
			} else {
				b = append(b, '%', hex[pathField[pos]>>4], hex[pathField[pos]&0xf])
			}
		}
		pathFields[idx] = string(b)
	}
	u2 := &url.URL{Scheme: u.Scheme, Host: u.Host}
	return u2.String() + strings.Join(pathFields, "/")
}

func (p *Renderer) prepareExternalCommand(vars map[string]string) (string, []string, error) {
	fields, err := shellquote.Split(strings.TrimSpace(p.Command))
	if err != nil {
		return "", nil, err
	}
	if len(fields) == 0 {
		return "", nil, errors.New("no command")
	}
	var replacements []string
	for k, v := range vars {
		replacements = append(replacements, "$"+k, v)
		replacements = append(replacements, "%"+k+"%", v) // for legacy Windows-style support
	}
	r := strings.NewReplacer(replacements...)
	for i := range fields {
		fields[i] = r.Replace(fields[i])
	}
	return fields[0], fields[1:], nil
}

func (p *Renderer) prepare(baseLinkSrc, baseLinkRaw string) (ret struct {
	envs []string
	prog string
	args []string
}, err error,
) {
	// Although the URLs are also passed via environment variables,
	// we still pass them via command line arguments because a 3rd party render program may not read environment variables.
	// In case some site admins would write wrong render commands like `sh -c "echo $VAR"`, we sanitize the URLs here to make up for their mistakes
	cmdVars := map[string]string{
		"GITEA_PREFIX_SRC": sanitizeCliArgUrl(baseLinkSrc),
		"GITEA_PREFIX_RAW": sanitizeCliArgUrl(baseLinkRaw),
	}
	ret.prog, ret.args, err = p.prepareExternalCommand(cmdVars)
	if err != nil {
		return ret, err
	}
	ret.envs = append(
		os.Environ(),
		"GITEA_PREFIX_SRC="+baseLinkSrc,
		"GITEA_PREFIX_RAW="+baseLinkRaw,
	)
	return ret, nil
}

// Render renders the data of the document to HTML via the external tool.
func (p *Renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	baseLinkSrc := ctx.RenderHelper.ResolveLink("", markup.LinkTypeDefault)
	baseLinkRaw := ctx.RenderHelper.ResolveLink("", markup.LinkTypeRaw)
	prepared, err := p.prepare(baseLinkSrc, baseLinkRaw)
	if err != nil {
		return fmt.Errorf("invalid external render (%s) command %q: %w", p.Name(), p.Command, err)
	}
	if p.IsInputFile {
		// write to temp file
		tmpFile, cleanup, err := setting.AppDataTempDir("git-repo-content").CreateTempFileRandom("gitea_input")
		if err != nil {
			return fmt.Errorf("%s create temp file when rendering %s failed: %w", p.Name(), p.Command, err)
		}
		defer cleanup()

		_, err = io.Copy(tmpFile, input)
		if err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("%s write data to temp file when rendering %s failed: %w", p.Name(), p.Command, err)
		}

		err = tmpFile.Close()
		if err != nil {
			return fmt.Errorf("%s close temp file when rendering %s failed: %w", p.Name(), p.Command, err)
		}
		prepared.args = append(prepared.args, tmpFile.Name())
	}

	processCtx, _, finished := process.GetManager().AddContext(ctx, fmt.Sprintf("Render [%s] for %s", prepared.prog, baseLinkSrc))
	defer finished()

	cmd := process.CommandContext(processCtx, prepared.prog, prepared.args...)
	cmd.Env = prepared.envs
	if !p.IsInputFile {
		cmd.Stdin = input
	}
	var stderr bytes.Buffer
	cmd.Stdout = output
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s render run command %s %v failed: %w\nStderr: %s", p.Name(), prepared.prog, shellquote.Join(prepared.args...), err, stderr.String())
	}
	return nil
}
