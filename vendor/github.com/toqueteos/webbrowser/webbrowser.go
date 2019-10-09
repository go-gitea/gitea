// Package webbrowser provides a simple API for opening web pages on your
// default browser.
package webbrowser

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var (
	ErrCantOpenBrowser = errors.New("webbrowser: can't open browser")
	ErrNoCandidates    = errors.New("webbrowser: no browser candidate found for your OS")
)

// Candidates contains a list of registered `Browser`s that will be tried with Open.
var Candidates []Browser

type Browser interface {
	// Command returns a ready to be used Cmd that will open an URL.
	Command(string) (*exec.Cmd, error)
	// Open tries to open a URL in your default browser. NOTE: This may cause
	// your program to hang until the browser process is closed in some OSes,
	// see https://github.com/toqueteos/webbrowser/issues/4.
	Open(string) error
}

// Open tries to open a URL in your default browser ensuring you have a display
// set up and not running this from SSH. NOTE: This may cause your program to
// hang until the browser process is closed in some OSes, see
// https://github.com/toqueteos/webbrowser/issues/4.
func Open(s string) (err error) {
	if len(Candidates) == 0 {
		return ErrNoCandidates
	}

	// Try to determine if there's a display available (only linux) and we
	// aren't on a terminal (all but windows).
	switch runtime.GOOS {
	case "linux":
		// No display, no need to open a browser. Lynx users **MAY** have
		// something to say about this.
		if os.Getenv("DISPLAY") == "" {
			return fmt.Errorf("webbrowser: tried to open %q, no screen found", s)
		}
		fallthrough
	case "darwin":
		// Check SSH env vars.
		if os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != "" {
			return fmt.Errorf("webbrowser: tried to open %q, but you are running a shell session", s)
		}
	}

	// Try all candidates
	for _, candidate := range Candidates {
		err := candidate.Open(s)
		if err == nil {
			return nil
		}
	}

	return ErrCantOpenBrowser
}

func init() {
	// Register the default Browser for current OS, if it exists.
	if os, ok := osCommand[runtime.GOOS]; ok {
		Candidates = append(Candidates, browserCommand{os.cmd, os.args})
	}
}

var (
	osCommand = map[string]*browserCommand{
		"android": &browserCommand{"xdg-open", nil},
		"darwin":  &browserCommand{"open", nil},
		"freebsd": &browserCommand{"xdg-open", nil},
		"linux":   &browserCommand{"xdg-open", nil},
		"netbsd":  &browserCommand{"xdg-open", nil},
		"openbsd": &browserCommand{"xdg-open", nil}, // It may be open instead
		"windows": &browserCommand{"cmd", []string{"/c", "start"}},
	}
	winSchemes = [3]string{"https", "http", "file"}
)

type browserCommand struct {
	cmd  string
	args []string
}

func (b browserCommand) Command(s string) (*exec.Cmd, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	validUrl := ensureValidURL(u)

	b.args = append(b.args, validUrl)

	return exec.Command(b.cmd, b.args...), nil
}

func (b browserCommand) Open(s string) error {
	cmd, err := b.Command(s)
	if err != nil {
		return err
	}

	return cmd.Run()
}

func ensureScheme(u *url.URL) {
	for _, s := range winSchemes {
		if u.Scheme == s {
			return
		}
	}
	u.Scheme = "http"
}

func ensureValidURL(u *url.URL) string {
	// Enforce a scheme (windows requires scheme to be set to work properly).
	ensureScheme(u)
	s := u.String()

	// Escape characters not allowed by cmd/bash
	switch runtime.GOOS {
	case "windows":
		s = strings.Replace(s, "&", `^&`, -1)
	}

	return s
}
